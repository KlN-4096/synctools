/*
文件作用:
- 实现依赖注入容器
- 管理所有服务实例的生命周期
- 提供服务注册和获取功能
- 负责服务的初始化和关闭

主要方法:
- New: 创建新的依赖注入容器
- Register: 注册服务到容器
- Get: 从容器获取服务
- InitializeServices: 初始化所有服务
- GetLogger/GetConfigManager/GetNetworkServer/GetSyncService/GetStorage: 获取特定服务
- Shutdown: 关闭所有服务
*/

package container

import (
	"fmt"
	"path/filepath"
	"sync"

	"synctools/internal/interfaces"
	"synctools/pkg/config"
	"synctools/pkg/logger"
	"synctools/pkg/network"
	"synctools/pkg/service"
	"synctools/pkg/storage"
)

// Container 依赖注入容器
type Container struct {
	services map[string]interface{}
	logger   interfaces.Logger
	mu       sync.RWMutex
}

// New 创建新的依赖注入容器
func New(baseDir string) (*Container, error) {
	// 创建容器
	c := &Container{
		services: make(map[string]interface{}),
	}

	// 初始化日志
	logDir := filepath.Join(baseDir, "logs")
	l, err := logger.NewDefaultLogger(logDir)
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %v", err)
	}
	c.logger = l
	c.Register("logger", l)

	return c, nil
}

// Register 注册服务
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
}

// Get 获取服务
func (c *Container) Get(name string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.services[name]
}

// InitializeServices 初始化所有服务
func (c *Container) InitializeServices(baseDir string, cfg *interfaces.Config) error {
	// 初始化存储
	store, err := storage.NewFileStorage(filepath.Join(baseDir, "configs"))
	if err != nil {
		return fmt.Errorf("初始化存储失败: %v", err)
	}
	c.Register("storage", store)

	// 初始化配置管理器
	cfgManager := config.NewManager(store, c.logger)
	c.Register("config_manager", cfgManager)

	// 初始化网络服务器
	server := network.NewServer(cfg, c.logger)
	c.Register("network_server", server)

	// 初始化同步服务
	syncService := service.NewSyncService(cfg, c.logger, store)
	c.Register("sync_service", syncService)

	return nil
}

// GetLogger 获取日志服务
func (c *Container) GetLogger() interfaces.Logger {
	return c.logger
}

// GetConfigManager 获取配置管理器
func (c *Container) GetConfigManager() interfaces.ConfigManager {
	if svc := c.Get("config_manager"); svc != nil {
		return svc.(interfaces.ConfigManager)
	}
	return nil
}

// GetNetworkServer 获取网络服务器
func (c *Container) GetNetworkServer() interfaces.NetworkServer {
	if svc := c.Get("network_server"); svc != nil {
		return svc.(interfaces.NetworkServer)
	}
	return nil
}

// GetSyncService 获取同步服务
func (c *Container) GetSyncService() interfaces.SyncService {
	if svc := c.Get("sync_service"); svc != nil {
		return svc.(interfaces.SyncService)
	}
	return nil
}

// GetStorage 获取存储服务
func (c *Container) GetStorage() interfaces.Storage {
	if svc := c.Get("storage"); svc != nil {
		return svc.(interfaces.Storage)
	}
	return nil
}

// Shutdown 关闭所有服务
func (c *Container) Shutdown() error {
	var errs []error

	// 停止同步服务
	if svc := c.GetSyncService(); svc != nil {
		if err := svc.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("停止同步服务失败: %v", err))
		}
	}

	// 停止网络服务器
	if svc := c.GetNetworkServer(); svc != nil {
		if err := svc.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("停止网络服务器失败: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭服务时发生错误: %v", errs)
	}
	return nil
}
