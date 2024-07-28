---
layout: post
title: 利用Go+Github Actions写个定时RSS爬虫
date: 2024-07-27 09:50:01 +0800
category: tech
thumb: ARTICLEPICTURES_PATH/1722043714404.jpg
tags: [Go, Github Actions, COS, 爬虫]
---

说起这事，还是受一位博友的启发<a href="https://1900.live/links" target="_blank">“1900”</a>他的左邻右舍页面很棒，决定模仿一下。我平时也用 Inoreader，但我还是喜欢直接打开博客的感觉，心血来潮，搞。

起初，我打算使用 COS 和 GitHub Actions，但在测试过程中发现 GitHub 的延迟非常高，验证和文件写入速度极慢，频频失败。干脆直接上 GitHub 自产自销。

## 大致思路
```plaintext
main()
│
├── readFeedsFromGitHub()
│   ├── GitHub API 调用
│   │   ├── 读取 rss_feeds.txt 文件
│   │   └── 处理文件报错
│   └── Return
│
├── fetchRSS()
│   ├── 遍历 RSS
│   │   ├── HTTP GET 请求
│   │   └── 处理请求错误
│   ├── 解析 RSS
│   │   ├── 清理 XML 内容中的非法字符
│   │   ├── 提取域名
│   │   └── 格式化并排序
│   └── Return
│
└── saveToGitHub()
    ├── GitHub API 调用
    │   ├── 保存到 _data/rss_data.json 供 Jekyll 调用
    │   └── 处理错误
    └── Return
```

由于用 Go 搬砖，所有的包、类型和方法均可在 GitHub API 客户端库的第 39 版文档查询

关于 Github API 有一点需要注意，配置好环境变量后，Token 操作仓库需要有一定的权限，务必启用 Read and write permissions 读取和写入权限，进行相应的操作


```go
go mod init github.com/achuanya/Grab-latest-RSS
// Go-GitHub v39
go get github.com/google/go-github/v39/github
// RSS 和 Atom feeds 解析库
go get github.com/mmcdole/gofeed
// OAuth2 认证和授权
go get golang.org/x/oauth2
```

## Go RSS 爬虫 Code

```go
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

type Config struct {
	GithubToken      string
	GithubName       string
	GithubRepository string
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
	filePath := "api/" + fileName
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

// 从 RSS 列表中抓取最新的文章，并按发布时间排序
func fetchRSS(config Config, feeds []string) ([]Article, error) {
	var articles []Article

	// RSS 解析器
	fp := gofeed.NewParser()

	for _, feedURL := range feeds {
		resp, err := http.Get(feedURL)

		// 获取 RSS 错误，写入日志
		if err != nil {
			logError(config, fmt.Sprintf("[%s] [Get RSS error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, err))

			// 跳过当前无法解析的 RSS
			continue
		}
		defer resp.Body.Close()

		bodyBytes := new(bytes.Buffer)
		bodyBytes.ReadFrom(resp.Body)
		bodyString := bodyBytes.String()

		// 清理 XML 内容中的非法字符
		cleanBody := cleanXMLContent(bodyString)
		feed, err := fp.ParseString(cleanBody)
		if err != nil {

			// 解析 RSS 错误，写入日志
			logError(config, fmt.Sprintf("[%s] [Parse RSS error] %s: %v", getBeijingTime().Format("Mon Jan 2 15:04:2006"), feedURL, err))
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
				Name:       feed.Title,
				Title:      item.Title,
				Link:       item.Link,

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

	filePath := "api/rss_feeds.txt"
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
```

### Go 生成的 json 数据
```json
[
    {
        "domainName": "https://lingxiankong.github.io",
        "name": "孔令贤的个人博客",
        "title": "关于 AI 的一些思考",
        "link": "https://lingxiankong.github.io/2024-05-15-openai-gpt-4o.html",
        "date": "May 15, 2024"
    },
    {
        "domainName": "https://blog.fooleap.org",
        "name": "Fooleap's Blog",
        "title": "渴望理想",
        "link": "https://blog.fooleap.org/longing-for-ideals.html",
        "date": "August 15, 2020"
    }
]
```

### Go 生成的日志

```log
[Sat Jul 27 08:42:2024] [Parse RSS error] https://lhasa.icu: Failed to detect feed type

[Sat Jul 27 08:41:2024] [Get RSS error] https://lhasa.icu: Get "https://lhasa.icu": net/http: TLS handshake timeout
```


## Github Actons 1h/次

```yml
name: ScheduledRssRawler

on:
  schedule:
    - cron: '0 * * * *'
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v3

    - name: Set up Go
      uses: actions/setup-go@v3
      with:
        go-version: '1.22.5'

    - name: Install dependencies
      run: go mod tidy
      working-directory: ./api

    - name: Build
      run: go build -o main
      working-directory: ./api

    - name: Run Go program
      env:
        TOKEN: ${{ secrets.KEY }}
      run: ./main
      working-directory: ./api

```

腾讯 COS 也写了一份，Github 有延迟问题就没用，也能用，逻辑上和 Go 是没啥区别

<a href="https://github.com/achuanya/Grab-latest-RSS" target="_blank">Grab-latest-RSS：https://github.com/achuanya/Grab-latest-RSS</a>

<a href="https://cloud.tencent.com/document/product/436/31215" target="_blank">COS Go SDK：https://cloud.tencent.com/document/product/436/31215</a>

[效果页：https://lhasa.icu/links.html](https://lhasa.icu/links.html)