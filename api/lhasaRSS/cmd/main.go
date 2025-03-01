package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"lhasaRSS/internal/config"
	"lhasaRSS/internal/logger"
	"lhasaRSS/internal/rss"
)

func main() {
	// 初始化日志（会删除 7 天前的日志，并准备当天的日志文件）
	if err := logger.InitLogger(); err != nil {
		fmt.Println("日志初始化失败:", err)
		os.Exit(1)
	}
	// 确保程序退出前，日志都能正确写入
	defer logger.CloseLogger()

	// 捕获最外层 panic，写入同一个日志文件
	defer func() {
		if r := recover(); r != nil {
			logger.LogPanic(r)
			// panic 之后，这里可根据需求决定是否继续向上抛或者强制退出
			os.Exit(1)
		}
	}()

	// 初始化配置
	cfg, err := config.InitConfig()
	if err != nil {
		logger.LogAsync("ERROR", "初始化配置失败: "+err.Error())
		os.Exit(1)
	}

	// 创建 RSS 处理器
	processor := rss.NewRSSProcessor(cfg)
	defer processor.Close()

	// 设置总超时 3 分钟
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 执行核心逻辑
	runSummary, err := processor.Run(ctx)
	if err != nil {
		logger.LogAsync("ERROR", "执行任务时出错: "+err.Error())
	}

	// 输出运行总结到日志
	rss.PrintRunSummary(runSummary)

	// 程序正常退出
	logger.LogAsync("INFO", "程序执行结束。")
	fmt.Println("程序执行结束。")
}
