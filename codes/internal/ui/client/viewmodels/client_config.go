package viewmodels

import (
	"strconv"

	"synctools/codes/internal/interfaces"
)

// SaveConfig 保存配置
func (vm *MainViewModel) SaveConfig() error {
	config := vm.GetCurrentConfig()
	if config == nil {
		vm.logger.Error("保存配置失败：配置为空", interfaces.Fields{})
		return nil
	}

	// 更新配置
	config.Host = vm.addressEdit.Text()
	config.Port = vm.parsePort()
	config.SyncDir = vm.syncPathEdit.Text()

	// 保存配置
	if err := vm.syncService.SaveConfig(config); err != nil {
		vm.logger.Error("保存配置失败", interfaces.Fields{
			"error": err,
		})
		return err
	}

	vm.logger.Info("配置保存成功", interfaces.Fields{
		"host":     config.Host,
		"port":     config.Port,
		"syncPath": config.SyncDir,
	})
	return nil
}

// GetCurrentConfig 获取当前配置
func (vm *MainViewModel) GetCurrentConfig() *interfaces.Config {
	if vm.syncService == nil {
		vm.logger.Error("获取配置失败：同步服务为空", interfaces.Fields{})
		return nil
	}
	return vm.syncService.GetCurrentConfig()
}

// parsePort 解析端口号
func (vm *MainViewModel) parsePort() int {
	portStr := vm.portEdit.Text()
	port, err := strconv.Atoi(portStr)
	if err != nil {
		vm.logger.Error("端口号解析失败", interfaces.Fields{
			"port":  portStr,
			"error": err,
		})
		return 0
	}
	return port
}
