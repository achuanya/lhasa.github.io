package utils

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"lhasaRSS/internal/logger"
	"net/url"
)

/*
@author: 游钓四方 <haibao1027@gmail.com>
@function: WithRetry 使用指数退避算法对 fn 进行重试
@params:
  - ctx: 上下文
  - maxRetries: 最大重试次数
  - baseInterval: 初始等待间隔
  - fn: 需要执行的函数

@return:
  - T: fn 的返回结果
  - error: 最终失败错误

@explanation:

	第 i 次重试失败后，等待 baseInterval * 2^(i-1) 的时长，再继续尝试。
*/
func WithRetry[T any](ctx context.Context, maxRetries int, baseInterval time.Duration, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	interval := baseInterval

	for i := 1; i <= maxRetries; i++ {
		result, lastErr = fn()
		if lastErr == nil {
			return result, nil
		}

		logger.LogAsync("WARN", fmt.Sprintf("重试 %d/%d: %v", i, maxRetries, lastErr))

		if i < maxRetries {
			select {
			case <-time.After(interval):
				interval = interval * 2 // 指数退避
			case <-ctx.Done():
				return result, fmt.Errorf("操作取消: %w", ctx.Err())
			}
		}
	}

	return result, fmt.Errorf("超过最大重试次数(%d): %w", maxRetries, lastErr)
}

// CleanXMLContent 去除响应体中的无效字符
func CleanXMLContent(content string) string {
	re := regexp.MustCompile(`[\x00-\x1F\x7F-\x9F]`)
	return re.ReplaceAllString(content, "")
}

// ParseTime 解析时间
func ParseTime(timeStr string) (time.Time, error) {
	timeFormats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123Z,
		time.RFC1123,
	}
	for _, format := range timeFormats {
		if t, err := time.Parse(format, timeStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("无法解析时间: %s", timeStr)
}

// FormatTime 将 time.Time 格式化为 January 2, 2006
func FormatTime(t time.Time) string {
	return t.Format("January 2, 2006")
}

// ExtractDomain 提取 URL 的协议 + 域名
func ExtractDomain(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "https"
	}
	return fmt.Sprintf("%s://%s", u.Scheme, u.Hostname()), nil
}

// NameMapping RSS Title 到更简短名称的映射
var NameMapping = map[string]string{
	"obaby@mars": "obaby",
	"青山小站 | 一个在帝都搬砖的新时代农民工":       "青山小站",
	"Homepage on Miao Yu | 于淼":    "于淼",
	"Homepage on Yihui Xie | 谢益辉": "谢益辉",
}
