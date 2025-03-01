package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"runtime/debug"
)

var (
	logChan       = make(chan string, 2000) // 日志消息通道
	wg            sync.WaitGroup            // 用于等待日志写入完成
	logFile       *os.File                  // 当前日志文件句柄
	logFileMu     sync.Mutex                // 并发写文件互斥锁
	currentLogDay string                    // 当前日志文件对应的日期（yyyy-mm-dd）
)

// InitLogger 初始化日志系统：
// 1. 创建 logs 文件夹
// 2. 删除 7 天前的日志
// 3. 打开当天日志文件
// 4. 启动写日志的协程
func InitLogger() error {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return err
	}

	// 删除 7 天前的日志
	if err := cleanOldLogs(7); err != nil {
		return err
	}

	// 打开当天文件
	if err := openLogFileForToday(); err != nil {
		return err
	}

	// 启动消费日志的协程
	go logWorker()

	return nil
}

// CloseLogger 关闭日志通道，等待所有日志写入后再关闭文件
func CloseLogger() {
	close(logChan)
	wg.Wait()
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// LogAsync 异步写日志
func LogAsync(level, message string) {
	// 加一个等待计数，保证在 CloseLogger 时能够等待它写完
	wg.Add(1)
	t := time.Now().Format("2006-01-02 15:04:05")
	logChan <- fmt.Sprintf("[%s] [%s] %s", t, level, message)
}

// LogPanic 用于在 `recover()` 时写入 panic 相关的日志和堆栈
func LogPanic(r interface{}) {
	LogAsync("PANIC", fmt.Sprintf("panic: %v\n%s", r, debug.Stack()))
}

// logWorker 从通道读取日志内容并写入文件
func logWorker() {
	defer func() {
		// 协程结束前，尽量写完剩余内容
		flushAllLogs()
	}()

	ticker := time.NewTicker(3 * time.Second) // 3 秒节流一次
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-logChan:
			if !ok {
				return
			}
			writeLog(msg)
		case <-ticker.C:
			// 定期检查日期是否变更
			rotateIfNeeded()
		}
	}
}

// writeLog 写单条日志到文件
func writeLog(msg string) {
	logFileMu.Lock()
	defer logFileMu.Unlock()

	if logFile == nil {
		// 理论上不会出现，但加一下保护
		return
	}

	_, _ = logFile.WriteString(msg + "\n")
	wg.Done()
}

// flushAllLogs 将通道内剩余日志全部写完
func flushAllLogs() {
	for {
		select {
		case msg, ok := <-logChan:
			if !ok {
				return
			}
			writeLog(msg)
		default:
			return
		}
	}
}

// rotateIfNeeded 如果日期变了，就切换到新日志文件
func rotateIfNeeded() {
	today := time.Now().Format("2006-01-02")

	logFileMu.Lock()
	defer logFileMu.Unlock()

	if currentLogDay != today {
		// 先关闭旧文件
		if logFile != nil {
			logFile.Close()
			logFile = nil
		}
		// 再打开新文件
		_ = openLogFileForToday()
	}
}

// openLogFileForToday 根据当前日期打开（或创建）对应日志文件
func openLogFileForToday() error {
	today := time.Now().Format("2006-01-02")
	filename := filepath.Join("logs", today+".log")

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	logFile = f
	currentLogDay = today
	return nil
}

// cleanOldLogs 清理 N 天前的日志文件
func cleanOldLogs(days int) error {
	entries, err := os.ReadDir("logs")
	if err != nil {
		return err
	}

	threshold := time.Now().AddDate(0, 0, -days)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 例如 2025-03-01.log
		if len(name) < len("2006-01-02.log") {
			continue
		}
		datePart := name[:10] // 截取 "YYYY-MM-DD"
		t, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if t.Before(threshold) {
			_ = os.Remove(filepath.Join("logs", name))
		}
	}
	return nil
}
