/*
文件作用:
- 实现服务端同步服务的入口
- 管理服务器的启动和停止
- 维护服务器状态

主要功能:
1. 服务器生命周期管理
2. 网络服务管理
3. 配置管理
*/

package server

import (
	"fmt"

	"synctools/codes/internal/interfaces"
	"synctools/codes/pkg/errors"
	netserver "synctools/codes/pkg/network/server"
	"synctools/codes/pkg/service/base"
)

// ServerSyncService 服务端同步服务实现
type ServerSyncService struct {
	*base.BaseSyncService
	server   interfaces.NetworkServer
	syncBase *base.ServerSyncBase
}

// NewServerSyncService 创建服务端同步服务
func NewServerSyncService(config *interfaces.Config, logger interfaces.Logger, storage interfaces.Storage) *ServerSyncService {
	baseService := base.NewBaseSyncService(config, logger, storage)
	srv := &ServerSyncService{
		BaseSyncService: baseService,
	}
	srv.syncBase = base.NewServerSyncBase(baseService)
	return srv
}

// StartServer 启动服务器
func (s *ServerSyncService) StartServer() error {
	if s.IsRunning() {
		return errors.ErrServiceStart
	}

	// 验证配置
	if err := s.syncBase.ValidateConfig(); err != nil {
		s.SetStatus(fmt.Sprintf("启动失败: %s", err.Error()))
		return err
	}

	if s.server == nil {
		s.server = netserver.NewServer(s.GetCurrentConfig(), s, s.Logger)
	}

	if err := s.server.Start(); err != nil {
		s.SetStatus("启动失败")
		return err
	}

	if err := s.Start(); err != nil {
		s.StopServer()
		return err
	}

	s.SetStatus("服务器运行中")
	return nil
}

// StopServer 停止服务器
func (s *ServerSyncService) StopServer() error {
	if !s.IsRunning() {
		return nil
	}

	if s.server != nil {
		if err := s.server.Stop(); err != nil {
			return err
		}
		s.server = nil
	}

	s.Stop()
	s.SetStatus("服务器已停止")
	return nil
}

// SetServer 设置网络服务器
func (s *ServerSyncService) SetServer(server interfaces.NetworkServer) {
	if s.server != nil {
		s.StopServer()
	}
	s.server = server
}

// GetNetworkServer 获取网络服务器
func (s *ServerSyncService) GetNetworkServer() interfaces.NetworkServer {
	return s.server
}

// HandleSyncRequest 处理同步请求
func (s *ServerSyncService) HandleSyncRequest(request interface{}) error {
	req, ok := request.(*interfaces.SyncRequest)
	if !ok {
		return fmt.Errorf("无效的请求类型")
	}
	return s.syncBase.HandleSyncRequest(req)
}

// GetLocalFilesWithMD5 获取本地文件的MD5信息
func (s *ServerSyncService) GetLocalFilesWithMD5(dir string) (map[string]string, error) {
	return s.BaseSyncService.GetLocalFilesWithMD5(dir)
}
