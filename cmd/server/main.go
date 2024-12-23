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

	"synctools/internal/container"
	"synctools/internal/interfaces"
	"synctools/internal/ui"
	"synctools/internal/ui/viewmodels"
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

	// 获取日志服务
	logger := c.GetLogger()
	logger.SetLevel(interfaces.DEBUG)
	logger.Info("服务器启动", interfaces.Fields{
		"base_dir": baseDir,
	})

	// 加载或创建配置
	cfg, err := loadOrCreateConfig(c, configFile)
	if err != nil {
		logger.Fatal("加载配置失败", interfaces.Fields{
			"error": err,
		})
	}

	// 初始化所有服务
	if err := c.InitializeServices(baseDir, cfg); err != nil {
		logger.Fatal("初始化服务失败", interfaces.Fields{
			"error": err,
		})
	}

	// 创建主视图模型
	mainViewModel := viewmodels.NewMainViewModel(c.GetSyncService(), logger)

	// 创建退出通道
	exitChan := make(chan struct{})

	// 创建并运行主窗口（在新的 goroutine 中）
	go func() {
		if err := ui.CreateMainWindow(mainViewModel); err != nil {
			logger.Fatal("创建主窗口失败", interfaces.Fields{
				"error": err,
			})
		}
		// 窗口关闭时发送退出信号
		close(exitChan)
	}()

	// 等待中断信号或窗口关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		logger.Info("收到中断信号", nil)
	case <-exitChan:
		logger.Info("窗口已关闭", nil)
	}

	// 优雅关闭
	logger.Info("开始关闭服务器...", nil)
	if err := c.Shutdown(); err != nil {
		logger.Error("关闭服务失败", interfaces.Fields{
			"error": err,
		})
	}
	logger.Info("服务器已关闭", nil)
	os.Exit(0)
}

// loadOrCreateConfig 加载或创建默认配置
func loadOrCreateConfig(c *container.Container, configFile string) (*interfaces.Config, error) {
	cfgManager := c.GetConfigManager()
	storage := c.GetStorage()
	logger := c.GetLogger()

	logger.Debug("开始加载配置", interfaces.Fields{
		"configFile": configFile,
	})

	// 如果指定了配置文件，尝试加载
	if configFile != "" {
		logger.Debug("尝试加载指定的配置文件", interfaces.Fields{
			"configFile": configFile,
		})
		if err := cfgManager.LoadConfig(configFile); err != nil {
			return nil, fmt.Errorf("加载配置文件失败: %v", err)
		}
		return cfgManager.GetCurrentConfig().(*interfaces.Config), nil
	}

	// 检查是否存在默认配置
	logger.Debug("检查默认配置文件", interfaces.Fields{
		"exists": storage.Exists("default.json"),
	})
	if storage.Exists("default.json") {
		if err := cfgManager.LoadConfig("default"); err != nil {
			return nil, fmt.Errorf("加载默认配置失败: %v", err)
		}
		cfg := cfgManager.GetCurrentConfig().(*interfaces.Config)
		logger.Debug("已加载默认配置", interfaces.Fields{
			"config": cfg,
		})
		return cfg, nil
	}

	// 创建默认配置
	cfg := &interfaces.Config{
		UUID:    "default",
		Type:    interfaces.ConfigTypeServer,
		Name:    "SyncTools Server",
		Version: "1.0.0",
		Host:    "0.0.0.0",
		Port:    defaultPort,
		SyncDir: filepath.Join(baseDir, "sync"),
	}

	logger.Info("创建默认配置", interfaces.Fields{
		"config": cfg,
	})

	if err := cfgManager.SaveConfig(cfg); err != nil {
		return nil, fmt.Errorf("保存默认配置失败: %v", err)
	}

	return cfg, nil
}
