package model_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// mockLogWriter 用于测试的日志写入器
type mockLogWriter struct {
	mu      sync.Mutex
	entries []string
}

func newMockLogWriter() *mockLogWriter {
	return &mockLogWriter{
		entries: make([]string, 0),
	}
}

func (w *mockLogWriter) Write(entry string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = append(w.entries, entry)
}

func (w *mockLogWriter) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = w.entries[:0]
}

func (w *mockLogWriter) GetEntries() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]string, len(w.entries))
	copy(result, w.entries)
	return result
}

// testLogger 测试用的日志记录器
type testLogger struct {
	writer      *mockLogWriter
	debugMode   bool
	timePattern string // 用于验证时间戳格式
}

func newTestLogger(writer *mockLogWriter) *testLogger {
	return &testLogger{
		writer:      writer,
		timePattern: `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`,
	}
}

func (l *testLogger) Log(format string, v ...interface{}) {
	l.writer.Write(fmt.Sprintf(format, v...))
}

func (l *testLogger) Info(msg string, args ...interface{}) {
	l.writer.Write(fmt.Sprintf("[INFO] "+msg, args...))
}

func (l *testLogger) Error(msg string, args ...interface{}) {
	l.writer.Write(fmt.Sprintf("[ERROR] "+msg, args...))
}

func (l *testLogger) Debug(msg string, args ...interface{}) {
	l.writer.Write(fmt.Sprintf("[DEBUG] "+msg, args...))
}

func (l *testLogger) DebugLog(format string, v ...interface{}) {
	if l.debugMode {
		l.writer.Write(fmt.Sprintf("[DEBUG] "+format, v...))
	}
}

func (l *testLogger) SetDebugMode(enabled bool) {
	l.debugMode = enabled
}

func (l *testLogger) GetDebugMode() bool {
	return l.debugMode
}

// TestLogger_BasicLogging 测试基本日志记录功能
func TestLogger_BasicLogging(t *testing.T) {
	writer := newMockLogWriter()
	logger := newTestLogger(writer)

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{
			name: "记录普通信息",
			logFunc: func() {
				logger.Log("普通消息")
			},
			expected: "普通消息",
		},
		{
			name: "记录带参数的普通信息",
			logFunc: func() {
				logger.Log("消息 %d: %s", 1, "测试")
			},
			expected: "消息 1: 测试",
		},
		{
			name: "记录信息级别日志",
			logFunc: func() {
				logger.Info("信息消息")
			},
			expected: "[INFO] 信息消息",
		},
		{
			name: "记录错误级别日志",
			logFunc: func() {
				logger.Error("错误消息")
			},
			expected: "[ERROR] 错误消息",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer.Clear()
			tt.logFunc()
			entries := writer.GetEntries()
			if len(entries) != 1 {
				t.Errorf("期望 1 条日志记录，实际得到 %d 条", len(entries))
				return
			}
			if !strings.Contains(entries[0], tt.expected) {
				t.Errorf("日志内容不匹配\n期望包含: %s\n实际得到: %s", tt.expected, entries[0])
			}
		})
	}
}

// TestLogger_DebugMode 测试调试模式控制
func TestLogger_DebugMode(t *testing.T) {
	writer := newMockLogWriter()
	logger := newTestLogger(writer)

	// 测试禁用调试模式
	logger.SetDebugMode(false)
	logger.DebugLog("调试消息1")
	if len(writer.GetEntries()) != 0 {
		t.Error("调试模式禁用时不应记录调试消息")
	}

	// 测试启用调试模式
	logger.SetDebugMode(true)
	logger.DebugLog("调试消息2")
	entries := writer.GetEntries()
	if len(entries) != 1 {
		t.Error("调试模式启用时应记录调试消息")
	} else if !strings.Contains(entries[0], "调试消息2") {
		t.Errorf("调试消息内容不匹配，got: %s", entries[0])
	}

	// 测试调试模式状态查询
	if !logger.GetDebugMode() {
		t.Error("GetDebugMode() 应返回 true")
	}
}

// TestLogger_FormatMessages 测试日志格式化
func TestLogger_FormatMessages(t *testing.T) {
	writer := newMockLogWriter()
	logger := newTestLogger(writer)

	tests := []struct {
		name     string
		logFunc  func()
		expected string
	}{
		{
			name: "格式化字符串",
			logFunc: func() {
				logger.Log("数字: %d, 字符串: %s", 42, "test")
			},
			expected: "数字: 42, 字符串: test",
		},
		{
			name: "格式化错误消息",
			logFunc: func() {
				err := fmt.Errorf("测试错误")
				logger.Error("发生错误: %v", err)
			},
			expected: "[ERROR] 发生错误: 测试错误",
		},
		{
			name: "格式化带多个参数的消息",
			logFunc: func() {
				logger.Info("用户 %s 在 %s 执行了 %s 操作", "admin", "2023-01-01", "登录")
			},
			expected: "[INFO] 用户 admin 在 2023-01-01 执行了 登录 操作",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer.Clear()
			tt.logFunc()
			entries := writer.GetEntries()
			if len(entries) != 1 {
				t.Errorf("期望 1 条日志记录，实际得到 %d 条", len(entries))
				return
			}
			if !strings.Contains(entries[0], tt.expected) {
				t.Errorf("日志内容不匹配\n期望包含: %s\n实际得到: %s", tt.expected, entries[0])
			}
		})
	}
}

// TestLogger_ConcurrentLogging 测试并发日志记录
func TestLogger_ConcurrentLogging(t *testing.T) {
	writer := newMockLogWriter()
	logger := newTestLogger(writer)

	var wg sync.WaitGroup
	numGoroutines := 10
	numLogsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numLogsPerGoroutine; j++ {
				logger.Info("来自 goroutine %d 的消息 %d", id, j)
			}
		}(i)
	}

	wg.Wait()

	entries := writer.GetEntries()
	expectedCount := numGoroutines * numLogsPerGoroutine
	if len(entries) != expectedCount {
		t.Errorf("期望 %d 条日志记录，实际得到 %d 条", expectedCount, len(entries))
	}
}

// TestLogger_LevelFiltering 测试日志级别过滤
func TestLogger_LevelFiltering(t *testing.T) {
	writer := newMockLogWriter()
	logger := newTestLogger(writer)

	// 禁用调试模式时的测试
	logger.SetDebugMode(false)
	logger.Info("信息消息")
	logger.Error("错误消息")
	logger.DebugLog("调试消息")

	entries := writer.GetEntries()
	if len(entries) != 2 { // 应该只有 INFO 和 ERROR 消息
		t.Errorf("期望 2 条日志记录，实际得到 %d 条", len(entries))
	}

	// 检查是否包含正确的消息
	hasInfo := false
	hasError := false
	hasDebug := false
	for _, entry := range entries {
		if strings.Contains(entry, "[INFO]") {
			hasInfo = true
		}
		if strings.Contains(entry, "[ERROR]") {
			hasError = true
		}
		if strings.Contains(entry, "[DEBUG]") {
			hasDebug = true
		}
	}

	if !hasInfo {
		t.Error("缺少 INFO 级别的日志")
	}
	if !hasError {
		t.Error("缺少 ERROR 级别的日志")
	}
	if hasDebug {
		t.Error("不应该有 DEBUG 级别的日志")
	}

	// 启用调试模式后的测试
	writer.Clear()
	logger.SetDebugMode(true)
	logger.Info("信息消息")
	logger.Error("错误消息")
	logger.DebugLog("调试消息")

	entries = writer.GetEntries()
	if len(entries) != 3 { // 现在应该有所有三种消息
		t.Errorf("期望 3 条日志记录，实际得到 %d 条", len(entries))
	}
}
