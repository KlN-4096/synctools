package server

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"

	"synctools/pkg/common"
	"synctools/pkg/handlers"
)

type SyncServer struct {
	Config            common.SyncConfig
	SyncFolders       []string
	ValidFolders      map[string]bool
	Logger            *common.GUILogger
	FolderEdit        *walk.TextEdit
	Status            *walk.StatusBarItem
	Running           bool
	Listener          net.Listener
	InvalidLabel      *walk.TextEdit
	ServerPath        *walk.LineEdit
	ClientPath        *walk.LineEdit
	ConfigFile        string          // 配置文件路径
	RedirectComposite *walk.Composite // 用于存放重定向配置的容器
	VersionEdit       *walk.LineEdit  // 版本编辑控件
}

func NewSyncServer() *SyncServer {
	// 设置配置文件路径
	configDir := filepath.Join(os.Getenv("APPDATA"), "SyncTools")
	configFile := filepath.Join(configDir, "server_config.json")

	// 加载配置
	config, err := common.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		config = &common.SyncConfig{
			Host:       "0.0.0.0",
			Port:       6666,
			SyncDir:    "",
			IgnoreList: []string{".clientconfig", ".DS_Store", "thumbs.db"},
			FolderRedirects: []common.FolderRedirect{
				{ServerPath: "clientmods", ClientPath: "mods"},
			},
		}
	}

	return &SyncServer{
		Config:       *config,
		ConfigFile:   configFile,
		SyncFolders:  []string{},
		ValidFolders: make(map[string]bool),
		Running:      false,
	}
}

// SaveConfig 保存配置到文件
func (s *SyncServer) SaveConfig() error {
	// 更新配置
	s.Config.SyncFolders = s.SyncFolders

	// 检查配置是否有变化
	currentConfig, err := common.LoadConfig(s.ConfigFile)
	if err == nil && s.Config.Equal(currentConfig) {
		s.Logger.DebugLog("配置未发生变化，跳过保存")
		return nil
	}

	if err := common.SaveConfig(&s.Config, s.ConfigFile); err != nil {
		s.Logger.Log("保存配置失败: %v", err)
		return err
	}
	s.Logger.Log("配置已保存到: %s", s.ConfigFile)
	return nil
}

func (s *SyncServer) ValidateFolders() {
	s.ValidFolders = make(map[string]bool)
	var invalidFolders []string

	s.Logger.DebugLog("开始验证文件夹列表...")
	s.Logger.DebugLog("当前根目录: %s", s.Config.SyncDir)
	s.Logger.DebugLog("待验证文件夹数: %d", len(s.SyncFolders))

	for _, folder := range s.SyncFolders {
		path := filepath.Join(s.Config.SyncDir, folder)
		valid := common.IsPathExists(path) && common.IsDir(path)
		s.ValidFolders[folder] = valid
		if valid {
			s.Logger.DebugLog("有效的同步文件夹: %s", folder)
		} else {
			s.Logger.DebugLog(">>> 无效的同步文件夹: %s <<<", folder)
			invalidFolders = append(invalidFolders, folder)
		}
	}

	// 更新无效文件夹文本框
	if len(invalidFolders) > 0 {
		s.InvalidLabel.SetText(strings.Join(invalidFolders, "\r\n"))
		s.Logger.DebugLog("----------------------------------------")
		s.Logger.DebugLog("发现 %d 个无效文件夹:", len(invalidFolders))
		for i, folder := range invalidFolders {
			s.Logger.DebugLog("%d. %s", i+1, folder)
		}
		s.Logger.DebugLog("----------------------------------------")
	} else {
		s.InvalidLabel.SetText("")
		s.Logger.DebugLog("所有文件夹都有效")
	}
}

func (s *SyncServer) StartServer() error {
	if s.Running {
		return common.ErrServerRunning
	}

	var err error
	s.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port))
	if err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	s.Running = true
	s.Status.SetText("状态: 运行中")
	s.Logger.Log("服务器启动于 %s:%d", s.Config.Host, s.Config.Port)
	s.Logger.Log("同步目录: %s", s.Config.SyncDir)

	go func() {
		for s.Running {
			conn, err := s.Listener.Accept()
			if err != nil {
				if s.Running {
					s.Logger.Log("接受连接错误: %v", err)
				}
				continue
			}

			go s.handleClient(conn)
		}
	}()

	return nil
}

func (s *SyncServer) StopServer() {
	if s.Running {
		s.Running = false
		if s.Listener != nil {
			s.Listener.Close()
		}
		s.Status.SetText("状态: 已停止")
		s.Logger.Log("服务器已停止")
	}
}

func (s *SyncServer) UpdateRedirectConfig() {
	// 清空所有重定向配置
	s.Config.FolderRedirects = nil

	// 遍历所有重定向配置组件
	for i := 0; i < s.RedirectComposite.Children().Len(); i++ {
		composite := s.RedirectComposite.Children().At(i).(*walk.Composite)
		if composite.Children().Len() < 5 {
			continue
		}

		// 获取输入框（第2个和第4个子组件）
		serverEdit := composite.Children().At(1).(*walk.LineEdit)
		clientEdit := composite.Children().At(3).(*walk.LineEdit)

		s.Config.FolderRedirects = append(s.Config.FolderRedirects, common.FolderRedirect{
			ServerPath: serverEdit.Text(),
			ClientPath: clientEdit.Text(),
		})
	}

	if s.Logger != nil {
		s.Logger.DebugLog("重定向配置已更新: %v", s.Config.FolderRedirects)
	}
}

func (s *SyncServer) handleClient(conn net.Conn) {
	handlers.HandleClient(conn, s.Config.SyncDir, s.Config.IgnoreList, s.Logger, func(path string) string {
		// 查找重定向配置
		for _, redirect := range s.Config.FolderRedirects {
			if strings.HasPrefix(path, redirect.ClientPath) {
				// 将客户端路径替换为服务器路径
				return strings.Replace(path, redirect.ClientPath, redirect.ServerPath, 1)
			}
		}
		return path
	}, s.Config.Version)
}
