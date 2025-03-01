package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config 存储项目运行所需的各种配置信息
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

// InitConfig 读取环境变量并初始化配置
func InitConfig() (*Config, error) {
	cfg := &Config{
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
	required := map[string]string{
		"TENCENT_CLOUD_SECRET_ID":  cfg.SecretID,
		"TENCENT_CLOUD_SECRET_KEY": cfg.SecretKey,
		"TOKEN":                    cfg.GithubToken,
		"NAME":                     cfg.GithubName,
		"REPOSITORY":               cfg.GithubRepository,
	}
	for k, v := range required {
		if v == "" {
			return nil, fmt.Errorf("环境变量 %s 必须设置", k)
		}
	}

	return cfg, nil
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
