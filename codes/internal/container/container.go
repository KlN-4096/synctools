package container

import (
	"fmt"
	"path/filepath"
	"sync"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/config"
	"synctools/codes/pkg/logger"
	"synctools/codes/pkg/service/client"
	"synctools/codes/pkg/service/server"
	"synctools/codes/pkg/storage"
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

	// 初始化存储
	store, err := storage.NewFileStorage(filepath.Join(baseDir, "configs"), l)
	if err != nil {
		return nil, fmt.Errorf("创建配置存储失败: %v", err)
	}
	c.Register("storage", store)

	// 初始化配置管理器
	cfgManager := config.NewManager(store, l)
	c.Register("config_manager", cfgManager)

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
	if cfg == nil {
		c.logger.Info("使用内存配置初始化服务器", interfaces.Fields{})
		// 创建内存配置
		cfg = &interfaces.Config{
			Type:        interfaces.ConfigTypeServer,
			Port:        25000,
			ConnTimeout: 300,
		}
	}

	c.logger.Info("初始化服务", interfaces.Fields{
		"type": cfg.Type,
	})

	var syncService interfaces.SyncService
	switch cfg.Type {
	case interfaces.ConfigTypeClient:
		// 创建客户端服务
		syncService = client.NewClientSyncService(cfg, c.GetLogger(), c.GetStorage())

	case interfaces.ConfigTypeServer:
		// 创建服务器服务
		syncService = server.NewServerSyncService(cfg, c.GetLogger(), c.GetStorage())

	default:
		return fmt.Errorf("未知的配置类型: %s", cfg.Type)
	}

	// 注册同步服务
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
	if svc := c.GetSyncService(); svc != nil {
		if serverService, ok := svc.(interfaces.ServerSyncService); ok {
			return serverService.GetNetworkServer()
		}
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
		if serverService, ok := svc.(interfaces.ServerSyncService); ok {
			// 如果是服务器类型，需要先停止网络服务器
			if err := serverService.StopServer(); err != nil {
				errs = append(errs, fmt.Errorf("停止网络服务器失败: %v", err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("关闭服务时发生错误: %v", errs)
	}
	return nil
}
