/*
文件作用:
- 实现服务器的主程序入口
- 初始化服务器配置和组件
- 启动网络服务和GUI界面
- 管理服务器状态和配置

主要方法:
- main: 程序入口,初始化各个组件并启动服务器
- initConfig: 初始化服务器配置
- setupLogger: 设置日志记录器
- createSyncService: 创建同步服务
- handlePanic: 处理全局异常
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"synctools/codes/internal/container"
	"synctools/codes/internal/interfaces"
	"synctools/codes/internal/ui/server/viewmodels"
	"synctools/codes/internal/ui/server/windows"
)

var (
	baseDir     string
	configFile  string
	defaultPort = 8080
)

func init() {
	// 获取可执行文件所在目录
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("获取可执行文件路径失败: %v", err))
	}
	baseDir = filepath.Dir(exe)

	// 解析命令行参数
	flag.StringVar(&configFile, "config", "", "配置文件路径")
	flag.Parse()
}

func main() {
	// 创建依赖注入容器
	c, err := container.New(baseDir)
	if err != nil {
		fmt.Printf("初始化容器失败: %v\n", err)
		os.Exit(1)
	}

	// 设置日志记录器
	logger := c.GetLogger()
	logger.Info("服务器启动", interfaces.Fields{
		"baseDir": baseDir,
	})

	// 加载配置
	cfg, err := loadOrCreateConfig(c, configFile)
	if err != nil {
		logger.Error("加载配置失败", interfaces.Fields{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// 初始化服务
	if err := c.InitializeServices(baseDir, cfg); err != nil {
		logger.Error("初始化服务失败", interfaces.Fields{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// 获取同步服务
	syncService := c.GetSyncService()
	if syncService == nil {
		logger.Error("获取同步服务失败", nil)
		os.Exit(1)
	}

	// 创建视图模型
	viewModel := viewmodels.NewConfigViewModel(syncService, logger)

	// 创建主窗口
	mainWindow, err := windows.NewMainWindow(viewModel)
	if err != nil {
		logger.Error("创建主窗口失败", interfaces.Fields{
			"error": err.Error(),
		})
		os.Exit(1)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		viewModel.HandleWindowClosing()
		os.Exit(0)
	}()

	// 运行主窗口
	mainWindow.Run()
}

// loadOrCreateConfig 加载或创建默认配置
func loadOrCreateConfig(c *container.Container, configFile string) (*interfaces.Config, error) {
	cfgManager := c.GetConfigManager()
	logger := c.GetLogger()

	logger.Debug("配置操作", interfaces.Fields{
		"action": "load",
		"file":   configFile,
	})

	// 如果指定了配置文件，尝试加载
	if configFile != "" {
		logger.Debug("配置操作", interfaces.Fields{
			"action": "load_specified",
			"file":   configFile,
		})
		if err := cfgManager.LoadConfig(configFile); err != nil {
			return nil, fmt.Errorf("加载配置文件失败: %v", err)
		}
		return cfgManager.GetCurrentConfig().(*interfaces.Config), nil
	}

	// 不创建默认配置，返回空配置
	logger.Info("配置操作", interfaces.Fields{
		"action": "use_empty",
		"reason": "no_file_specified",
	})
	return nil, nil
}
