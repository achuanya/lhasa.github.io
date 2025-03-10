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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v39/github"
	"github.com/mmcdole/gofeed"
	"github.com/tencentyun/cos-go-sdk-v5"
	"golang.org/x/oauth2"
)

// ========================
//
//	配置结构体
//
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
//
//	全局变量/常量
//
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
		"obaby@mars": "obaby",
		"青山小站 | 一个在帝都搬砖的新时代农民工":       "青山小站",
		"Homepage on Miao Yu | 于淼":    "于淼",
		"Homepage on Yihui Xie | 谢益辉": "谢益辉",
	}

	logChanMu     sync.Mutex                    // 通道操作互斥锁
	logChanClosed bool                          // 通道关闭状态标志
	logChan       = make(chan logMessage, 1000) // 异步日志通道
	errorChan     = make(chan error, 100)       // 错误收集通道
	shutdownWG    sync.WaitGroup                // 优雅关闭等待组
)

// ========================
//
//	数据结构定义
//
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
//
//	初始化函数
//
// ========================
func init() {
	// 启动日志处理协程
	go logWorker()
}

// ========================
//
//	主程序入口
//
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

	// 安全关闭日志通道
	logChanMu.Lock()
	logChanClosed = true
	close(logChan)
	logChanMu.Unlock()

	// 带超时的等待
	done := make(chan struct{})
	go func() {
		shutdownWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("日志处理完成")
	case <-time.After(30 * time.Second):
		fmt.Println("警告：日志处理超时")
	}
}

// ========================
//
//	配置初始化
//
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
//
//	RSS处理器实现
//
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
//
//	核心业务方法
//
// ========================
func (p *RSSProcessor) loadAvatars(ctx context.Context) error {
	content, err := p.fetchCOSFile(ctx, "data/AvatarData.json")
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
	content, err := p.fetchCOSFile(ctx, "data/MyFavoriteRSS.txt")
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
	// 十年之约
	articles = append(articles, Article{
		DomainName: "https://foreverblog.cn",
		Name:       "十年之约",
		Title:      "穿梭虫洞-随机访问十年之约友链博客",
		Link:       "https://foreverblog.cn/go.html",
		Date:       "January 01, 2000",
		Avatar:     "https://cos.lhasa.icu/LinksAvatar/foreverblog.cn.png",
	})

	// 按日期倒序排序 最新在前
	sort.Slice(articles, func(i, j int) bool {
		t1, err1 := time.Parse("January 2, 2006", articles[i].Date)
		t2, err2 := time.Parse("January 2, 2006", articles[j].Date)

		// 处理解析错误 错误时间视为更早
		if err1 != nil {
			t1 = time.Time{}
		}
		if err2 != nil {
			t2 = time.Time{}
		}
		return t1.After(t2)
	})

	jsonData, err := json.Marshal(articles)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %w", err)
	}

	// 带重试的上传
	_, err = withRetry(ctx, p.config.MaxRetries, p.config.RetryInterval, func() (interface{}, error) {
		resp, err := p.cosClient.Object.Put(ctx, "data/rss.json", bytes.NewReader(jsonData), nil)
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
//
//	辅助工具函数
//
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
//
//	日志系统
//
// ========================
func logAsync(level, message, fileName string) {
	logChanMu.Lock()
	defer logChanMu.Unlock()

	if logChanClosed {
		return // 通道已关闭时不发送
	}

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
		for fileName, msgs := range batch {
			// 发送日志到 GitHub
			logToGithub(msgs, fileName)
			// 释放计数器（每条日志对应一次 Done）
			shutdownWG.Add(-len(msgs))
			delete(batch, fileName)
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
			// 批量达到 50 条时立即刷新
			if len(batch[msg.fileName]) >= 50 {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func logToGithub(messages []string, fileName string) {
	config, err := initConfig()
	if err != nil || config.GithubToken == "" {
		return
	}

	ctx := context.Background()
	client := github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.GithubToken})))
	filePath := "_data/" + fileName
	content := strings.Join(messages, "\n") + "\n"

	// 重试逻辑（最多3次）
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		file, _, _, _ := client.Repositories.GetContents(ctx, config.GithubName, config.GithubRepository, filePath, nil)

		var opts github.RepositoryContentFileOptions
		if file == nil {
			opts = github.RepositoryContentFileOptions{
				Message: github.String("创建日志文件: " + fileName),
				Content: []byte(content),
			}
		} else {
			currentContent, _ := file.GetContent()
			opts = github.RepositoryContentFileOptions{
				Message: github.String("更新日志: " + fileName),
				Content: []byte(currentContent + "\n" + content),
				SHA:     file.SHA,
			}
		}

		_, _, err = client.Repositories.UpdateFile(ctx, config.GithubName, config.GithubRepository, filePath, &opts)
		if err == nil {
			return // 成功则退出
		}

		// 处理 409 冲突错误
		if githubErr, ok := err.(*github.ErrorResponse); ok && githubErr.Response.StatusCode == http.StatusConflict {
			time.Sleep(500 * time.Millisecond) // 等待后重试
			continue
		}

		if err != nil {
			errorMsg := fmt.Sprintf("日志写入失败: %v", err)
			fmt.Println(errorMsg) // 控制台输出

			// 尝试将错误写入本地文件
			if f, err := os.OpenFile("fallback_error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
				defer f.Close()
				f.WriteString(errorMsg + "\n")
			}
		}
		return
	}
	fmt.Printf("经过 %d 次重试仍失败: %v\n", maxRetries, err)
}

// ========================
//
//	环境变量助手
//
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
//
//	重试机制实现
//
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
//
//	时间处理函数
//
// ========================
func getBeijingTime() time.Time {
	return time.Now().In(time.FixedZone("CST", 8*3600))
}
