package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lxn/walk"

	"synctools/cmd/server/ui"
	"synctools/pkg/server"
)

func main() {
	fmt.Println("程序启动...")

	defer func() {
		if r := recover(); r != nil {
			fmt.Println("捕获到错误:", r)
			// 确保日志目录存在
			if err := os.MkdirAll("logs", 0755); err == nil {
				// 创建应急日志文件
				logFile, err := os.OpenFile(
					filepath.Join("logs", fmt.Sprintf("server_crash_%s.log",
						time.Now().Format("2006-01-02_15-04-05"))),
					os.O_CREATE|os.O_WRONLY|os.O_APPEND,
					0644,
				)
				if err == nil {
					fmt.Fprintf(logFile, "[%s] 程序崩溃: %v\n",
						time.Now().Format("2006-01-02 15:04:05"), r)
					logFile.Close()
				}
			}

			// 显示错误对话框
			errorMsg := fmt.Sprintf("程序发生致命错误:\n%v", r)
			if _, err := os.Stat("logs"); err == nil {
				errorMsg += "\n\n详细信息请查看logs目录下的日志文件"
			}
			walk.MsgBox(nil, "错误", errorMsg, walk.MsgBoxIconError)

			// 给用户一些时间看错误信息
			time.Sleep(time.Second * 5)
		}
	}()

	fmt.Println("创建日志目录...")

	// 确保日志目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		walk.MsgBox(nil, "错误",
			fmt.Sprintf("创建日志目录失败: %v", err),
			walk.MsgBoxIconError)
		return
	}

	server := server.NewSyncServer()
	mainWindow, err := ui.CreateMainWindow(server)
	if err != nil {
		walk.MsgBox(nil, "错误", err.Error(), walk.MsgBoxIconError)
		return
	}

	mainWindow.Run()
}
