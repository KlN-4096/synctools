package model_test

import (
	"testing"

	"synctools/internal/model"
)

// TestConfigValidation 测试配置验证
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *model.Config
		wantErr bool
	}{
		{
			name: "完整有效的配置",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-pack",
				Version: "1.0.0",
				Host:    "localhost",
				Port:    8080,
				SyncDir: "/test/dir",
				SyncFolders: []model.SyncFolder{
					{Path: "mods", SyncMode: "mirror"},
				},
				IgnoreList: []string{"*.tmp"},
			},
			wantErr: false,
		},
		{
			name: "缺少必填字段-名称",
			config: &model.Config{
				UUID:    "test-uuid",
				Version: "1.0.0",
				Host:    "localhost",
				Port:    8080,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "缺少必填字段-版本",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-pack",
				Host:    "localhost",
				Port:    8080,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "无效端口-负数",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-pack",
				Version: "1.0.0",
				Host:    "localhost",
				Port:    -1,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "无效端口-超出范围",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-pack",
				Version: "1.0.0",
				Host:    "localhost",
				Port:    65536,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "无效主机地址-空",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-pack",
				Version: "1.0.0",
				Host:    "",
				Port:    8080,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "无效同步目录-空",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-pack",
				Version: "1.0.0",
				Host:    "localhost",
				Port:    8080,
				SyncDir: "",
			},
			wantErr: true,
		},
		{
			name: "无效同步模式",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-pack",
				Version: "1.0.0",
				Host:    "localhost",
				Port:    8080,
				SyncDir: "/test/dir",
				SyncFolders: []model.SyncFolder{
					{Path: "mods", SyncMode: "invalid"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfigClone 测试配置克隆
func TestConfigClone(t *testing.T) {
	original := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-pack",
		Version: "1.0.0",
		Host:    "localhost",
		Port:    8080,
		SyncDir: "/test/dir",
		SyncFolders: []model.SyncFolder{
			{Path: "mods", SyncMode: "mirror"},
		},
		IgnoreList: []string{"*.tmp"},
		FolderRedirects: []model.FolderRedirect{
			{ServerPath: "server", ClientPath: "client"},
		},
	}

	clone := original.Clone()

	// 验证基本字段
	if clone.UUID != original.UUID {
		t.Errorf("Clone() UUID = %v, want %v", clone.UUID, original.UUID)
	}
	if clone.Name != original.Name {
		t.Errorf("Clone() Name = %v, want %v", clone.Name, original.Name)
	}
	if clone.Version != original.Version {
		t.Errorf("Clone() Version = %v, want %v", clone.Version, original.Version)
	}
	if clone.Host != original.Host {
		t.Errorf("Clone() Host = %v, want %v", clone.Host, original.Host)
	}
	if clone.Port != original.Port {
		t.Errorf("Clone() Port = %v, want %v", clone.Port, original.Port)
	}
	if clone.SyncDir != original.SyncDir {
		t.Errorf("Clone() SyncDir = %v, want %v", clone.SyncDir, original.SyncDir)
	}

	// 验证切片是否深度复制
	if len(clone.SyncFolders) != len(original.SyncFolders) {
		t.Errorf("Clone() SyncFolders length = %v, want %v", len(clone.SyncFolders), len(original.SyncFolders))
	}
	if len(clone.IgnoreList) != len(original.IgnoreList) {
		t.Errorf("Clone() IgnoreList length = %v, want %v", len(clone.IgnoreList), len(original.IgnoreList))
	}
	if len(clone.FolderRedirects) != len(original.FolderRedirects) {
		t.Errorf("Clone() FolderRedirects length = %v, want %v", len(clone.FolderRedirects), len(original.FolderRedirects))
	}

	// 修改克隆不应影响原始对象
	clone.Name = "modified"
	if original.Name == "modified" {
		t.Error("Clone() did not create a deep copy")
	}
}
