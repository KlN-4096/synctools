package server

import (
	"crypto/rand"
	"encoding/hex"
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
	Config          common.SyncConfig
	ConfigFile      string
	ConfigList      []common.SyncConfig
	ConfigTable     *walk.TableView
	ConfigListModel *ConfigListModel
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
	SelectedUUID    string
	IgnoreListEdit  *walk.TextEdit
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

type RedirectTableModel struct {
	walk.TableModelBase
	server *SyncServer
}

func (m *RedirectTableModel) RowCount() int {
	return len(m.server.Config.FolderRedirects)
}

func (m *RedirectTableModel) Value(row, col int) interface{} {
	redirect := m.server.Config.FolderRedirects[row]
	switch col {
	case 0:
		return redirect.ServerPath
	case 1:
		return redirect.ClientPath
	}
	return nil
}

func (m *RedirectTableModel) PublishRowsReset() {
	m.TableModelBase.PublishRowsReset()
}

type ConfigListModel struct {
	walk.TableModelBase
	server *SyncServer
}

func (m *ConfigListModel) RowCount() int {
	return len(m.server.ConfigList)
}

func (m *ConfigListModel) Value(row, col int) interface{} {
	config := m.server.ConfigList[row]
	switch col {
	case 0:
		return config.UUID == m.server.SelectedUUID
	case 1:
		return config.Name
	case 2:
		return config.Version
	case 3:
		return config.UUID
	}
	return nil
}

func (m *ConfigListModel) SetValue(row, col int, value interface{}) error {
	if col == 0 {
		if checked, ok := value.(bool); ok {
			if checked {
				// 设置新的选中项
				newUUID := m.server.ConfigList[row].UUID
				if newUUID != m.server.SelectedUUID {
					m.server.SelectedUUID = newUUID
					// 立即刷新列表以更新所有复选框状态
					m.PublishRowsReset()
					// 加载新配置
					if err := m.server.LoadConfigByUUID(newUUID); err != nil {
						return err
					}
					if m.server.Logger != nil {
						m.server.Logger.DebugLog("已切换到配置: %s", m.server.Config.Name)
					}
				}
			} else {
				// 如果试图取消选中当前选中项，阻止这个操作
				if m.server.ConfigList[row].UUID == m.server.SelectedUUID {
					// 立即恢复选中状态
					m.PublishRowsReset()
					return nil
				}
			}
		}
	}
	return nil
}

// 实现 walk.TableModel 接口
func (m *ConfigListModel) Checked(row int) bool {
	return m.server.ConfigList[row].UUID == m.server.SelectedUUID
}

func (m *ConfigListModel) SetChecked(row int, checked bool) error {
	return m.SetValue(row, 0, checked)
}

func (m *ConfigListModel) CheckedCount() int {
	return 1 // 始终只有一个选中项
}

func (m *ConfigListModel) PublishRowsReset() {
	m.TableModelBase.PublishRowsReset()
}

func NewSyncServer() *SyncServer {
	// 设置配置文件路径
	configDir := filepath.Join(os.Getenv("APPDATA"), "SyncTools")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		fmt.Printf("创建配置目录失败: %v\n", err)
	}

	// 生成默认UUID
	uuid := make([]byte, 16)
	rand.Read(uuid)
	uuidStr := hex.EncodeToString(uuid)

	server := &SyncServer{
		Config: common.SyncConfig{
			UUID:    uuidStr,
			Name:    "默认整合包",
			Version: "1.0.0",
			Host:    "0.0.0.0",
			Port:    6666,
			SyncDir: "",
			IgnoreList: []string{
				".clientconfig",
				".DS_Store",
				"thumbs.db",
			},
			FolderRedirects: []common.FolderRedirect{
				{ServerPath: "clientmods", ClientPath: "mods"},
			},
		},
		ConfigFile:   filepath.Join(configDir, "server_config.json"),
		ValidFolders: make(map[string]bool),
		ConfigList:   make([]common.SyncConfig, 0),
		SelectedUUID: uuidStr, // 设置默认选中的UUID
	}

	// 初始化表格模型
	server.FolderModel = &FolderTableModel{server: server}
	server.RedirectModel = &RedirectTableModel{server: server}
	server.ConfigListModel = &ConfigListModel{
		TableModelBase: walk.TableModelBase{},
		server:         server,
	}

	// 加载所有配置文件
	if err := server.LoadAllConfigs(); err != nil {
		fmt.Printf("加载配置文件失败: %v\n", err)
	}

	return server
}

// LoadAllConfigs 加载所有配置文件
func (s *SyncServer) LoadAllConfigs() error {
	configDir := filepath.Dir(s.ConfigFile)
	files, err := os.ReadDir(configDir)
	if err != nil {
		return fmt.Errorf("读取配置目录失败: %v", err)
	}

	// 尝试加载选中的UUID
	selectedPath := filepath.Join(configDir, "selected_uuid.txt")
	if data, err := os.ReadFile(selectedPath); err == nil {
		s.SelectedUUID = strings.TrimSpace(string(data))
		if s.Logger != nil {
			s.Logger.DebugLog("已从文件加载选中的UUID: %s", s.SelectedUUID)
		}
	}

	s.ConfigList = make([]common.SyncConfig, 0)
	for _, file := range files {
		if !file.IsDir() && strings.HasPrefix(file.Name(), "config_") && strings.HasSuffix(file.Name(), ".json") {
			configPath := filepath.Join(configDir, file.Name())
			if config, err := common.LoadConfig(configPath); err == nil {
				s.ConfigList = append(s.ConfigList, *config)
			}
		}
	}

	// 如果没有配置文件，使用当前配置作为默认配置
	if len(s.ConfigList) == 0 {
		// 保存当前配置
		configPath := filepath.Join(configDir, fmt.Sprintf("config_%s.json", s.Config.UUID))
		if err := common.SaveConfig(&s.Config, configPath); err == nil {
			s.ConfigList = append(s.ConfigList, s.Config)
			s.SelectedUUID = s.Config.UUID
		}
	} else {
		// 如果没有选中的UUID或者选中的UUID不存在于配置列表中，使用第一个配置
		validUUID := false
		if s.SelectedUUID != "" {
			for _, config := range s.ConfigList {
				if config.UUID == s.SelectedUUID {
					validUUID = true
					s.Config = config
					break
				}
			}
		}

		if !validUUID {
			s.SelectedUUID = s.ConfigList[0].UUID
			s.Config = s.ConfigList[0]
			if s.Logger != nil {
				s.Logger.DebugLog("使用第一个配置作为默认配置: %s", s.Config.Name)
			}
		}
	}

	// 更新UI（如果已初始化）
	s.updateUI()

	return nil
}

// updateUI 更新所有UI元素
func (s *SyncServer) updateUI() {
	// 更新基本设置
	if s.HostEdit != nil {
		s.HostEdit.SetText(s.Config.Host)
	}
	if s.PortEdit != nil {
		s.PortEdit.SetText(fmt.Sprintf("%d", s.Config.Port))
	}
	if s.DirLabel != nil {
		s.DirLabel.SetText(s.Config.SyncDir)
	}

	// 更新整合包信息
	if s.NameEdit != nil {
		s.NameEdit.SetText(s.Config.Name)
	}
	if s.VersionEdit != nil {
		s.VersionEdit.SetText(s.Config.Version)
	}

	// 更新忽略列表
	if s.IgnoreListEdit != nil {
		s.IgnoreListEdit.SetText(strings.Join(s.Config.IgnoreList, "\r\n"))
	}

	// 更新表格模型
	if s.RedirectModel != nil {
		s.RedirectModel.PublishRowsReset() // 更新重定向配置表格
	}
	if s.FolderModel != nil {
		s.FolderModel.PublishRowsReset() // 更新同步文件夹表格
	}
	if s.ConfigListModel != nil {
		s.ConfigListModel.PublishRowsReset() // 更新配置列表表格
	}

	// 验证文件夹并更新无效文件夹列表
	s.ValidateFolders()

	// 更新配置表格选中项
	if s.ConfigTable != nil {
		// 找到当前UUID对应的索引
		for i, config := range s.ConfigList {
			if config.UUID == s.Config.UUID {
				s.ConfigTable.SetCurrentIndex(i)
				break
			}
		}
	}

	// 记录日志
	if s.Logger != nil {
		s.Logger.DebugLog("UI已更新:")
		s.Logger.DebugLog("- 主机: %s", s.Config.Host)
		s.Logger.DebugLog("- 端口: %d", s.Config.Port)
		s.Logger.DebugLog("- 同步目录: %s", s.Config.SyncDir)
		s.Logger.DebugLog("- 整合包名称: %s", s.Config.Name)
		s.Logger.DebugLog("- 版本: %s", s.Config.Version)
		s.Logger.DebugLog("- 忽略列表: %v", s.Config.IgnoreList)
		s.Logger.DebugLog("- 同步文件夹数量: %d", len(s.Config.SyncFolders))
		s.Logger.DebugLog("- 重定向配置数量: %d", len(s.Config.FolderRedirects))
	}
}

// LoadConfigByUUID 根据UUID加载配置
func (s *SyncServer) LoadConfigByUUID(uuid string) error {
	// 先从内存中查找
	for _, config := range s.ConfigList {
		if config.UUID == uuid {
			// 直接使用配置文件中的值，不使用默认值
			s.Config = common.SyncConfig{
				UUID:            config.UUID,
				Name:            config.Name,
				Host:            config.Host,
				Port:            config.Port,
				SyncDir:         config.SyncDir,
				Version:         config.Version,
				SyncFolders:     nil,
				IgnoreList:      nil,
				FolderRedirects: nil,
			}

			// 只有在源配置中存在时才复制这些字段
			if config.SyncFolders != nil {
				s.Config.SyncFolders = make([]common.SyncFolder, len(config.SyncFolders))
				copy(s.Config.SyncFolders, config.SyncFolders)
			}

			if config.IgnoreList != nil {
				s.Config.IgnoreList = make([]string, len(config.IgnoreList))
				copy(s.Config.IgnoreList, config.IgnoreList)
			}

			if config.FolderRedirects != nil {
				s.Config.FolderRedirects = make([]common.FolderRedirect, len(config.FolderRedirects))
				copy(s.Config.FolderRedirects, config.FolderRedirects)
			}

			if s.Logger != nil {
				s.Logger.DebugLog("已加载配置:")
				s.Logger.DebugLog("- UUID: %s", s.Config.UUID)
				s.Logger.DebugLog("- 名称: %s", s.Config.Name)
				s.Logger.DebugLog("- 版本: %s", s.Config.Version)
				s.Logger.DebugLog("- 主机: %s", s.Config.Host)
				s.Logger.DebugLog("- 端口: %d", s.Config.Port)
				s.Logger.DebugLog("- 同步目录: %s", s.Config.SyncDir)
				s.Logger.DebugLog("- 忽略列表: %v", s.Config.IgnoreList)
				s.Logger.DebugLog("- 同步文件夹: %v", s.Config.SyncFolders)
				s.Logger.DebugLog("- 重定向配置: %v", s.Config.FolderRedirects)
			}

			s.updateUI()
			s.ValidateFolders()
			return nil
		}
	}

	// 如果内存中没有，尝试从文件加载
	configPath := filepath.Join(filepath.Dir(s.ConfigFile), fmt.Sprintf("config_%s.json", uuid))
	config, err := common.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 直接使用配置文件中的值，不使用默认值
	s.Config = common.SyncConfig{
		UUID:            config.UUID,
		Name:            config.Name,
		Host:            config.Host,
		Port:            config.Port,
		SyncDir:         config.SyncDir,
		Version:         config.Version,
		SyncFolders:     nil,
		IgnoreList:      nil,
		FolderRedirects: nil,
	}

	// 只有在源配置中存在时才复制这些字段
	if config.SyncFolders != nil {
		s.Config.SyncFolders = make([]common.SyncFolder, len(config.SyncFolders))
		copy(s.Config.SyncFolders, config.SyncFolders)
	}

	if config.IgnoreList != nil {
		s.Config.IgnoreList = make([]string, len(config.IgnoreList))
		copy(s.Config.IgnoreList, config.IgnoreList)
	}

	if config.FolderRedirects != nil {
		s.Config.FolderRedirects = make([]common.FolderRedirect, len(config.FolderRedirects))
		copy(s.Config.FolderRedirects, config.FolderRedirects)
	}

	if s.Logger != nil {
		s.Logger.DebugLog("已从文件加载配置:")
		s.Logger.DebugLog("- UUID: %s", s.Config.UUID)
		s.Logger.DebugLog("- 名称: %s", s.Config.Name)
		s.Logger.DebugLog("- 版本: %s", s.Config.Version)
		s.Logger.DebugLog("- 主机: %s", s.Config.Host)
		s.Logger.DebugLog("- 端口: %d", s.Config.Port)
		s.Logger.DebugLog("- 同步目录: %s", s.Config.SyncDir)
		s.Logger.DebugLog("- 忽略列表: %v", s.Config.IgnoreList)
		s.Logger.DebugLog("- 同步文件夹: %v", s.Config.SyncFolders)
		s.Logger.DebugLog("- 重定向配置: %v", s.Config.FolderRedirects)
	}

	s.updateUI()
	s.ValidateFolders()
	return nil
}

// DeleteConfig 删除配置
func (s *SyncServer) DeleteConfig(configPath string, index int) error {
	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("删除配置文件失败: %v", err)
	}

	// 从列表中移除
	s.ConfigList = append(s.ConfigList[:index], s.ConfigList[index+1:]...)
	s.ConfigListModel.PublishRowsReset()

	// 如果删除的是当前配置，加载第一个配置
	if len(s.ConfigList) > 0 {
		s.LoadConfigByUUID(s.ConfigList[0].UUID)
		s.ConfigTable.SetCurrentIndex(0)
	}

	return nil
}

// SaveConfig 保存配置到文件
func (s *SyncServer) SaveConfig() error {
	// 校验UUID
	if s.Config.UUID == "" {
		// 生成新的UUID
		uuid := make([]byte, 16)
		if _, err := rand.Read(uuid); err != nil {
			return fmt.Errorf("生成UUID失败: %v", err)
		}
		s.Config.UUID = hex.EncodeToString(uuid)
		if s.Logger != nil {
			s.Logger.DebugLog("生成新的UUID: %s", s.Config.UUID)
		}
	}

	// 检查UUID是否与当前选中的UUID匹配
	if s.SelectedUUID != "" && s.Config.UUID != s.SelectedUUID {
		if s.Logger != nil {
			s.Logger.DebugLog("UUID不匹配: 当前=%s, 选中=%s", s.Config.UUID, s.SelectedUUID)
		}
		return fmt.Errorf("配置UUID不匹配，无法保存")
	}

	if s.Logger != nil {
		s.Logger.DebugLog("正在保存配置...")
		s.Logger.DebugLog("UUID: %s", s.Config.UUID)
		s.Logger.DebugLog("整合包名称: %s", s.Config.Name)
		s.Logger.DebugLog("版本: %s", s.Config.Version)
		s.Logger.DebugLog("主机: %s", s.Config.Host)
		s.Logger.DebugLog("端口: %d", s.Config.Port)
		s.Logger.DebugLog("同步目录: %s", s.Config.SyncDir)
		s.Logger.DebugLog("忽略列表: %v", s.Config.IgnoreList)
		s.Logger.DebugLog("重定向配置:")
		for i, redirect := range s.Config.FolderRedirects {
			s.Logger.DebugLog("  %d. %s -> %s", i+1, redirect.ServerPath, redirect.ClientPath)
		}
		s.Logger.DebugLog("同步文件夹:")
		for i, folder := range s.Config.SyncFolders {
			s.Logger.DebugLog("  %d. %s (%s)", i+1, folder.Path, folder.SyncMode)
		}
	}

	configPath := filepath.Join(filepath.Dir(s.ConfigFile), fmt.Sprintf("config_%s.json", s.Config.UUID))
	if err := common.SaveConfig(&s.Config, configPath); err != nil {
		if s.Logger != nil {
			s.Logger.Log("保存配置失败: %v", err)
		}
		return err
	}

	// 更新配置列表中的对应项
	found := false
	for i, config := range s.ConfigList {
		if config.UUID == s.Config.UUID {
			s.ConfigList[i] = s.Config
			found = true
			break
		}
	}

	// 如果在列表中没找到，说明是新配置，添加到列表中
	if !found {
		s.ConfigList = append(s.ConfigList, s.Config)
	}

	// 保存选中的UUID
	selectedPath := filepath.Join(filepath.Dir(s.ConfigFile), "selected_uuid.txt")
	if err := os.WriteFile(selectedPath, []byte(s.SelectedUUID), 0644); err != nil {
		if s.Logger != nil {
			s.Logger.Log("保存选中UUID失败: %v", err)
		}
	}

	// 更新UI
	if s.ConfigListModel != nil {
		s.ConfigListModel.PublishRowsReset()
	}

	if s.Logger != nil {
		s.Logger.DebugLog("配置已保存到: %s", configPath)
	}
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
	}, s.Config)
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

// UpdateAllUI 更新所有UI元素
func (s *SyncServer) UpdateAllUI() {
	s.updateUI()
}
