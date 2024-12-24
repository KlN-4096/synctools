/*
文件作用:
- 实现客户端的主程序入口
- 初始化客户端配置和组件
- 启动GUI界面和同步服务
- 理客户端状态和配置

主要方法:
- main: 程序入口,初始化各个组件并启动GUI界面
- init: 初始化基础配置和命令行参数
- loadOrCreateConfig: 加载或创建默认配置文件
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"synctools/internal/container"
	"synctools/internal/interfaces"
	"synctools/internal/ui/client/viewmodels"
	"synctools/internal/ui/client/windows"
)

var (
	baseDir    string
	configFile string
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
	logger.Info("客户端启动", interfaces.Fields{
		"base_dir": baseDir,
	})

	// 加载或创建默认配置
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
	mainViewModel := viewmodels.NewMainViewModel(
		c.GetSyncService(),
		c.GetLogger(),
	)

	// 初始化视图模型
	if err := mainViewModel.Initialize(nil); err != nil {
		logger.Fatal("初始化视图模型失败", interfaces.Fields{
			"error": err,
		})
	}

	// 创建并运行主窗口
	if err := windows.CreateMainWindow(mainViewModel); err != nil {
		logger.Fatal("创建主窗口失败", interfaces.Fields{
			"error": err,
		})
	}

	// 关闭视图模型
	if err := mainViewModel.Shutdown(); err != nil {
		logger.Error("关闭视图模型失败", interfaces.Fields{
			"error": err,
		})
	}

	// 关闭所有服务
	if err := c.Shutdown(); err != nil {
		logger.Error("关闭服务失败", interfaces.Fields{
			"error": err,
		})
	}
}

// loadOrCreateConfig 加载或创建默认配置
func loadOrCreateConfig(c *container.Container, configFile string) (*interfaces.Config, error) {
	cfgManager := c.GetConfigManager()
	storage := c.GetStorage()
	logger := c.GetLogger()

	// 如果指定了配置文件，尝试加载
	if configFile != "" {
		if err := cfgManager.LoadConfig(configFile); err != nil {
			return nil, fmt.Errorf("加载配置文件失败: %v", err)
		}
		return cfgManager.GetCurrentConfig().(*interfaces.Config), nil
	}

	// 检查是否存在默认配置
	if storage.Exists("client.json") {
		if err := cfgManager.LoadConfig("client"); err != nil {
			return nil, fmt.Errorf("加载默认配置失败: %v", err)
		}
		return cfgManager.GetCurrentConfig().(*interfaces.Config), nil
	}

	// 创建默认配置
	cfg := &interfaces.Config{
		UUID:    "default",
		Type:    interfaces.ConfigTypeClient,
		Name:    "SyncTools Client",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    8080,
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
