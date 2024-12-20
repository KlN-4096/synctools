package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"synctools/internal/config"
	"synctools/internal/model"
	"synctools/internal/network"
	"synctools/internal/service"
	"synctools/internal/ui"
	"synctools/internal/ui/viewmodels"
)

func main() {
	// 获取程序所在目录
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("获取程序路径失败: %v\n", err)
		return
	}
	workDir := filepath.Dir(exePath)
	os.Chdir(workDir)
	fmt.Printf("工作目录: %s\n", workDir)

	// 创建日志目录
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
		return
	}
	fmt.Printf("日志目录: %s\n", logDir)

	// 创建日志记录器
	logger := &model.DefaultLogger{
		DebugEnabled: true,
	}
	logger.Log("日志记录器初始化完成")

	// 设置panic处理
	defer func() {
		if r := recover(); r != nil {
			logPath := filepath.Join(logDir, "crash.log")
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				fmt.Fprintf(f, "[%s] 程序崩溃: %v\n堆栈信息:\n%s\n",
					time.Now().Format("2006-01-02 15:04:05"),
					r,
					string(debug.Stack()))
				f.Close()
			}
			logger.Error("程序崩溃: %v", r)
			debug.PrintStack()
		}
	}()

	// 创建配置目录
	configDir := "./configs"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		logger.Error("创建配置目录失败: %v", err)
		return
	}
	logger.Log("配置目录创建成功: %s", configDir)

	// 创建配置管理器
	logger.Log("正在创建配置管理器")
	configManager, err := config.NewManager(configDir, logger)
	if err != nil {
		logger.Error("创建配置管理器失败: %v", err)
		return
	}
	logger.Log("配置管理器创建成功")

	// 创建同步服务
	logger.Log("正在创建同步服务")
	syncService := service.NewSyncService(model.ConfigManager(configManager), logger)
	if syncService == nil {
		logger.Error("同步服务创建失败: %v", nil)
		return
	}

	// 创建网络服务器
	logger.Log("正在创建网络服务器")
	if config := configManager.GetCurrentConfig(); config != nil {
		server := network.NewServer(config, logger)
		syncService.SetServer(server)
	}
	logger.Log("同步服务创建成功")

	// 创建主视图模型
	logger.Log("正在创建主视图模型")
	mainViewModel := viewmodels.NewMainViewModel(syncService, logger)
	if mainViewModel == nil {
		logger.Error("主视图模型创建失败: %v", nil)
		return
	}
	logger.Log("主视图模型创建成功")

	// 创建并显示主窗口
	logger.Log("正在创建主窗口")
	if err := ui.CreateMainWindow(mainViewModel); err != nil {
		logger.Error("创建主窗口失败: %v", err)
		return
	}
}
