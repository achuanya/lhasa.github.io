package rss

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"lhasaRSS/internal/config"
	"lhasaRSS/internal/logger"
	"lhasaRSS/internal/utils"

	"github.com/mmcdole/gofeed"
	"github.com/tencentyun/cos-go-sdk-v5"
)

// RSSProcessor 处理 RSS 的结构体
type RSSProcessor struct {
	config     *config.Config
	httpClient *http.Client
	cosClient  *cos.Client
	parser     *gofeed.Parser
	avatarMap  map[string]string
}

// Article 用于存储文章信息
type Article struct {
	DomainName string `json:"domainName"`
	Name       string `json:"name"`
	Title      string `json:"title"`
	Link       string `json:"link"`
	Date       string `json:"date"`
	Avatar     string `json:"avatar"`
}

// AvatarData 用于解析 AvatarData.json
type AvatarData struct {
	DomainName string `json:"domainName"`
	Name       string `json:"name"`
	Avatar     string `json:"avatar"`
}

// RunSummary 运行完成后的统计数据，用于最后的总结打印
type RunSummary struct {
	TotalRSS       int           // 总计需要处理的 RSS 数量
	SuccessCount   int           // 成功数量
	FailCount      int           // 失败数量
	ParseFailCount int           // 解析时间等失败
	MissingAvatar  int           // 找不到头像
	DefaultAvatar  int           // 使用默认头像
	FailedList     []string      // 失败列表
	StartTime      time.Time     // 开始时间
	EndTime        time.Time     // 结束时间
	Elapsed        time.Duration // 耗时
}

// NewRSSProcessor 创建新的 RSSProcessor
func NewRSSProcessor(cfg *config.Config) *RSSProcessor {
	transport := &http.Transport{
		MaxIdleConns:        cfg.MaxConcurrency * 2,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		MaxConnsPerHost:     cfg.MaxConcurrency,
		MaxIdleConnsPerHost: cfg.MaxConcurrency,
	}

	// 初始化 COS 客户端
	u, _ := url.Parse(cfg.COSURL)
	cosClient := cos.NewClient(&cos.BaseURL{BucketURL: u}, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cfg.SecretID,
			SecretKey: cfg.SecretKey,
		},
	})

	return &RSSProcessor{
		config:     cfg,
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout, Transport: transport},
		cosClient:  cosClient,
		parser:     gofeed.NewParser(),
		avatarMap:  make(map[string]string),
	}
}

// Close 用于在 main() 退出前关闭相关连接
func (p *RSSProcessor) Close() {
	p.httpClient.CloseIdleConnections()
}

// Run 核心流程，返回统计结果
func (p *RSSProcessor) Run(ctx context.Context) (*RunSummary, error) {
	summary := &RunSummary{StartTime: time.Now()}

	// 加载头像数据
	if err := p.loadAvatars(ctx); err != nil {
		return summary, fmt.Errorf("加载头像数据失败: %w", err)
	}

	// 获取订阅列表
	feeds, err := p.getFeeds(ctx)
	if err != nil {
		return summary, fmt.Errorf("获取订阅列表失败: %w", err)
	}
	summary.TotalRSS = len(feeds)

	// 并发抓取 RSS
	articles, failList, parseFailCount := p.fetchAllRSS(ctx, feeds)
	summary.FailCount = len(failList)
	summary.ParseFailCount = parseFailCount
	summary.SuccessCount = len(articles)
	summary.FailedList = failList

	// 统计头像情况
	// （示例给出的需求中并未强制统计“找不到头像”与“使用默认头像”逻辑，这里演示一下）
	// 根据 p.avatarMap 与结果来判断
	var missingAvatar, defaultAvatar int
	for _, art := range articles {
		if art.Avatar == "" {
			missingAvatar++
		} else if strings.Contains(art.Avatar, "default.png") {
			defaultAvatar++
		}
	}
	summary.MissingAvatar = missingAvatar
	summary.DefaultAvatar = defaultAvatar

	// 保存数据到 COS
	if err := p.saveToCOS(ctx, articles); err != nil {
		logger.LogAsync("ERROR", "保存到COS失败: "+err.Error())
	}

	// 填充时间
	summary.EndTime = time.Now()
	summary.Elapsed = summary.EndTime.Sub(summary.StartTime)
	return summary, nil
}

// PrintRunSummary 输出类似你给出的模板到日志文件
func PrintRunSummary(summary *RunSummary) {
	if summary == nil {
		return
	}
	logger.LogAsync("INFO", "本次运行完成！")
	logger.LogAsync("INFO", fmt.Sprintf("总计需要处理的 RSS 数量：%d", summary.TotalRSS))
	logger.LogAsync("INFO", fmt.Sprintf("成功：%d, 失败：%d（解析失败：%d）",
		summary.SuccessCount, summary.FailCount, summary.ParseFailCount))
	logger.LogAsync("INFO", fmt.Sprintf("找不到头像：%d, 使用默认头像：%d",
		summary.MissingAvatar, summary.DefaultAvatar))

	if len(summary.FailedList) > 0 {
		logger.LogAsync("INFO", "本次运行的【失败明细】如下：")
		for _, f := range summary.FailedList {
			// f 本身是类似"抓取失败(http://xxx): some error..."
			logger.LogAsync("INFO", " - "+f)
		}
	}
	logger.LogAsync("INFO", fmt.Sprintf("本次执行总耗时：%v", summary.Elapsed))
	logger.LogAsync("INFO", "程序执行结束。")
}

// ========================================
// 以下是内部具体实现
// ========================================

func (p *RSSProcessor) loadAvatars(ctx context.Context) error {
	content, err := p.fetchCOSFile(ctx, "data/AvatarData.json")
	if err != nil {
		return err
	}

	var avatarData []AvatarData
	if err := json.Unmarshal([]byte(content), &avatarData); err != nil {
		return fmt.Errorf("解析头像数据失败: %w", err)
	}

	for _, a := range avatarData {
		domain, e := utils.ExtractDomain(a.DomainName)
		if e == nil && domain != "" {
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

// fetchAllRSS 并发获取所有 RSS，返回文章切片、失败列表、解析失败次数
func (p *RSSProcessor) fetchAllRSS(ctx context.Context, feeds []string) ([]Article, []string, int) {
	var (
		articles       []Article
		failList       []string
		parseFailCount int
		mutex          sync.Mutex
	)

	feedChan := make(chan string, len(feeds))
	var wg sync.WaitGroup

	// 创建工作池
	for i := 0; i < p.config.MaxConcurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for feedURL := range feedChan {
				art, err := p.processFeed(ctx, feedURL)
				mutex.Lock()
				if err != nil {
					failList = append(failList, fmt.Sprintf("抓取失败(%s): %v", feedURL, err))
					if strings.Contains(err.Error(), "时间解析失败") {
						parseFailCount++
					}
				} else {
					articles = append(articles, *art)
				}
				mutex.Unlock()
			}
		}()
	}

	// 分发任务
	for _, f := range feeds {
		feedChan <- f
	}
	close(feedChan)

	wg.Wait()
	return articles, failList, parseFailCount
}

func (p *RSSProcessor) processFeed(ctx context.Context, feedURL string) (*Article, error) {
	// 获取并清理内容
	body, err := utils.WithRetry(ctx, p.config.MaxRetries, p.config.RetryInterval, func() (string, error) {
		resp, err := p.httpClient.Get(feedURL)
		if err != nil {
			return "", fmt.Errorf("HTTP 请求失败: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("非200状态码: %d", resp.StatusCode)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取响应体失败: %w", err)
		}

		return utils.CleanXMLContent(string(data)), nil
	})
	if err != nil {
		return nil, fmt.Errorf("获取源失败 %s: %w", feedURL, err)
	}

	// 解析 RSS
	feed, err := utils.WithRetry(ctx, p.config.MaxRetries, p.config.RetryInterval, func() (*gofeed.Feed, error) {
		return p.parser.ParseString(body)
	})
	if err != nil {
		return nil, fmt.Errorf("解析源失败 %s: %w", feedURL, err)
	}

	// 域名
	domain, e := utils.ExtractDomain(feed.Link)
	if e != nil {
		domain = "unknown"
		logger.LogAsync("WARN", fmt.Sprintf("域名解析失败 %s: %v", feed.Link, e))
	}

	// 头像
	avatarURL := p.avatarMap[domain]
	if avatarURL == "" {
		avatarURL = "https://cos.lhasa.icu/LinksAvatar/default.png"
	}

	// 取最新文章
	if len(feed.Items) == 0 {
		return nil, fmt.Errorf("没有找到文章: %s", feedURL)
	}
	item := feed.Items[0]

	publishedTime, e := utils.ParseTime(item.Published)
	if e != nil && item.Updated != "" {
		publishedTime, e = utils.ParseTime(item.Updated)
	}
	if e != nil {
		return nil, fmt.Errorf("时间解析失败: %s", feedURL)
	}

	// 名称映射
	name := feed.Title
	if mapped, ok := utils.NameMapping[name]; ok {
		name = mapped
	}

	return &Article{
		DomainName: domain,
		Name:       name,
		Title:      item.Title,
		Link:       item.Link,
		Date:       utils.FormatTime(publishedTime),
		Avatar:     avatarURL,
	}, nil
}

// saveToCOS 将数据保存到 COS
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

	// 按日期倒序排序，最新在前
	sort.Slice(articles, func(i, j int) bool {
		t1, err1 := time.Parse("January 2, 2006", articles[i].Date)
		t2, err2 := time.Parse("January 2, 2006", articles[j].Date)
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
	_, err = utils.WithRetry(ctx, p.config.MaxRetries, p.config.RetryInterval, func() (interface{}, error) {
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

	logger.LogAsync("INFO", fmt.Sprintf("成功上传 %d 篇文章 (%d bytes)", len(articles), len(jsonData)))
	return nil
}

// fetchCOSFile 直接从 COS 获取文件内容
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
