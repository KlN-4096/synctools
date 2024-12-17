package server

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"

	"synctools/pkg/common"
	"synctools/pkg/handlers"
)

type SyncServer struct {
	ConfigManager   *ConfigManager
	ConfigListModel *ConfigListModel
	ConfigTable     *walk.TableView
	ValidFolders    map[string]bool
	Running         bool
	Status          *walk.StatusBarItem
	Logger          common.Logger
	InvalidLabel    *walk.TextEdit
	RedirectTable   *walk.TableView
	RedirectModel   *RedirectTableModel
	NameEdit        *walk.LineEdit
	VersionEdit     *walk.LineEdit
	FolderTable     *walk.TableView
	FolderModel     *FolderTableModel
	Listener        net.Listener
	HostEdit        *walk.LineEdit
	PortEdit        *walk.LineEdit
	DirLabel        *walk.Label
	IgnoreListEdit  *walk.TextEdit
}

func NewSyncServer() *SyncServer {
	server := &SyncServer{
		ValidFolders: make(map[string]bool),
	}

	// 初始化配置管理器
	server.ConfigManager = NewConfigManager(server.Logger)
	server.ConfigManager.OnConfigChanged = server.updateUI

	// 初始化配置列表模型
	server.ConfigListModel = NewConfigListModel(server.ConfigManager)

	// 初始化表格模型
	server.FolderModel = &FolderTableModel{server: server}
	server.RedirectModel = &RedirectTableModel{server: server}

	// 加载所有配置文件
	if err := server.ConfigManager.LoadAllConfigs(); err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
	}

	return server
}

func (s *SyncServer) ValidateFolders() {
	s.ValidFolders = make(map[string]bool)
	var invalidFolders []string

	config := s.ConfigManager.GetCurrentConfig()

	if s.Logger != nil {
		s.Logger.DebugLog("开始验证文件夹列表...")
		s.Logger.DebugLog("当前根目录: %s", config.SyncDir)
		s.Logger.DebugLog("待验证文件夹数: %d", len(config.SyncFolders))
	}

	for _, folder := range config.SyncFolders {
		path := filepath.Join(config.SyncDir, folder.Path)
		valid := common.IsPathExists(path) && common.IsDir(path)
		s.ValidFolders[folder.Path] = valid

		if !valid {
			invalidFolders = append(invalidFolders, folder.Path)
		}

		if s.Logger != nil {
			if valid {
				s.Logger.DebugLog("有效的同步文件夹: %s (%s)", folder.Path, folder.SyncMode)
			} else {
				s.Logger.DebugLog(">>> 无效的同步文件夹: %s (%s) <<<", folder.Path, folder.SyncMode)
			}
		}
	}

	// 更新无效文件夹文本框
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

	config := s.ConfigManager.GetCurrentConfig()
	var err error
	s.Listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", config.Host, config.Port))
	if err != nil {
		return fmt.Errorf("启动服务器失败: %v", err)
	}

	handlers.SetServerInstance(s)
	s.Running = true
	s.Status.SetText("状态: 运行中")
	s.Logger.Log("服务器启动于 %s:%d", config.Host, config.Port)
	s.Logger.Log("同步目录: %s", config.SyncDir)

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

func (s *SyncServer) handleClient(conn net.Conn) {
	config := s.ConfigManager.GetCurrentConfig()
	handlers.HandleClient(conn, config.SyncDir, config.IgnoreList, s.Logger, func(path string) string {
		// 查找重定向配置
		for _, redirect := range config.FolderRedirects {
			if strings.HasPrefix(path, redirect.ClientPath) {
				// 将客户端路径替换为服务器路径
				return strings.Replace(path, redirect.ClientPath, redirect.ServerPath, 1)
			}
		}
		return path
	}, *config)
}

// GetFolderConfig 获取文件夹配置
func (s *SyncServer) GetFolderConfig(path string) (*common.SyncFolder, bool) {
	config := s.ConfigManager.GetCurrentConfig()
	for _, folder := range config.SyncFolders {
		if strings.HasPrefix(path, folder.Path) {
			folderCopy := folder // 创建副本以避免返回切片元素的指针
			return &folderCopy, true
		}
	}
	return nil, false
}

// updateUI 更新所有UI元素
func (s *SyncServer) updateUI() {
	config := s.ConfigManager.GetCurrentConfig()

	// 更新基本设置
	if s.HostEdit != nil {
		s.HostEdit.SetText(config.Host)
	}
	if s.PortEdit != nil {
		s.PortEdit.SetText(fmt.Sprintf("%d", config.Port))
	}
	if s.DirLabel != nil {
		s.DirLabel.SetText(config.SyncDir)
	}

	// 更新整合包信息
	if s.NameEdit != nil {
		s.NameEdit.SetText(config.Name)
	}
	if s.VersionEdit != nil {
		s.VersionEdit.SetText(config.Version)
	}

	// 更新忽略列表
	if s.IgnoreListEdit != nil {
		s.IgnoreListEdit.SetText(strings.Join(config.IgnoreList, "\r\n"))
	}

	// 更新表格模型
	if s.RedirectModel != nil {
		s.RedirectModel.PublishRowsReset()
	}
	if s.FolderModel != nil {
		s.FolderModel.PublishRowsReset()
	}

	// 验证文件夹
	s.ValidateFolders()
}
