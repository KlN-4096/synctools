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
	ConfigFile        string
	ValidFolders      map[string]bool
	Running           bool
	Status            *walk.StatusBarItem
	Logger            common.Logger
	InvalidLabel      *walk.TextEdit
	RedirectComposite *walk.Composite
	VersionEdit       *walk.LineEdit
	FolderTable       *walk.TableView
	FolderModel       *FolderTableModel
	Listener          net.Listener
}

type FolderTableModel struct {
	walk.TableModelBase
	server *SyncServer
}

func (m *FolderTableModel) RowCount() int {
	return len(m.server.Config.SyncFolders)
}

func (m *FolderTableModel) Value(row, col int) interface{} {
	folder := m.server.Config.SyncFolders[row]
	switch col {
	case 0:
		return folder.Path
	case 1:
		return folder.SyncMode
	}
	return nil
}

func (m *FolderTableModel) PublishRowsReset() {
	m.TableModelBase.PublishRowsReset()
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

	server := &SyncServer{
		Config:       *config,
		ConfigFile:   configFile,
		ValidFolders: make(map[string]bool),
		Running:      false,
	}

	server.FolderModel = &FolderTableModel{server: server}

	// 初始验证文件夹
	server.ValidateFolders()

	return server
}

// SaveConfig 保存配置到文件
func (s *SyncServer) SaveConfig() error {
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

	// 如果Logger已初始化，则输出调试信息
	if s.Logger != nil {
		s.Logger.DebugLog("开始验证文件夹列表...")
		s.Logger.DebugLog("当前根目录: %s", s.Config.SyncDir)
		s.Logger.DebugLog("待验证文件夹数: %d", len(s.Config.SyncFolders))
	}

	for _, folder := range s.Config.SyncFolders {
		path := filepath.Join(s.Config.SyncDir, folder.Path)
		valid := common.IsPathExists(path) && common.IsDir(path)
		s.ValidFolders[folder.Path] = valid

		if s.Logger != nil {
			if valid {
				s.Logger.DebugLog("有效的同步文件夹: %s (%s)", folder.Path, folder.SyncMode)
			} else {
				s.Logger.DebugLog(">>> 无效的同步文件夹: %s (%s) <<<", folder.Path, folder.SyncMode)
				invalidFolders = append(invalidFolders, folder.Path)
			}
		} else if !valid {
			invalidFolders = append(invalidFolders, folder.Path)
		}
	}

	// 更新无效文件夹文本框（如果已初始化）
	if s.InvalidLabel != nil {
		if len(invalidFolders) > 0 {
			s.InvalidLabel.SetText(strings.Join(invalidFolders, "\r\n"))
			if s.Logger != nil {
				s.Logger.DebugLog("----------------------------------------")
				s.Logger.DebugLog("发现 %d 个无效文件夹:", len(invalidFolders))
				for i, folder := range invalidFolders {
					s.Logger.DebugLog("%d. %s", i+1, folder)
				}
				s.Logger.DebugLog("----------------------------------------")
			}
		} else {
			s.InvalidLabel.SetText("")
			if s.Logger != nil {
				s.Logger.DebugLog("所有文件夹都有效")
			}
		}
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

	handlers.SetServerInstance(s)
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
		if composite.Children().Len() < 5 { // 需要至少5个组件：2个标签、2个输入框和1个删除按钮
			continue
		}

		// 尝试获取输入框（索引1是服务器输入框，索引3是客户端输入框）
		serverEdit, ok1 := composite.Children().At(1).(*walk.LineEdit)
		clientEdit, ok2 := composite.Children().At(3).(*walk.LineEdit)

		// 确保类型转换成功
		if !ok1 || !ok2 {
			if s.Logger != nil {
				s.Logger.DebugLog("类型转换失败: ok1=%v, ok2=%v", ok1, ok2)
			}
			continue
		}

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

// GetFolderConfig 获取文件夹配置
func (s *SyncServer) GetFolderConfig(path string) (*common.SyncFolder, bool) {
	for _, folder := range s.Config.SyncFolders {
		if strings.HasPrefix(path, folder.Path) {
			folderCopy := folder // 创建副本以避免返回切片元素的指针
			return &folderCopy, true
		}
	}
	return nil, false
}
