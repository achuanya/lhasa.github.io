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
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/tencentyun/cos-go-sdk-v5"
)

type Config struct {
	CosBucketURL string
	SecretID     string
	SecretKey    string
}

// 爬虫数据
type Article struct {
	// 博客名称
	Name string `json:"name"`
	// 文章标题
	Title string `json:"title"`
	// 文章链接
	Link string `json:"link"`
	// 文章发布时间，非爬虫原数据，而是格式化后的结果
	Date string `json:"date"`
}

func initConfig() Config {
	return Config{
		CosBucketURL: "https://cos.lhasa.icu",
		SecretID:     os.Getenv("COS_SECRET_ID"),
		SecretKey:    os.Getenv("COS_SECRET_KEY"),
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
		t, err := time.Parse(format, timeStr)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

// 将文章时间统一格式化，例如：July 26, 2024
func formatTime(t time.Time) string {
	return t.Format("January 2, 2006")
}

// 中国标准时间 CST，UTC+8
func getBeijingTime() time.Time {
	beijingTimeZone := time.FixedZone("CST", 8*3600)
	return time.Now().In(beijingTimeZone)
}

// 记录错误信息到 error.log 文件
func logError(config Config, message string) {
	baseURL, _ := url.Parse(config.CosBucketURL)
	b := &cos.BaseURL{BucketURL: baseURL}

	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.SecretID,
			SecretKey: config.SecretKey,
		},
		Timeout: time.Second * 30,
	})

	// 如果 error.log 文件存在则追加写入日志，否则，创建文件并写入日志
	var existingLog []byte
	resp, err := client.Object.Get(context.Background(), "rss/error.log", nil)
	if err != nil {
		// 检查错误类型是否是文件不存在错误
		if errResp, ok := err.(*cos.ErrorResponse); ok && errResp.Code == "NoSuchKey" {
			existingLog = nil
		} else {
			fmt.Printf("error downloading error.log from COS: %v\n", err)
			return
		}
	} else {
		defer resp.Body.Close()
		existingLog, _ = io.ReadAll(resp.Body)
	}

	// 将新的错误信息追加到现有日志中
	newLog := append(existingLog, []byte(message+"\n\n")...)

	// 上传更新后的 error.log 文件
	_, err = client.Object.Put(context.Background(), "rss/error.log", bytes.NewReader(newLog), nil)
	if err != nil {
		fmt.Printf("error saving error log to COS: %v\n", err)
	}
}

// 每个 RSS 只抓取一条最新的文章，并按发布时间排序
func fetchRSS(config Config, feeds []string) ([]Article, error) {
	var articles []Article
	fp := gofeed.NewParser()

	for _, feedURL := range feeds {
		resp, err := http.Get(feedURL)
		// 记录获取 RSS 源的错误信息，并写入日志
		if err != nil {
			logError(config, fmt.Sprintf("[%s] [Get RSS error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, err))
			// 跳过当前无法解析的 RSS 源，继续处理其他源
			continue
		}
		defer resp.Body.Close()

		bodyBytes := new(bytes.Buffer)
		bodyBytes.ReadFrom(resp.Body)
		bodyString := bodyBytes.String()
		cleanBody := cleanXMLContent(bodyString)

		feed, err := fp.ParseString(cleanBody)
		if err != nil {
			// 记录解析 RSS 源的错误信息，并写入日志
			logError(config, fmt.Sprintf("[%s] [Parse RSS error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, err))
			continue
		}

		// 只获取最新的一篇文章
		if len(feed.Items) > 0 {
			item := feed.Items[0]
			var publishedTime time.Time
			var err error

			// 尝试解析不同的时间字段
			if item.Published != "" {
				publishedTime, err = parseTime(item.Published)
			} else if item.Updated != "" {
				publishedTime, err = parseTime(item.Updated)
			}

			// 如果解析失败，则使用当前时间
			if err != nil {
				logError(config, fmt.Sprintf("[%s] [Getting article time error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), item.Title, err))
				publishedTime = time.Now()
			}

			articles = append(articles, Article{
				Name:  feed.Title,
				Title: item.Title,
				Link:  item.Link,
				Date:  formatTime(publishedTime),
			})
		}
	}

	// 根据发布时间对文章进行排序，最新的文章在最前面
	sort.Slice(articles, func(i, j int) bool {
		date1, _ := time.Parse("January 2, 2006", articles[i].Date)
		date2, _ := time.Parse("January 2, 2006", articles[j].Date)
		return date1.After(date2)
	})

	return articles, nil
}

// 将爬虫数据保存到 COS
func saveToCOS(config Config, data []Article) error {
	baseURL, _ := url.Parse(config.CosBucketURL)
	b := &cos.BaseURL{BucketURL: baseURL}

	client := cos.NewClient(b, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  config.SecretID,
			SecretKey: config.SecretKey,
		},
		Timeout: time.Second * 30,
	})

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = client.Object.Put(context.Background(), "rss/rss_data.json", bytes.NewReader(jsonData), nil)
	if err != nil {
		return fmt.Errorf("error saving data to COS: %v", err)
	}

	return nil
}

// 从文件中读取 RSS 源
func readFeedsFromFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	var feeds []string
	scanner := bufio.NewScanner(file)
	// 按行读取文件内容，将每一行作为 RSS 源并添加到 feeds 列表中
	for scanner.Scan() {
		feeds = append(feeds, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return feeds, nil
}

func main() {
	config := initConfig()
	// 从 rss_feeds.txt 文件中读取 RSS 源，如果报错，则记录到日志
	rssFeeds, err := readFeedsFromFile("data/rss_feeds.txt")
	if err != nil {
		logError(config, fmt.Sprintf("[%s] [Read RSS feeds error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), err))
		fmt.Printf("Error reading RSS feeds from file: %v\n", err)
		return
	}

	// 抓取 RSS 源，如果报错，则记录到日志
	articles, err := fetchRSS(config, rssFeeds)
	if err != nil {
		logError(config, fmt.Sprintf("[%s] [Fetch RSS error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), err))
		fmt.Printf("Error fetching RSS feeds: %v\n", err)
		return
	}

	// 保存文章到 COS，如果报错，则记录到日志
	err = saveToCOS(config, articles)
	if err != nil {
		logError(config, fmt.Sprintf("[%s] [Save data to COS error] %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), err))
		fmt.Printf("Error saving data to COS: %v\n", err)
		return
	}

	fmt.Println("Stop writing code and go ride a road bike now!")
}
