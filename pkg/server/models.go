package server

import (
	"github.com/lxn/walk"
)

// ConfigListModel represents the model for configuration list
type ConfigListModel struct {
	walk.TableModelBase
	manager *ConfigManager
}

// NewConfigListModel creates a new configuration list model
func NewConfigListModel(manager *ConfigManager) *ConfigListModel {
	model := &ConfigListModel{
		manager: manager,
	}
	manager.SetConfigListModel(model)
	return model
}

func (m *ConfigListModel) RowCount() int {
	return len(m.manager.ConfigList)
}

func (m *ConfigListModel) Value(row, col int) interface{} {
	config := m.manager.ConfigList[row]
	switch col {
	case 0:
		return config.UUID == m.manager.SelectedUUID
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
				newUUID := m.manager.ConfigList[row].UUID
				if newUUID != m.manager.SelectedUUID {
					m.manager.SelectedUUID = newUUID
					// 立即刷新列表以更新所有复选框状态
					m.PublishRowsReset()
					// 加载新配置
					if err := m.manager.LoadConfigByUUID(newUUID); err != nil {
						return err
					}
				}
			} else {
				// 如果试图取消选中当前选中项，阻止这个操作
				if m.manager.ConfigList[row].UUID == m.manager.SelectedUUID {
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
	return m.manager.ConfigList[row].UUID == m.manager.SelectedUUID
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

// FolderTableModel represents the model for folder list
type FolderTableModel struct {
	walk.TableModelBase
	server *SyncServer
}

func (m *FolderTableModel) RowCount() int {
	return len(m.server.ConfigManager.GetCurrentConfig().SyncFolders)
}

func (m *FolderTableModel) Value(row, col int) interface{} {
	folder := m.server.ConfigManager.GetCurrentConfig().SyncFolders[row]
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

// RedirectTableModel represents the model for redirect list
type RedirectTableModel struct {
	walk.TableModelBase
	server *SyncServer
}

func (m *RedirectTableModel) RowCount() int {
	return len(m.server.ConfigManager.GetCurrentConfig().FolderRedirects)
}

func (m *RedirectTableModel) Value(row, col int) interface{} {
	redirect := m.server.ConfigManager.GetCurrentConfig().FolderRedirects[row]
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
