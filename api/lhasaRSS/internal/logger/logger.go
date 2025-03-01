package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"
)

var (
	// 可通过修改 LogLevel = "DEBUG"/"INFO"/"WARN"/"ERROR" 来控制日志输出的粒度
	LogLevel      = "INFO"
	logChan       = make(chan string, 2000)
	wg            sync.WaitGroup
	logFile       *os.File
	logFileMu     sync.Mutex
	currentLogDay string
	levelPriority = map[string]int{"DEBUG": 1, "INFO": 2, "WARN": 3, "ERROR": 4}
)

// InitLogger 初始化日志系统
func InitLogger() error {
	// 1. 创建 logs 文件夹
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建logs文件夹失败: %v", err)
	}
	// 2. 删除 7 天前日志
	if err := cleanOldLogs(7); err != nil {
		return fmt.Errorf("清理旧日志失败: %v", err)
	}
	// 3. 打开当天日志文件
	if err := openLogFileForToday(); err != nil {
		return fmt.Errorf("打开日志文件失败: %v", err)
	}
	// 4. 启动日志写入协程
	go logWorker()
	return nil
}

// CloseLogger 关闭通道，等待剩余日志写完，关闭文件
func CloseLogger() {
	close(logChan)
	wg.Wait()
	logFileMu.Lock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	logFileMu.Unlock()
}

// LogAsync 写异步日志，带级别过滤
func LogAsync(level, message string) {
	// 先看是否应该记录该级别
	if !shouldLog(level) {
		return
	}

	wg.Add(1)
	t := time.Now().Format("2006-01-02 15:04:05")
	// 你想要什么格式，这里随意
	formatted := fmt.Sprintf("[%s] [%s] %s", t, level, message)
	logChan <- formatted
}

// LogPanic 用于在 recover() 时记录 panic
func LogPanic(r interface{}) {
	LogAsync("ERROR", fmt.Sprintf("panic: %v\n%s", r, debug.Stack()))
}

// 判断是否需要记录某个级别的日志
func shouldLog(level string) bool {
	if levelPriority[level] >= levelPriority[LogLevel] {
		return true
	}
	return false
}

// ----------------------
// 以下是内部实现
// ----------------------

func logWorker() {
	defer flushAllLogs()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-logChan:
			if !ok {
				return
			}
			writeLog(msg)
		case <-ticker.C:
			rotateIfNeeded()
		}
	}
}

func writeLog(msg string) {
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if logFile == nil {
		return
	}
	_, _ = logFile.WriteString(msg + "\n")
	wg.Done()
}

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

func rotateIfNeeded() {
	today := time.Now().Format("2006-01-02")
	if currentLogDay == today {
		return
	}
	// 日期变了就重新打开文件
	logFileMu.Lock()
	defer logFileMu.Unlock()
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	_ = openLogFileForToday()
}

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

func cleanOldLogs(days int) error {
	entries, err := os.ReadDir("logs")
	if err != nil {
		return err
	}
	threshold := time.Now().AddDate(0, 0, -days)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < len("2006-01-02.log") {
			continue
		}
		datePart := name[:10]
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
