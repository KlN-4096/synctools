/*
文件作用:
- 测试同步服务的各种同步模式
- 验证同步功能的正确性
- 测试文件完整性检查
- 测试进度回调功能
*/

package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"synctools/internal/interfaces"
	"synctools/pkg/service"
	"synctools/pkg/storage"
)

// TestSyncModes 测试不同的同步模式
func TestSyncModes(t *testing.T) {
	// 创建测试目录
	testDir := filepath.Join(os.TempDir(), "synctools_test")
	srcDir := filepath.Join(testDir, "src")
	dstDir := filepath.Join(testDir, "dst")

	// 清理测试目录
	os.RemoveAll(testDir)
	defer os.RemoveAll(testDir)

	// 创建源目录和目标目录
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("创建源目录失败: %v", err)
	}
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("创建目标目录失败: %v", err)
	}

	// 创建测试文件
	testFiles := map[string][]byte{
		"test1.txt":           []byte("这是测试文件1的内容"),
		"test2.exe":           []byte("这是测试文件2的内容"),
		"test3.zip":           []byte("这是测试文件3的内容"),
		"subfolder/test4.txt": []byte("这是子文件夹中测试文件4的内容"),
	}

	// 写入测试文件
	for name, content := range testFiles {
		filePath := filepath.Join(srcDir, name)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("写入测试文件失败: %v", err)
		}
	}

	// 创建存储服务
	fileStorage, err := storage.NewFileStorage(dstDir)
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 创建日志记录器
	logger := &TestLogger{
		t:     t,
		level: interfaces.DEBUG,
	}

	// 创建同步服务
	syncService := service.NewSyncService(&interfaces.Config{
		SyncDir: dstDir,
		SyncFolders: []interfaces.SyncFolder{
			{
				Path:     "test1.txt",
				SyncMode: interfaces.PushSync,
			},
			{
				Path:     "test2.exe",
				SyncMode: interfaces.MirrorSync,
			},
			{
				Path:     "test3.zip",
				SyncMode: interfaces.PackSync,
			},
			{
				Path:     "subfolder",
				SyncMode: interfaces.AutoSync,
			},
		},
	}, logger, fileStorage)

	// 复制源文件到存储服务
	for name, content := range testFiles {
		if err := fileStorage.Save(name, content); err != nil {
			t.Fatalf("复制源文件失败: %v", err)
		}
	}

	// 设置进度回调
	progressChan := make(chan interfaces.Progress, 100)
	syncService.SetProgressCallback(func(p *interfaces.Progress) {
		select {
		case progressChan <- *p:
		default:
		}
	})

	// 测试推送同步
	t.Run("PushSync", func(t *testing.T) {
		if err := syncService.SyncFiles("test1.txt"); err != nil {
			t.Errorf("推送同步失败: %v", err)
		}

		// 验证文件是否正确同步
		content, err := os.ReadFile(filepath.Join(dstDir, "test1.txt"))
		if err != nil {
			t.Errorf("读取同步后的文件失败: %v", err)
		} else if string(content) != string(testFiles["test1.txt"]) {
			t.Errorf("文件内容不匹配")
		}
	})

	// 测试镜像同步
	t.Run("MirrorSync", func(t *testing.T) {
		if err := syncService.SyncFiles("test2.exe"); err != nil {
			t.Errorf("镜像同步失败: %v", err)
		}

		// 验证文件是否正确同步
		content, err := os.ReadFile(filepath.Join(dstDir, "test2.exe"))
		if err != nil {
			t.Errorf("读取同步后的文件失败: %v", err)
		} else if string(content) != string(testFiles["test2.exe"]) {
			t.Errorf("文件内容不匹配")
		}
	})

	// 测试打包同步
	t.Run("PackSync", func(t *testing.T) {
		if err := syncService.SyncFiles("test3.zip"); err != nil {
			t.Errorf("打包同步失败: %v", err)
		}

		// 验证压缩文件是否创建
		zipPath := filepath.Join(dstDir, "test3.zip")
		if _, err := os.Stat(zipPath); os.IsNotExist(err) {
			t.Errorf("压缩文件未创建")
		}
	})

	// 测试自动同步
	t.Run("AutoSync", func(t *testing.T) {
		if err := syncService.SyncFiles("subfolder"); err != nil {
			t.Errorf("自动同步失败: %v", err)
		}

		// 验证子文件夹中的文件是否正确同步
		content, err := os.ReadFile(filepath.Join(dstDir, "subfolder/test4.txt"))
		if err != nil {
			t.Errorf("读取同步后的文件失败: %v", err)
		} else if string(content) != string(testFiles["subfolder/test4.txt"]) {
			t.Errorf("文件内容不匹配")
		}
	})

	// 测试进度回调
	t.Run("ProgressCallback", func(t *testing.T) {
		// 等待所有进度更新
		timeout := time.After(5 * time.Second)
		progressCount := 0

		for {
			select {
			case p := <-progressChan:
				t.Logf("进度更新: %d/%d - %s", p.Current, p.Total, p.FileName)
				progressCount++
			case <-timeout:
				if progressCount == 0 {
					t.Error("没有收到进度更新")
				}
				return
			}
		}
	})
}

// TestServiceStartStop 测试服务的启动和停止
func TestServiceStartStop(t *testing.T) {
	// 创建测试目录
	testDir := filepath.Join(os.TempDir(), "synctools_test")
	defer os.RemoveAll(testDir)

	// 创建存储服务
	fileStorage, err := storage.NewFileStorage(testDir)
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
	}

	// 创建日志记录器
	logger := &TestLogger{
		t:     t,
		level: interfaces.DEBUG,
	}

	// 创建同步服务
	syncService := service.NewSyncService(&interfaces.Config{
		SyncDir: testDir,
	}, logger, fileStorage)

	// 测试初始状态
	if syncService.IsRunning() {
		t.Error("服务初始状态应该是未运行")
	}

	// 测试启动服务
	if err := syncService.Start(); err != nil {
		t.Errorf("启动服务失败: %v", err)
	}

	if !syncService.IsRunning() {
		t.Error("服务启动后状态应该是运行中")
	}

	// 测试同步文件
	if err := syncService.SyncFiles(testDir); err != nil {
		t.Errorf("同步文件失败: %v", err)
	}

	// 测试停止服务
	if err := syncService.Stop(); err != nil {
		t.Errorf("停止服务失败: %v", err)
	}

	if syncService.IsRunning() {
		t.Error("服务停止后状态应该是未运行")
	}
}

// TestLogger 测试用的日志记录器
type TestLogger struct {
	t     *testing.T
	level interfaces.LogLevel
}

func (l *TestLogger) Debug(msg string, fields interfaces.Fields) {
	l.t.Logf("[DEBUG] %s %v", msg, fields)
}

func (l *TestLogger) Info(msg string, fields interfaces.Fields) {
	l.t.Logf("[INFO] %s %v", msg, fields)
}

func (l *TestLogger) Warn(msg string, fields interfaces.Fields) {
	l.t.Logf("[WARN] %s %v", msg, fields)
}

func (l *TestLogger) Error(msg string, fields interfaces.Fields) {
	l.t.Logf("[ERROR] %s %v", msg, fields)
}

func (l *TestLogger) Fatal(msg string, fields interfaces.Fields) {
	l.t.Fatalf("[FATAL] %s %v", msg, fields)
}

// GetLevel 获取日志级别
func (l *TestLogger) GetLevel() interfaces.LogLevel {
	return l.level
}

// SetLevel 设置日志级别
func (l *TestLogger) SetLevel(level interfaces.LogLevel) {
	l.level = level
}

// WithFields 返回带有字段的日志记录器
func (l *TestLogger) WithFields(fields interfaces.Fields) interfaces.Logger {
	return l
}
