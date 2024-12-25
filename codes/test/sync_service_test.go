/*
文件作用:
- 测试同步服务的各种同步模式
- 验证同步功能的正确性
- 测试文件完整性检查
- 测试进度回调功能
*/

package test

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"synctools/codes/internal/interfaces"
	"synctools/codes/internal/ui/client/viewmodels"
	"synctools/codes/pkg/network"
	"synctools/codes/pkg/service"
	"synctools/codes/pkg/storage"
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

	// 创建日志记录器
	logger := &TestLogger{
		t:     t,
		level: interfaces.DEBUG,
	}

	// 创建存储服务
	fileStorage, err := storage.NewFileStorage(dstDir, logger)
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
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

	// 创建日志记录器
	logger := &TestLogger{
		t:     t,
		level: interfaces.DEBUG,
	}

	// 创建存储服务
	fileStorage, err := storage.NewFileStorage(testDir, logger)
	if err != nil {
		t.Fatalf("创建文件存储失败: %v", err)
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

// TestSyncWithConfig 测试带配置的同步功能
func TestSyncWithConfig(t *testing.T) {
	// 使用指定的测试目录
	serverRoot := "G:\\test\\server"
	clientRoot := "G:\\test\\client"

	// 使用随机高端口
	port := 50000 + rand.Intn(10000)

	// 创建服务端配置
	serverConfig := &interfaces.Config{
		UUID:    "500a59d565c795e7d49d089836aff8ec",
		Type:    "server",
		Name:    "AA",
		Version: "1.0.0",
		Host:    "0.0.0.0",
		Port:    port,
		SyncDir: serverRoot,
		SyncFolders: []interfaces.SyncFolder{
			{
				Path:     "aaa",
				SyncMode: "mirror",
				PackMD5:  "",
			},
			{
				Path:     "bbb",
				SyncMode: "mirror",
				PackMD5:  "",
			},
		},
		IgnoreList: []string{
			".clientconfig",
			".DS_Store",
			"thumbs.db",
		},
		FolderRedirects: []interfaces.FolderRedirect{
			{
				ServerPath: "clientmods",
				ClientPath: "mods",
			},
			{
				ServerPath: "aaa",
				ClientPath: "aaB",
			},
		},
	}

	// 创建客户端配置
	clientConfig := &interfaces.Config{
		UUID:    "default",
		Type:    "client",
		Name:    "SyncTools Client",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    port,
		SyncDir: clientRoot,
	}

	// 创建测试文件
	testFiles := map[string][]byte{
		"aaa/test1.txt": []byte("test1 content"),
		"aaa/test2.txt": []byte("test2 content"),
		"bbb/test3.txt": []byte("test3 content"),
		"bbb/test4.txt": []byte("test4 content"),
	}

	// 创建服务端目录结构和文件
	for path, content := range testFiles {
		fullPath := filepath.Join(serverRoot, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		err = os.WriteFile(fullPath, content, 0644)
		if err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}
	}

	// 创建日志记录器
	logger := NewTestLogger(t)

	// 创建存储接口
	serverStorage, err := storage.NewFileStorage(serverRoot, logger)
	if err != nil {
		t.Fatalf("创建服务端存储失败: %v", err)
	}
	clientStorage, err := storage.NewFileStorage(clientRoot, logger)
	if err != nil {
		t.Fatalf("创建客户端存储失败: %v", err)
	}

	// 创建服务端同步服务
	serverService := service.NewSyncService(serverConfig, logger, serverStorage)
	if err := serverService.Start(); err != nil {
		t.Fatalf("启动服务端服务失败: %v", err)
	}
	defer serverService.Stop()

	// 启动服务端网络服务
	if err := serverService.StartServer(); err != nil {
		t.Fatalf("启动服务端网络服务失败: %v", err)
	}

	// 创建客户端同步服务
	clientService := service.NewSyncService(clientConfig, logger, clientStorage)
	if err := clientService.Start(); err != nil {
		t.Fatalf("启动客户端服务失败: %v", err)
	}
	defer clientService.Stop()

	// 创建同步请求
	syncRequest := &interfaces.SyncRequest{
		Path:    clientRoot,
		Storage: clientStorage,
	}

	// 执行同步
	if err := serverService.HandleSyncRequest(syncRequest); err != nil {
		t.Fatalf("处理同步请求失败: %v", err)
	}

	// 验证同步结果
	for path, expectedContent := range testFiles {
		// 根据重定向规则调整路径
		clientPath := path
		for _, redirect := range serverConfig.FolderRedirects {
			if strings.HasPrefix(path, redirect.ServerPath) {
				clientPath = strings.Replace(path, redirect.ServerPath, redirect.ClientPath, 1)
				break
			}
		}

		// 检查文件是否存在
		fullPath := filepath.Join(clientRoot, clientPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("读取同步后的文件失败: %v", err)
			continue
		}

		// 验证文件内容
		if !bytes.Equal(content, expectedContent) {
			t.Errorf("文件内容不匹配 path=%s\nexpected: %s\nactual: %s", clientPath, expectedContent, content)
		}
	}
}

// TestLogger 测试用的日志记录器
type TestLogger struct {
	t     *testing.T
	level interfaces.LogLevel
}

func (l *TestLogger) Debug(msg string, fields interfaces.Fields) {
	l.t.Logf("[DEBUG] %s %+v", msg, fields)
}

func (l *TestLogger) Info(msg string, fields interfaces.Fields) {
	l.t.Logf("[INFO] %s %+v", msg, fields)
}

func (l *TestLogger) Warn(msg string, fields interfaces.Fields) {
	l.t.Logf("[WARN] %s %+v", msg, fields)
}

func (l *TestLogger) Error(msg string, fields interfaces.Fields) {
	l.t.Logf("[ERROR] %s %+v", msg, fields)
}

func (l *TestLogger) Fatal(msg string, fields interfaces.Fields) {
	l.t.Fatalf("[FATAL] %s %+v", msg, fields)
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

// NewTestLogger 创建用于测试的日志记录器
func NewTestLogger(t *testing.T) interfaces.Logger {
	return &TestLogger{
		t:     t,
		level: interfaces.INFO,
	}
}

// TestSyncButton 测试点击同步按钮的逻辑
func TestSyncButton(t *testing.T) {
	// 创建测试目录
	serverRoot := "G:\\test\\server"
	clientRoot := "G:\\test\\client"

	// 创建目录
	if err := os.MkdirAll(serverRoot, 0755); err != nil {
		t.Fatalf("创建服务器目录失败: %v", err)
	}
	if err := os.MkdirAll(clientRoot, 0755); err != nil {
		t.Fatalf("创建客户端目录失败: %v", err)
	}

	// 创建测试文件
	testFiles := map[string][]byte{
		"aaa/test1.txt": []byte("test1 content"),
		"aaa/test2.txt": []byte("test2 content"),
		"bbb/test3.txt": []byte("test3 content"),
		"bbb/test4.txt": []byte("test4 content"),
	}

	// 创建服务器端的文件
	for path, content := range testFiles {
		fullPath := filepath.Join(serverRoot, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}
	}

	// 创建日志记录器
	logger := NewTestLogger(t)

	// 创建服务器端存储
	serverStorage, err := storage.NewFileStorage(serverRoot, logger)
	if err != nil {
		t.Fatalf("创建服务器端存储失败: %v", err)
	}

	// 创建服务器端配置
	serverConfig := &interfaces.Config{
		UUID:    "AA",
		Name:    "测试服务器",
		Version: "1.0.0",
		Host:    "0.0.0.0",
		Port:    50108,
		SyncDir: serverRoot,
		SyncFolders: []interfaces.SyncFolder{
			{
				Path:     "aaa",
				SyncMode: interfaces.MirrorSync,
			},
			{
				Path:     "bbb",
				SyncMode: interfaces.MirrorSync,
			},
		},
		FolderRedirects: []interfaces.FolderRedirect{
			{
				ServerPath: "aaa",
				ClientPath: "aaB",
			},
		},
	}

	// 创建服务器端同步服务
	serverService := service.NewSyncService(serverConfig, logger, serverStorage)
	if err := serverService.Start(); err != nil {
		t.Fatalf("启动服务器端服务失败: %v", err)
	}
	defer serverService.Stop()

	// 创建客户端配置
	clientConfig := &interfaces.Config{
		UUID:    "BB",
		Name:    "测试客户端",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    50108,
		SyncDir: clientRoot,
	}

	// 创建客户端存储
	clientStorage, err := storage.NewFileStorage(clientRoot, logger)
	if err != nil {
		t.Fatalf("创建客户端存储失败: %v", err)
	}

	// 创建客户端同步服务
	clientService := service.NewSyncService(clientConfig, logger, clientStorage)
	if err := clientService.Start(); err != nil {
		t.Fatalf("启动客户端服务失败: %v", err)
	}
	defer clientService.Stop()

	// 创建同步请求
	syncRequest := &interfaces.SyncRequest{
		Path:    clientRoot,
		Storage: clientStorage,
	}

	// 执行同步
	if err := serverService.HandleSyncRequest(syncRequest); err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 验证同步结果
	for path, expectedContent := range testFiles {
		// 根据重定向规则调整路径
		clientPath := path
		for _, redirect := range serverConfig.FolderRedirects {
			if strings.HasPrefix(path, redirect.ServerPath) {
				clientPath = strings.Replace(path, redirect.ServerPath, redirect.ClientPath, 1)
				break
			}
		}

		// 检查文件是否存在
		fullPath := filepath.Join(clientRoot, clientPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("读取同步后的文件失败 %s: %v", fullPath, err)
			continue
		}

		// 验证文件内容
		if !bytes.Equal(content, expectedContent) {
			t.Errorf("文件内容不匹配 %s: 期望 %s, 实际 %s", clientPath, expectedContent, content)
		}
	}

	// 检查目录结构
	expectedDirs := []string{
		filepath.Join(clientRoot, "aaB"),
		filepath.Join(clientRoot, "bbb"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("目录不存在: %s", dir)
		}
	}
}

// TestStartSyncMethod 测试实际的StartSync方法
func TestStartSyncMethod(t *testing.T) {
	// 创建测试目录
	serverRoot := "G:\\test\\server"
	clientRoot := "G:\\test\\client"

	// 创建目录
	if err := os.MkdirAll(serverRoot, 0755); err != nil {
		t.Fatalf("创建服务器目录失败: %v", err)
	}
	if err := os.MkdirAll(clientRoot, 0755); err != nil {
		t.Fatalf("创建客户端目录失败: %v", err)
	}

	// 创建测试文件
	testFiles := map[string][]byte{
		"aaa/test1.txt": []byte("test1 content"),
		"aaa/test2.txt": []byte("test2 content"),
		"bbb/test3.txt": []byte("test3 content"),
		"bbb/test4.txt": []byte("test4 content"),
	}

	// 创建服务器端的文件
	for path, content := range testFiles {
		fullPath := filepath.Join(serverRoot, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}
	}

	// 创建日志记录器
	logger := NewTestLogger(t)

	// 创建服务器端存储
	serverStorage, err := storage.NewFileStorage(serverRoot, logger)
	if err != nil {
		t.Fatalf("创建服务器端存储失败: %v", err)
	}

	// 检查服务器端文件列表
	serverFiles, err := serverStorage.List()
	if err != nil {
		t.Fatalf("获取服务器端文件列表失败: %v", err)
	}
	t.Logf("服务器端文件列表: %v", serverFiles)

	// 创建服务器端配置
	serverConfig := &interfaces.Config{
		UUID:    "AA",
		Name:    "测试服务器",
		Version: "1.0.0",
		Host:    "0.0.0.0",
		Port:    50108,
		SyncDir: serverRoot,
		SyncFolders: []interfaces.SyncFolder{
			{
				Path:     "aaa",
				SyncMode: interfaces.MirrorSync,
			},
			{
				Path:     "bbb",
				SyncMode: interfaces.MirrorSync,
			},
		},
		FolderRedirects: []interfaces.FolderRedirect{
			{
				ServerPath: "aaa",
				ClientPath: "aaB",
			},
		},
	}

	// 创建服务器端同步服务
	serverService := service.NewSyncService(serverConfig, logger, serverStorage)
	if err := serverService.Start(); err != nil {
		t.Fatalf("启动服务器端服务失败: %v", err)
	}
	defer serverService.Stop()

	// 创建并启动网络服务
	networkService := network.NewServer(serverConfig, serverService, logger)
	if err := networkService.Start(); err != nil {
		t.Fatalf("启动网络服务失败: %v", err)
	}
	defer networkService.Stop()

	// 创建客户端配置
	clientConfig := &interfaces.Config{
		UUID:    "BB",
		Name:    "测试客户端",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    50108,
		SyncDir: clientRoot,
	}

	// 创建客户端存储
	clientStorage, err := storage.NewFileStorage(clientRoot, logger)
	if err != nil {
		t.Fatalf("创建客户端存储失败: %v", err)
	}

	// 创建客户端同步服务
	clientService := service.NewSyncService(clientConfig, logger, clientStorage)
	if err := clientService.Start(); err != nil {
		t.Fatalf("启动客户端服务失败: %v", err)
	}
	defer clientService.Stop()

	// 创建视图模型
	viewModel := viewmodels.NewMainViewModel(clientService, logger)

	// 设置同步路径
	viewModel.SetSyncPath(clientRoot)

	// 连接到服务器
	if err := viewModel.Connect(); err != nil {
		t.Fatalf("连接服务器失败: %v", err)
	}
	defer viewModel.Disconnect()

	// 执行StartSync方法
	if err := viewModel.StartSync(); err != nil {
		t.Fatalf("StartSync失败: %v", err)
	}

	// 等待同步完成
	time.Sleep(time.Second)

	// 检查客户端文件列表
	clientFiles, err := clientStorage.List()
	if err != nil {
		t.Fatalf("获取客户端文件列表失败: %v", err)
	}
	t.Logf("客户端文件列表: %v", clientFiles)

	// 验证同步结果
	for path, expectedContent := range testFiles {
		// 根据重定向规则调整路径
		clientPath := path
		for _, redirect := range serverConfig.FolderRedirects {
			if strings.HasPrefix(path, redirect.ServerPath) {
				clientPath = strings.Replace(path, redirect.ServerPath, redirect.ClientPath, 1)
				break
			}
		}

		// 检查文件是否存在
		fullPath := filepath.Join(clientRoot, clientPath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("读取同步后的文件失败 %s: %v", fullPath, err)
			continue
		}

		// 验证文件内容
		if !bytes.Equal(content, expectedContent) {
			t.Errorf("文件内容不匹配 %s: 期望 %s, 实际 %s", clientPath, expectedContent, content)
		}
	}

	// 检查目录结构
	expectedDirs := []string{
		filepath.Join(clientRoot, "aaB"),
		filepath.Join(clientRoot, "bbb"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("目录不存在: %s", dir)
		}
	}
}
