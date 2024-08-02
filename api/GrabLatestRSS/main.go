package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/google/go-github/v39/github"
	"github.com/mmcdole/gofeed"
	"golang.org/x/oauth2"
)

const maxRetries = 3
const retryInterval = 2 * time.Second

type Config struct {
	GithubToken      string
	GithubName       string
	GithubRepository string
}

// 用于解析 avatar_data.json 文件
type Avatar struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// 爬虫数据
type Article struct {
	// 域名
	DomainName string `json:"domainName"`
	// 博客名称
	Name string `json:"name"`
	// 文章标题
	Title string `json:"title"`
	// 文章链接
	Link string `json:"link"`
	// 文章发布时间，非爬虫原数据，而是格式化后的结果
	Date string `json:"date"`
	// 头像
	Avatar string `json:"avatar"`
}

func initConfig() Config {
	return Config{
		// GitHub API 令牌
		GithubToken: os.Getenv("TOKEN"),
		// GitHub 用户名
		GithubName: "achuanya",
		// GitHub 仓库名
		GithubRepository: "lhasa.github.io",
	}
}

// 清理 XML 内容中的非法字符
func cleanXMLContent(content string) string {
	re := regexp.MustCompile(`[\x00-\x1F\x7F-\x9F]`)
	return re.ReplaceAllString(content, "")
}

// 解析文章时间字段
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

// 将文章时间统一格式化，例如：July 26, 2024
func formatTime(t time.Time) string {
	return t.Format("January 2, 2006")
}

// 提取域名并加上 https:// 前缀
func extractDomain(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	domain := u.Hostname()
	protocol := "https://"
	if u.Scheme != "" {
		protocol = u.Scheme + "://"
	}
	fullURL := protocol + domain

	return fullURL, nil
}

// 中国标准时间 CST，UTC+8
func getBeijingTime() time.Time {
	beijingTimeZone := time.FixedZone("CST", 8*3600)
	return time.Now().In(beijingTimeZone)
}

// 记录错误信息到 error.log 文件
func logError(config Config, message string) {
	logMessage(config, message, "error.log")
}

// 记录错误信息到 error.log 文件
func logMessage(config Config, message string, fileName string) {
	// 控制请求周期
	ctx := context.Background()

	// 使用 OAuth2 进行验证
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: config.GithubToken,
	})))

	// 定义文件路径
	filePath := "api/GrabLatestRSS/" + fileName
	fileContent := []byte(message + "\n\n")

	// 尝试获取 error.log 文件
	file, _, resp, err := client.Repositories.GetContents(ctx, config.GithubName, config.GithubRepository, filePath, nil)

	// 检查文件是否存在，如果不存在则创建新文件并写入日志
	if err != nil && resp.StatusCode == http.StatusNotFound {

		// 文件不存在，创建新文件
		_, _, err := client.Repositories.CreateFile(ctx, config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
			// 文件名
			Message: github.String("Create " + fileName),
			// 数据
			Content: fileContent,
			// 分支
			Branch: github.String("master"),
		})
		if err != nil {
			fmt.Printf("error creating %s in GitHub: %v\n", fileName, err)
		}
		return
	} else if err != nil {
		fmt.Printf("error checking %s in GitHub: %v\n", fileName, err)
		return
	}

	// 如果文件存在，则获取文件内容并追加日志
	decodedContent, err := file.GetContent()
	if err != nil {
		fmt.Printf("error decoding %s content: %v\n", fileName, err)
		return
	}

	// 将新日志追加到现有内容后面
	updatedContent := append([]byte(decodedContent), fileContent...)

	// 更新文件内容，将新的日志追加到文件中
	_, _, err = client.Repositories.UpdateFile(ctx, config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
		Message: github.String("Update " + fileName),
		Content: updatedContent,
		SHA:     github.String(*file.SHA),
		Branch:  github.String("master"),
	})
	if err != nil {
		fmt.Printf("error updating %s in GitHub: %v\n", fileName, err)
	}
}

// 从 GitHub 仓库中获取 JSON 文件内容
func fetchFileFromGitHub(config Config, filePath string) (string, error) {
	ctx := context.Background()
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: config.GithubToken,
	})))

	file, _, resp, err := client.Repositories.GetContents(ctx, config.GithubName, config.GithubRepository, filePath, nil)
	if err != nil {
		if resp.StatusCode == http.StatusNotFound {
			return "", fmt.Errorf("file not found: %s", filePath)
		}
		return "", fmt.Errorf("error fetching file %s from GitHub: %v", filePath, err)
	}

	content, err := file.GetContent()
	if err != nil {
		return "", fmt.Errorf("error decoding file %s content: %v", filePath, err)
	}

	return content, nil
}

// 从 GitHub 仓库中读取头像配置
func loadAvatarsFromGitHub(config Config) (map[string]string, error) {
	content, err := fetchFileFromGitHub(config, "_data/avatar_data.json") // 从 GitHub 仓库中获取 avatar_data.json
	if err != nil {
		return nil, err
	}

	var avatars []Avatar
	if err := json.Unmarshal([]byte(content), &avatars); err != nil {
		return nil, err
	}

	avatarMap := make(map[string]string)
	for _, a := range avatars {
		avatarMap[a.Name] = a.Avatar
	}

	return avatarMap, nil
}

// 从 RSS 列表中抓取最新的文章，并按发布时间排序
func fetchRSS(config Config, feeds []string) ([]Article, error) {
	var articles []Article

	// 从 GitHub 仓库中读取头像配置
	avatars, err := loadAvatarsFromGitHub(config)
	if err != nil {
		logError(config, fmt.Sprintf("[%s] [Load avatars error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), err))
		return nil, err
	}

	// 截断
	nameMapping := map[string]string{
		"obaby@mars": "obaby",
		"青山小站 | 一个在帝都搬砖的新时代农民工":       "青山小站",
		"Homepage on Miao Yu | 于淼":    "于淼",
		"Homepage on Yihui Xie | 谢益辉": "谢益辉",
	}

	// RSS 解析器
	fp := gofeed.NewParser()
	for _, feedURL := range feeds {
		var resp *http.Response
		var bodyString string
		var fetchErr error

		// 尝试获取 RSS 内容，添加重试逻辑
		for i := 0; i < maxRetries; i++ {
			resp, fetchErr = http.Get(feedURL)
			if fetchErr == nil {
				bodyBytes := new(bytes.Buffer)
				bodyBytes.ReadFrom(resp.Body)
				bodyString = bodyBytes.String()
				resp.Body.Close()
				break
			}
			// 记录获取 RSS 失败的日志，并等待一段时间后重试
			logError(config, fmt.Sprintf("[%s] [Get RSS error] %s: Attempt %d/%d: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, i+1, maxRetries, fetchErr))
			time.Sleep(retryInterval)
		}

		if fetchErr != nil {
			// 如果所有重试都失败，记录失败日志并跳过当前 RSS
			logError(config, fmt.Sprintf("[%s] [Failed to fetch RSS] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, fetchErr))
			continue
		}

		// 清理 XML 内容中的非法字符
		cleanBody := cleanXMLContent(bodyString)

		// 尝试解析 RSS 内容，添加重试逻辑
		var feed *gofeed.Feed
		var parseErr error
		for i := 0; i < maxRetries; i++ {
			feed, parseErr = fp.ParseString(cleanBody)
			if parseErr == nil {
				break
			}
			// 记录解析 RSS 错误的日志，并等待一段时间后重试
			logError(config, fmt.Sprintf("[%s] [Parse RSS error] %s: Attempt %d/%d: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, i+1, maxRetries, parseErr))
			time.Sleep(retryInterval)
		}

		if parseErr != nil {
			// 如果所有重试都失败，记录失败日志并跳过当前 RSS
			logError(config, fmt.Sprintf("[%s] [Failed to parse RSS] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, parseErr))
			continue
		}

		// 使用 feed.Link 作为主网站 URL
		mainSiteURL := feed.Link

		// 提取主网站的域名
		domainName, err := extractDomain(mainSiteURL)
		if err != nil {
			logError(config, fmt.Sprintf("[%s] [Extract domain error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), mainSiteURL, err))
			// 如果提取失败，使用默认值
			domainName = "unknown"
		}

		// 使用 feed.Title 作为博客名称
		name := feed.Title

		// 检查名称映射
		if mappedName, ok := nameMapping[name]; ok {
			name = mappedName
		}

		// 获取头像
		avatarURL := avatars[name]
		if avatarURL == "" {

			// 默认头像
			avatarURL = "https://cos.lhasa.icu/LinksAvatar/default.png"
		}

		// 只获取最新的一篇文章
		if len(feed.Items) > 0 {
			item := feed.Items[0]

			// 尝试解析不同的时间字段
			publishedTime, err := parseTime(item.Published)
			if err != nil && item.Updated != "" {
				publishedTime, err = parseTime(item.Updated)
			}

			// 获取文章时间错误，写入日志
			if err != nil {
				logError(config, fmt.Sprintf("[%s] [Getting article time error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), item.Title, err))

				// 使用当前时间作为文章时间
				publishedTime = time.Now()
			}

			articles = append(articles, Article{
				DomainName: domainName,
				Name:       name,
				Title:      item.Title,
				Link:       item.Link,
				Avatar:     avatarURL,

				// 格式化后的发布时间
				Date: formatTime(publishedTime),
			})
		}
	}

	// 根据发布时间对文章进行排序，最新的文章在最前面
	sort.Slice(articles, func(i, j int) bool {
		date1, _ := time.Parse("January 2, 2006", articles[i].Date)
		date2, _ := time.Parse("January 2, 2006", articles[j].Date)

		// 按照文章时间降序排序
		return date1.After(date2)
	})

	return articles, nil
}

// 将爬虫抓取的数据保存到 GitHub
func saveToGitHub(config Config, data []Article) error {
	ctx := context.Background()
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: config.GithubToken,
	})))

	// 固定数据
	manualArticles := []Article{
		{
			DomainName: "https://foreverblog.cn",
			Name:       "十年之约",
			Title:      "穿梭虫洞-随机访问十年之约友链博客",
			Link:       "https://foreverblog.cn/go.html",
			Date:       "January 01, 2000",
			Avatar:     "https://cos.lhasa.icu/LinksAvatar/foreverblog.cn.png",
		},
		{
			DomainName: "https://www.travellings.cn",
			Name:       "开往",
			Title:      "开往-友链接力",
			Link:       "https://www.travellings.cn/go.html",
			Date:       "January 01, 2000",
			Avatar:     "https://cos.lhasa.icu/LinksAvatar/www.travellings.png",
		},
	}

	data = append(data, manualArticles...)

	// 将文章数据序列化为 JSON 格式
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// fmt.Printf("Saving data to GitHub: %s\n", string(jsonData))

	filePath := "_data/rss_data.json"
	file, _, resp, err := client.Repositories.GetContents(ctx, config.GithubName, config.GithubRepository, filePath, nil)
	if err != nil && resp.StatusCode == http.StatusNotFound {

		// 如果文件不存在，则创建新文件
		_, _, err := client.Repositories.CreateFile(ctx, config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
			Message: github.String("Create rss_data.json"),
			Content: jsonData,
			Branch:  github.String("master"),
		})

		// 创建 rss_data.json 文件错误，写入日志
		if err != nil {
			return fmt.Errorf("error creating rss_data.json in GitHub: %v", err)
		}

		// 文件创建成功，返回 nil
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking rss_data.json in GitHub: %v", err)
	}

	_, _, err = client.Repositories.UpdateFile(ctx, config.GithubName, config.GithubRepository, filePath, &github.RepositoryContentFileOptions{
		Message: github.String("Update rss_data.json"),
		Content: jsonData,
		SHA:     github.String(*file.SHA),
		Branch:  github.String("master"),
	})
	if err != nil {
		return fmt.Errorf("error updating rss_data.json in GitHub: %v", err)
	}

	return nil
}

// 从 GitHub 仓库中获取 RSS 文件
func readFeedsFromGitHub(config Config) ([]string, error) {
	ctx := context.Background()
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: config.GithubToken,
	})))

	filePath := "_data/rss_feeds.txt"
	file, _, resp, err := client.Repositories.GetContents(ctx, config.GithubName, config.GithubRepository, filePath, nil)

	// 如果文件不存在，记录错误信息并返回错误
	if err != nil && resp.StatusCode == http.StatusNotFound {
		errMsg := fmt.Sprintf("Error: %s not found in GitHub repository", filePath)
		logError(config, fmt.Sprintf("[%s] [Read RSS file error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), errMsg))
		return nil, fmt.Errorf(errMsg)
	} else if err != nil {
		// 如果获取文件时发生其他错误，记录错误信息并返回错误
		errMsg := fmt.Sprintf("Error fetching %s from GitHub: %v", filePath, err)
		logError(config, fmt.Sprintf("[%s] [Read RSS file error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), errMsg))
		return nil, fmt.Errorf(errMsg)
	}

	// 获取文件内容
	content, err := file.GetContent()
	if err != nil {
		errMsg := fmt.Sprintf("Error decoding %s content: %v", filePath, err)
		logError(config, fmt.Sprintf("[%s] [Read RSS file error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), errMsg))
		return nil, fmt.Errorf(errMsg)
	}

	var feeds []string
	scanner := bufio.NewScanner(bytes.NewReader([]byte(content)))

	// 按行读取文件内容，将每一行作为 RSS 并添加到 feeds 列表中
	for scanner.Scan() {
		feeds = append(feeds, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		errMsg := fmt.Sprintf("Error reading RSS file content: %v", err)
		logError(config, fmt.Sprintf("[%s] [Read RSS file error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), errMsg))
		return nil, fmt.Errorf(errMsg)
	}

	return feeds, nil
}

func main() {

	githubToken := os.Getenv("TOKEN")
	fmt.Printf("GitHub Token: %s\n", githubToken)
	// 其他代码

	config := initConfig()

	// 从 GitHub 仓库中读取 RSS
	rssFeeds, err := readFeedsFromGitHub(config)
	if err != nil {
		logError(config, fmt.Sprintf("[%s] [Read RSS feeds error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), err))
		fmt.Printf("Error reading RSS feeds from GitHub: %v\n", err)
		return
	}

	// 抓取 RSS
	articles, err := fetchRSS(config, rssFeeds)
	if err != nil {
		logError(config, fmt.Sprintf("[%s] [Fetch RSS error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), err))
		fmt.Printf("Error fetching RSS feeds: %v\n", err)
		return
	}

	// 将爬虫数据保存到 Github
	err = saveToGitHub(config, articles)
	if err != nil {
		logError(config, fmt.Sprintf("[%s] [Save data to GitHub error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), err))
		fmt.Printf("Error saving data to GitHub: %v\n", err)
		return
	}

	fmt.Println("Stop writing code and go ride a road bike now!")
}
