package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"sync"
	"time"
	"strings"

	"golang.org/x/oauth2"
	"github.com/google/go-github/v39/github"
	"github.com/mmcdole/gofeed"
	"github.com/tencentyun/cos-go-sdk-v5"
)

const (
	maxRetries     = 3						 // 最大重试次数
	retryInterval  = 10 * time.Second		 // 最大间隔时间
	maxConcurrency = 10						 // 并发控制
	cosURL         = "https://lhasa-1253887673.cos.ap-shanghai.myqcloud.com/data/rss_data.json" // 腾讯云 COS URL
)

// 全局日志缓存
var (
	errorLogCache []string            // 错误日志缓存
	errorLogLock  sync.Mutex          // 缓存锁
	logCache      map[string][]string // 其他日志缓存（按文件名分类）
	logCacheLock  sync.Mutex          // 日志缓存锁
)

func init() {
	logCache = make(map[string][]string)
}

type Config struct {
	SecretID         string		 // 腾讯云 SecretID
	SecretKey        string		 // 腾讯云 SecretKey
	GithubToken      string		 // GitHub API 令牌
	GithubName       string		 // GitHub 用户名
	GithubRepository string		 // GitHub 仓库名
}

// 解析头像数据
// 如果用 Article 解析 avatar_data.json，会导致 domainName 字段丢失
type AvatarData struct {
	DomainName string `json:"domainName"` // 标准化的博客域名
	Name       string `json:"name"`		  // avatar_data.json 自选订阅数据
	Avatar     string `json:"avatar"`	  // 缓存头像 URL
}

// 抓取的爬虫数据
type Article struct {
	DomainName string `json:"domainName"` // 域名
	Name       string `json:"name"`       // 博客名称（经过处理后用于显示名称）
	Title      string `json:"title"`      // 文章标题
	Link       string `json:"link"`       // 文章链接
	Date       string `json:"date"`       // 格式化后的文章发布时间
	Avatar     string `json:"avatar"`     // 头像 URL
}

func initConfig() (*Config, error) {
	config := &Config{
		SecretID:  		  os.Getenv("TENCENT_CLOUD_SECRET_ID"),
		SecretKey: 		  os.Getenv("TENCENT_CLOUD_SECRET_KEY"),
		GithubToken:      os.Getenv("TOKEN"),
		GithubName:       os.Getenv("NAME"),
		GithubRepository: os.Getenv("REPOSITORY"),
	}

	// 验证
	required := map[string]string{
		"TENCENT_CLOUD_SECRET_ID":  config.SecretID,
		"TENCENT_CLOUD_SECRET_KEY": config.SecretKey,
		"TOKEN":                    config.GithubToken,
		"NAME":                     config.GithubName,
		"REPOSITORY":               config.GithubRepository,
	}

	for k, v := range required {
		if v == "" {
			return nil, fmt.Errorf("%s is required", k)
		}
	}

	return config, nil
}

// 重试机制
func withRetry(ctx context.Context, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = fn(); err == nil {
			return nil
		}
		select {
		case <-time.After(retryInterval):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

// 清理 XML 内容中的非法字符
func cleanXMLContent(content string) string {
	re := regexp.MustCompile(`[\x00-\x1F\x7F-\x9F]`)
	return re.ReplaceAllString(content, "")
}

// 尝试解析不同格式的时间字符串
func parseTime(timeStr string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123Z,
		time.RFC1123,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// 将时间格式化为 "Jnuary 2, 2006a"
func formatTime(t time.Time) string {
	return t.Format("January 2, 2006")
}

// 提取域名
func extractDomain(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	
	// 自动补全协议头
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	
	// 返回标准化域名（包含协议）
	return fmt.Sprintf("%s://%s", u.Scheme, u.Hostname()), nil

}

// 获取当前的北京时间
func getBeijingTime() time.Time {
	return time.Now().In(time.FixedZone("CST", 8*3600))
}

// 正常
// 日志
// func logMessage(config *Config, message, fileName string) {
// 	ctx := context.Background()
// 	client := github.NewClient(oauth2.NewClient(
// 		ctx, oauth2.StaticTokenSource(
// 			&oauth2.Token{AccessToken: config.GithubToken},
// 	)))

// 	filePath := "_data/" + fileName
// 	content := []byte(fmt.Sprintf("[%s] %s\n", 
// 	getBeijingTime().Format("2006-01-02 15:04:05"), message))

// 	err := withRetry(ctx, func() error {
// 		file, _, _, err := client.Repositories.GetContents(ctx, 
// 			config.GithubName, config.GithubRepository, filePath, nil)
// 			// 文件不存在则创建
// 			if err != nil {
// 				_, _, err = client.Repositories.CreateFile(ctx, 
// 					config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
// 						Message: github.String("Create " + fileName),
// 						Content: content,
// 						Branch:  github.String("master"),
// 					})
// 				return err
// 			}

// 			// 文件存在则追加内容
// 			decoded, _ := file.GetContent()
// 			newContent := append([]byte(decoded+"\n"), content...)
// 			_, _, err = client.Repositories.UpdateFile(ctx, config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
// 				Message: github.String("Update " + fileName),
// 				Content: newContent,
// 				SHA:     file.SHA,
// 				Branch:  github.String("master"),
// 			})
// 			return err
// 		})

// 	if err != nil {
// 		fmt.Printf("Log error: %v\n", err)
// 	}
// }

// 统一日志写入
func logMessages(config *Config, messages []string, fileName string) {
	ctx := context.Background()
	client := github.NewClient(oauth2.NewClient(
		ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: config.GithubToken},
		)))

	filePath := "_data/" + fileName
	content := []byte(strings.Join(messages, "\n") + "\n")

	err := withRetry(ctx, func() error {
		file, _, _, err := client.Repositories.GetContents(ctx,
			config.GithubName, config.GithubRepository, filePath, nil)
		
		// 文件不存在则创建
		if err != nil {
			_, _, err = client.Repositories.CreateFile(ctx,
				config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
					Message: github.String("Batch update " + fileName),
					Content: content,
					Branch:  github.String("master"),
				})
			return err
		}

		// 文件存在则追加内容
		decoded, _ := file.GetContent()
		newContent := append([]byte(decoded+"\n"), content...)
		_, _, err = client.Repositories.UpdateFile(ctx,
			config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
				Message: github.String("Batch update " + fileName),
				Content: newContent,
				SHA:     file.SHA,
				Branch:  github.String("master"),
			})
		return err
	})

	if err != nil {
		fmt.Printf("Final log write error: %v\n", err)
	}
}

// 正常
// func logError(config *Config, message string) {
// 	logMessage(config, message, "error.log")
// }

// 错误日志记录（缓存模式）
func logError(config *Config, message string) {
	errorLogLock.Lock()
	defer errorLogLock.Unlock()
	errorLogCache = append(errorLogCache, fmt.Sprintf("[%s] %s",
		getBeijingTime().Format("2006-01-02 15:04:05"), message))
}

// COS客户端初始化
func newCOSClient(config *Config) *cos.Client {
	u, _ := url.Parse(cosURL)
	return cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.SecretID,
			SecretKey: config.SecretKey,
		},
	})
	logError(config, fmt.Sprintf("COS client initialized for bucket: %s", u.Host))
	return client
}

// 从腾讯云 COS 获取 JSON 文件
func fetchFileFromCOS(config *Config, filePath string) (string, error) {
	client := newCOSClient(config)
	var content string
	var lastErr error

	err := withRetry(context.Background(), func() error {
		// 获取文件内容
		resp, err := client.Object.Get(context.Background(), filePath, nil)
		if err != nil {
			lastErr = fmt.Errorf("COS GET %s: %w", filePath, err)
			return err
		}
		defer resp.Body.Close()

		// 读取文件内容
		data, _ := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("Read body %s: %w", filePath, err)
			return lastErr
		}
		content = string(data)
		return nil
	})
	if err != nil {
		logError(config, fmt.Sprintf("Failed to fetch %s after %d retries: %v",
			filePath, maxRetries, lastErr))
		return "", err
	}
	
	logError(config, fmt.Sprintf("Successfully fetched %s (%d bytes)", 
		filePath, len(content)))
	return content, nil
}

// 从腾讯云 COS 获取头像
func loadAvatarsFromCOS(config *Config) (map[string]string, error) {
	content, err := fetchFileFromCOS(config, "data/avatar_data.json")
	if err != nil {
		return nil, err
	}

	var avatarData []AvatarData
	if err := json.Unmarshal([]byte(content), &avatarData); err != nil {
		return nil, err
	}

	avatarMap := make(map[string]string)
	for _, a := range avatarData {
		// 解析标准化域名作为键
		if domain, err := extractDomain(a.DomainName); err == nil {
			avatarMap[domain] = a.Avatar
		}
	}
	return avatarMap, nil
}

// 从 RSS 列表中抓取最新的文章，并按发布时间排序
func fetchRSS(config *Config, feeds []string) ([]Article, error) {
	var (
		articles []Article
		mu       sync.Mutex
		wg       sync.WaitGroup
		sem      = make(chan struct{}, maxConcurrency)
	)

	// 获取头像（使用标准化域名作为键）
	avatars, err := loadAvatarsFromCOS(config)
	if err != nil {
		logError(config, fmt.Sprintf("Load avatars error: %v", err))
		return nil, err
	}

	fp := gofeed.NewParser()
	httpClient := &http.Client{Timeout: 10 * time.Second,}

	for _, feedURL := range feeds {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var (
				bodyString string
				feed       *gofeed.Feed
			)

			// 重试机制获取 RSS 数据
			if err := withRetry(context.Background(), func() error {
				resp, err := httpClient.Get(url)
				if err != nil {
					return err
				}
				defer resp.Body.Close()

				data, _ := io.ReadAll(resp.Body)
				bodyString = cleanXMLContent(string(data))
				return nil
			}); err != nil {
				logError(config, fmt.Sprintf("Failed to fetch RSS: %s (%v)", url, err))
				return
			}

			// 重试机制解析 RSS 数据
			if err := withRetry(context.Background(), func() error {
				f, err := fp.ParseString(bodyString)
				if err != nil {
					return err
				}
				feed = f
				return nil
			}); err != nil {
				logError(config, fmt.Sprintf("Failed to parse RSS: %s (%v)", url, err))
				return
			}

			if len(feed.Items) == 0 {
				return
			}

			mainSiteURL := feed.Link
			domainName, err := extractDomain(mainSiteURL)
			if err != nil {
				logError(config, fmt.Sprintf("[%s] [Extract domain error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), mainSiteURL, err))
				domainName = "unknown"
			}

			name := feed.Title
			avatarURL := avatars[domainName]
			if avatarURL == "" {
				avatarURL = "https://cos.lhasa.icu/LinksAvatar/default.png"
			}

			item := feed.Items[0]
			published, _ := parseTime(item.Published)
			if item.Updated != "" {
				published, _ = parseTime(item.Updated)
			}

			// 名称映射
			nameMapping := map[string]string{
				"obaby@mars":                   	  "obaby",
				"青山小站 | 一个在帝都搬砖的新时代农民工": "青山小站",
				"Homepage on Miao Yu | 于淼":          "于淼",
				"Homepage on Yihui Xie | 谢益辉":      "谢益辉",
			}

			if mapped, ok := nameMapping[name]; ok {
				name = mapped
			}

			mu.Lock()
			articles = append(articles, Article{
				DomainName: domainName,
				Name:       name,
				Title:      item.Title,
				Link:       item.Link,
				Date:       formatTime(published),
				Avatar:     avatarURL,
			})
			mu.Unlock()
		}(feedURL)
	}

	wg.Wait()

	// 根据时间排序
	sort.Slice(articles, func(i, j int) bool {
		ti, _ := time.Parse("January 2, 2006", articles[i].Date)
		tj, _ := time.Parse("January 2, 2006", articles[j].Date)
		return ti.After(tj)
	})

	return articles, nil
}

// 将爬虫抓取的数据保存到腾讯云 COS
func saveToCOS(config *Config, data []Article) error {
	// 十年之约
	data = append(data, Article{
		DomainName: "https://foreverblog.cn",
		Name:       "十年之约",
		Title:      "穿梭虫洞-随机访问十年之约友链博客",
		Link:       "https://foreverblog.cn/go.html",
		Date:       "January 01, 2000",
		Avatar:     "https://cos.lhasa.icu/LinksAvatar/foreverblog.cn.png",
	})

	jsonData, err := json.Marshal(data)
	if err != nil {
		logError(config, fmt.Sprintf("JSON marshal error: %v", err))
		return fmt.Errorf("marshal error: %v", err)
	}

	client := newCOSClient(config)

	// 重试上传
	var lastErr error
	err = withRetry(context.Background(), func() error {
		startTime := time.Now()
		_, err := client.Object.Put(
			context.Background(),
			"data/rss_data.json",
			bytes.NewReader(jsonData),
			nil,
		)
		
		// 记录单次上传结果
		if err != nil {
			lastErr = fmt.Errorf("COS PUT error: %w", err)
			logError(config, fmt.Sprintf("Upload attempt failed: %v", lastErr))
		} else {
			logError(config, fmt.Sprintf("Upload succeeded in %v", time.Since(startTime)))
		}
		return err
	})

	// 最终结果处理
	if err != nil {
		finalErr := fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
		logError(config, fmt.Sprintf("Final upload failure: %v", finalErr))
		return finalErr
	}

	// 成功日志（记录数据量）
	logError(config, fmt.Sprintf("Successfully uploaded %d articles (%d bytes)", 
		len(data), len(jsonData)))
	return nil
}

// 从腾讯云 COS 获取 RSS 文件
func readFeedsFromCOS(config *Config) ([]string, error) {
	content, err := fetchFileFromCOS(config, "data/rss_feeds.txt")
	if err != nil {
		return nil, err
	}

	var feeds []string
	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	for scanner.Scan() {
		feeds = append(feeds, scanner.Text())
	}
	return feeds, nil
}

func main() {
	var config *Config
	// 最终日志写入处理
    defer func() {
        if len(errorLogCache) > 0 {
            // 传递当前config到闭包
            if config != nil {
                logMessages(config, errorLogCache, "error.log")
            } else {
                fmt.Println("无法写入日志：配置未初始化")
            }
        }
    }()

	// 初始化配置
	var err error
	config, err := initConfig()
    if err != nil {
        errorLogCache = append(errorLogCache, fmt.Sprintf("配置初始化失败: %v", err))
        fmt.Println(err)
        return
    }

	// 从COS读取RSS订阅列表
	feeds, err := readFeedsFromCOS(config)
	if err != nil {
		logError(config, fmt.Sprintf("读取订阅列表失败: %v", err))
		return
	}
	logError(config, fmt.Sprintf("成功加载 %d 个订阅源", len(feeds)))


	// 抓取RSS数据
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	
	var articles []Article
	done := make(chan struct{})
	go func() {
		articles, err = fetchRSS(config, feeds)
		close(done)
	}()

	select {
	case <-done:
		// 正常完成
	case <-ctx.Done():
		logError(config, "RSS抓取超时，已终止")
		return
	}

	if err != nil {
		logError(config, fmt.Sprintf("抓取RSS失败: %v", err))
		return
	}
	logError(config, fmt.Sprintf("成功抓取 %d 篇文章", len(articles)))

	// 保存到COS
	startSave := time.Now()
	if err := saveToCOS(config, articles); err != nil {
		logError(config, fmt.Sprintf("保存到COS失败: %v", err))
		return
	}
	logError(config, fmt.Sprintf("数据保存完成，耗时 %v", time.Since(startSave)))

	// 爬虫报告
	logError(config, fmt.Sprintf("流程完成，共处理 %d 篇文章", len(articles)))
	fmt.Println("Stop writing code and go ride a road bike now!")
}
