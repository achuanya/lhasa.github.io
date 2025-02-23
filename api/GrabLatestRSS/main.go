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
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"github.com/google/go-github/v39/github"
	"github.com/mmcdole/gofeed"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// ========================
//      配置结构体
// ========================
type Config struct {
	SecretID         string
	SecretKey        string
	GithubToken      string
	GithubName       string
	GithubRepository string
	COSURL           string
	MaxRetries       int
	RetryInterval    time.Duration
	MaxConcurrency   int
	HTTPTimeout      time.Duration
}

// ========================
//     全局变量/常量
// ========================
var (
	// 预编译时间格式
	timeFormats = []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123Z,
		time.RFC1123,
	}

	// 名称映射表
	nameMapping = map[string]string{
		"obaby@mars":                     "obaby",
		"青山小站 | 一个在帝都搬砖的新时代农民工": "青山小站",
		"Homepage on Miao Yu | 于淼":      "于淼",
		"Homepage on Yihui Xie | 谢益辉":  "谢益辉",
	}

	logChan    = make(chan logMessage, 1000)  // 异步日志通道
	errorChan  = make(chan error, 100)        // 错误收集通道
	shutdownWG sync.WaitGroup                 // 优雅关闭等待组
)

// ========================
//      数据结构定义
// ========================
type logMessage struct {
	level    string
	message  string
	fileName string
}

type RSSProcessor struct {
	config     *Config
	httpClient *http.Client
	cosClient  *cos.Client
	parser     *gofeed.Parser
	avatarMap  map[string]string
}

type Article struct {
	DomainName string `json:"domainName"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Link       string `json:"link"`
	Date       string `json:"date"`
	Avatar     string `json:"avatar"`
}

type AvatarData struct {
	DomainName string `json:"domainName"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
}

// ========================
//      初始化函数
// ========================
func init() {
	// 启动日志处理协程
	go logWorker()
}

// ========================
//      主程序入口
// ========================
func main() {
	config, err := initConfig()
	if err != nil {
		logAsync("ERROR", err.Error(), "error.log")
		os.Exit(1)
	}

	processor := NewRSSProcessor(config)
	defer processor.Close()

    // 设置总超时3分钟
    ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
    defer cancel()

	if err := processor.Run(ctx); err != nil {
		logAsync("ERROR", err.Error(), "error.log")
	}

	close(logChan)		// 关闭日志通道
	shutdownWG.Wait()	// 等待剩余日志写入

	// 确认资源释放
	logAsync("INFO", "程序正常退出", "system.log")

	fmt.Println("任务完成，去享受骑行吧！")
}

// ========================
//     配置初始化
// ========================
func initConfig() (*Config, error) {
	config := &Config{
		SecretID:         os.Getenv("TENCENT_CLOUD_SECRET_ID"),
		SecretKey:        os.Getenv("TENCENT_CLOUD_SECRET_KEY"),
		GithubToken:      os.Getenv("TOKEN"),
		GithubName:       os.Getenv("NAME"),
		GithubRepository: os.Getenv("REPOSITORY"),
		COSURL:           os.Getenv("COSURL"),
		MaxRetries:       getEnvInt("MAX_RETRIES", 3),
		RetryInterval:    getEnvDuration("RETRY_INTERVAL", 10*time.Second),
		MaxConcurrency:   getEnvInt("MAX_CONCURRENCY", 10),
		HTTPTimeout:      getEnvDuration("HTTP_TIMEOUT", 15*time.Second),
	}

	// 验证必需配置
	for k, v := range map[string]string{
		"TENCENT_CLOUD_SECRET_ID":  config.SecretID,
		"TENCENT_CLOUD_SECRET_KEY": config.SecretKey,
		"TOKEN":                    config.GithubToken,
		"NAME":                     config.GithubName,
		"REPOSITORY":               config.GithubRepository,
	} {
		if v == "" {
			return nil, fmt.Errorf("环境变量 %s 必须设置", k)
		}
	}

	return config, nil
}

// ========================
//     RSS处理器实现
// ========================
func NewRSSProcessor(config *Config) *RSSProcessor {
	transport := &http.Transport{
		MaxIdleConns:        config.MaxConcurrency * 2,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		MaxConnsPerHost:     config.MaxConcurrency,
		MaxIdleConnsPerHost: config.MaxConcurrency,
	}

	// 初始化COS客户端
	u, _ := url.Parse(config.COSURL)
	cosClient := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.SecretID,
			SecretKey: config.SecretKey,
		},
	})

	return &RSSProcessor{
		config:     config,
		httpClient: &http.Client{Timeout: config.HTTPTimeout, Transport: transport},
		cosClient:  cosClient,
		parser:     gofeed.NewParser(),
	}
}

func (p *RSSProcessor) Close() {
	p.httpClient.CloseIdleConnections()
}

func (p *RSSProcessor) Run(ctx context.Context) error {
    // 所有网络操作添加 ctx 控制，只调用一次获取feeds
    feeds, err := p.getFeeds(ctx)
	if err != nil {
        return err
    }

    articles, errs := p.fetchAllRSS(ctx, feeds)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 加载头像数据
	if err := p.loadAvatars(ctx); err != nil {
		return fmt.Errorf("加载头像数据失败: %w", err)
	}

	// 获取订阅列表
	feeds, err = p.getFeeds(ctx)
	if err != nil {
		return fmt.Errorf("获取订阅列表失败: %w", err)
	}

	// 并发抓取数据
	articles, errs = p.fetchAllRSS(ctx, feeds)
	if len(errs) > 0 {
		logAsync("WARN", fmt.Sprintf("共发生 %d 个错误", len(errs)), "warnings.log")
	}

	// 保存数据到COS
	if err := p.saveToCOS(ctx, articles); err != nil {
		return fmt.Errorf("保存到COS失败: %w", err)
	}

	return nil
}

// ========================
//      核心业务方法
// ========================
func (p *RSSProcessor) loadAvatars(ctx context.Context) error {
	content, err := p.fetchCOSFile(ctx, "data/avatar_data.json")
	if err != nil {
		return err
	}

	var avatarData []AvatarData
	if err := json.Unmarshal([]byte(content), &avatarData); err != nil {
		return fmt.Errorf("解析头像数据失败: %w", err)
	}

	p.avatarMap = make(map[string]string)
	for _, a := range avatarData {
		if domain, err := extractDomain(a.DomainName); err == nil {
			p.avatarMap[domain] = a.Avatar
		}
	}
	return nil
}

func (p *RSSProcessor) getFeeds(ctx context.Context) ([]string, error) {
	content, err := p.fetchCOSFile(ctx, "data/rss_feeds.txt")
	if err != nil {
		return nil, err
	}

	var feeds []string
	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))
	for scanner.Scan() {
		if feed := strings.TrimSpace(scanner.Text()); feed != "" {
			feeds = append(feeds, feed)
		}
	}
	return feeds, nil
}

func (p *RSSProcessor) fetchAllRSS(ctx context.Context, feeds []string) ([]Article, []error) {
    var (
        articles []Article
        errs     []error
        mutex    sync.Mutex
    )

    feedChan := make(chan string, len(feeds))
    var wg sync.WaitGroup

    // 创建工作池
    for i := 0; i < p.config.MaxConcurrency; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for url := range feedChan {
                article, err := p.processFeed(ctx, url)
                
                mutex.Lock()
                if err != nil {
                    errs = append(errs, err)
                } else {
                    articles = append(articles, *article)
                }
                mutex.Unlock()
            }
        }()
    }

    // 分发任务
    for _, feed := range feeds {
        feedChan <- feed
    }
    close(feedChan)

    wg.Wait()
    return articles, errs
}

func (p *RSSProcessor) processFeed(ctx context.Context, feedURL string) (*Article, error) {
	// 获取并清理内容
	body, err := withRetry(ctx, p.config.MaxRetries, p.config.RetryInterval, func() (string, error) {
		resp, err := p.httpClient.Get(feedURL)
		if err != nil {
			return "", fmt.Errorf("HTTP请求失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("非200状态码: %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取响应体失败: %w", err)
		}

		return cleanXMLContent(string(data)), nil
	})

	if err != nil {
		return nil, fmt.Errorf("获取源失败 %s: %w", feedURL, err)
	}

	// 解析内容
	feed, err := withRetry(ctx, p.config.MaxRetries, p.config.RetryInterval, func() (*gofeed.Feed, error) {
		return p.parser.ParseString(body)
	})

	if err != nil {
		return nil, fmt.Errorf("解析源失败 %s: %w", feedURL, err)
	}

	// 构造文章对象
	domain, err := extractDomain(feed.Link)
	if err != nil {
		domain = "unknown"
		logAsync("WARN", fmt.Sprintf("域名解析失败 %s: %v", feed.Link, err), "warnings.log")
	}

	// 获取头像URL
	avatarURL := p.avatarMap[domain]
	if avatarURL == "" {
		avatarURL = "https://cos.lhasa.icu/LinksAvatar/default.png"
	}

	// 处理最新文章
	if len(feed.Items) == 0 {
		return nil, fmt.Errorf("没有找到文章: %s", feedURL)
	}

	item := feed.Items[0]
	publishedTime, err := parseTime(item.Published)
	if err != nil && item.Updated != "" {
		publishedTime, err = parseTime(item.Updated)
	}
	if err != nil {
		return nil, fmt.Errorf("时间解析失败: %s", feedURL)
	}

	// 名称映射
	name := feed.Title
	if mapped, ok := nameMapping[name]; ok {
		name = mapped
	}

	return &Article{
		DomainName: domain,
		Name:       name,
		Title:      item.Title,
		Link:       item.Link,
		Date:       formatTime(publishedTime),
		Avatar:     avatarURL,
	}, nil
}

func (p *RSSProcessor) saveToCOS(ctx context.Context, articles []Article) error {
	// 添加十年之约入口
	articles = append(articles, Article{
		DomainName: "https://foreverblog.cn",
		Name:       "十年之约",
		Title:      "穿梭虫洞-随机访问十年之约友链博客",
		Link:       "https://foreverblog.cn/go.html",
		Date:       "January 01, 2000",
		Avatar:     "https://cos.lhasa.icu/LinksAvatar/foreverblog.cn.png",
	})

	jsonData, err := json.MarshalIndent(articles, "", "  ")
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}

	// 带重试的上传
	_, err = withRetry(ctx, p.config.MaxRetries, p.config.RetryInterval, func() (interface{}, error) {
		resp, err := p.cosClient.Object.Put(ctx, "data/rss_data.json", bytes.NewReader(jsonData), nil)
		if err != nil {
			return nil, fmt.Errorf("COS上传失败: %w", err)
		}
		defer resp.Body.Close()
		return nil, nil
	})

	if err != nil {
		return fmt.Errorf("最终上传失败: %w", err)
	}

	logAsync("INFO", fmt.Sprintf("成功上传 %d 篇文章 (%d bytes)", 
		len(articles), len(jsonData)), "success.log")
	return nil
}

// ========================
//      辅助工具函数
// ========================
func (p *RSSProcessor) fetchCOSFile(ctx context.Context, path string) (string, error) {
	resp, err := p.cosClient.Object.Get(ctx, path, nil)
	if err != nil {
		return "", fmt.Errorf("获取COS文件失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取COS内容失败: %w", err)
	}
	return string(data), nil
}

func cleanXMLContent(content string) string {
	re := regexp.MustCompile(`[\x00-\x1F\x7F-\x9F]`)
	return re.ReplaceAllString(content, "")
}

func parseTime(timeStr string) (time.Time, error) {
	for _, format := range timeFormats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间: %s", timeStr)
}

func formatTime(t time.Time) string {
	return t.Format("January 2, 2006")
}

func extractDomain(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Hostname()), nil
}

// ========================
//      日志系统实现
// ========================
func logAsync(level, message, fileName string) {
	shutdownWG.Add(1)
	logChan <- logMessage{
		level:    level,
		message:  fmt.Sprintf("[%s] %s", getBeijingTime().Format(time.RFC3339), message),
		fileName: fileName,
	}
}

func logWorker() {
	batch := make(map[string][]string)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	flush := func() {
		// 最终刷新剩余日志
		for _, msgs := range batch {
			for _, msgs := range batch {
				logToGithub(msgs, fileName)
			}
		}
	}

	for {
		select {
		case msg, ok := <-logChan:
			if !ok {
				flush()
				return
			}
			batch[msg.fileName] = append(batch[msg.fileName], msg.message)
			if len(batch[msg.fileName]) >= 50 {
				logToGithub(batch[msg.fileName], msg.fileName)
				delete(batch, msg.fileName)
			}
		case <-ticker.C:
			flush()
		}
	}
}

func logToGithub(messages []string, fileName string) {
	defer shutdownWG.Done()

	config, err := initConfig()
	if err != nil || config.GithubToken == "" {
		return
	}

	ctx := context.Background()
	client := github.NewClient(oauth2.NewClient(ctx,
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.GithubToken})))

	filePath := "_data/" + fileName
	content := strings.Join(messages, "\n") + "\n"

	// 获取现有内容
	var opts github.RepositoryContentFileOptions
	file, _, _, err := client.Repositories.GetContents(ctx,
		config.GithubName, config.GithubRepository, filePath, nil)

	if err == nil && file != nil {
		decoded, _ := file.GetContent()
		opts = github.RepositoryContentFileOptions{
			Message: github.String("日志追加: " + fileName),
			Content: []byte(decoded + "\n" + content),
			SHA:     file.SHA,
		}
	} else {
		opts = github.RepositoryContentFileOptions{
			Message: github.String("创建日志文件: " + fileName),
			Content: []byte(content),
		}
	}

	_, _, err = client.Repositories.UpdateFile(ctx,
		config.GithubName, config.GithubRepository, fileName, &opts)

	if err != nil {
		fmt.Printf("日志写入失败: %v\n", err)
	}
}

// ========================
//      环境变量助手
// ========================
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

// ========================
//      重试机制实现
// ========================
func withRetry[T any](ctx context.Context, maxRetries int, interval time.Duration, 
	fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for i := 1; i <= maxRetries; i++ {
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		logAsync("WARN", fmt.Sprintf("重试 %d/%d: %v", i, maxRetries, lastErr), "retries.log")
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return result, fmt.Errorf("操作取消: %w", ctx.Err())
		}
	}

	return result, fmt.Errorf("超过最大重试次数(%d): %w", maxRetries, lastErr)
}

// ========================
//      时间处理函数
// ========================
func getBeijingTime() time.Time {
	return time.Now().In(time.FixedZone("CST", 8*3600))
}