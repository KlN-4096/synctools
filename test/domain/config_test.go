package domain

import (
	"testing"

	"synctools/internal/model"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *model.Config
		wantErr bool
	}{
		{
			name: "有效配置",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-config",
				Version: "1.0.0",
				Host:    "127.0.0.1",
				Port:    8080,
				SyncDir: "/test/dir",
			},
			wantErr: false,
		},
		{
			name: "名称为空",
			config: &model.Config{
				UUID:    "test-uuid",
				Version: "1.0.0",
				Host:    "127.0.0.1",
				Port:    8080,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "版本为空",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-config",
				Host:    "127.0.0.1",
				Port:    8080,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "主机地址为空",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-config",
				Version: "1.0.0",
				Port:    8080,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "端口无效",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-config",
				Version: "1.0.0",
				Host:    "127.0.0.1",
				Port:    0,
				SyncDir: "/test/dir",
			},
			wantErr: true,
		},
		{
			name: "同步目录为空",
			config: &model.Config{
				UUID:    "test-uuid",
				Name:    "test-config",
				Version: "1.0.0",
				Host:    "127.0.0.1",
				Port:    8080,
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

func TestConfig_Clone(t *testing.T) {
	original := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0.0",
		Host:    "127.0.0.1",
		Port:    8080,
		SyncDir: "/test/dir",
		SyncFolders: []model.SyncFolder{
			{Path: "folder1", SyncMode: "mirror"},
			{Path: "folder2", SyncMode: "push"},
		},
		IgnoreList: []string{".git", "node_modules"},
		FolderRedirects: []model.FolderRedirect{
			{ServerPath: "server1", ClientPath: "client1"},
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

	// 验证切片长度
	if len(clone.SyncFolders) != len(original.SyncFolders) {
		t.Errorf("Clone() SyncFolders length = %v, want %v", len(clone.SyncFolders), len(original.SyncFolders))
	}
	if len(clone.IgnoreList) != len(original.IgnoreList) {
		t.Errorf("Clone() IgnoreList length = %v, want %v", len(clone.IgnoreList), len(original.IgnoreList))
	}
	if len(clone.FolderRedirects) != len(original.FolderRedirects) {
		t.Errorf("Clone() FolderRedirects length = %v, want %v", len(clone.FolderRedirects), len(original.FolderRedirects))
	}

	// 验证切片内容
	for i, folder := range original.SyncFolders {
		if folder.Path != clone.SyncFolders[i].Path || folder.SyncMode != clone.SyncFolders[i].SyncMode {
			t.Errorf("Clone() SyncFolder[%d] = %v, want %v", i, clone.SyncFolders[i], folder)
		}
	}
	for i, ignore := range original.IgnoreList {
		if ignore != clone.IgnoreList[i] {
			t.Errorf("Clone() IgnoreList[%d] = %v, want %v", i, clone.IgnoreList[i], ignore)
		}
	}
	for i, redirect := range original.FolderRedirects {
		if redirect.ServerPath != clone.FolderRedirects[i].ServerPath || redirect.ClientPath != clone.FolderRedirects[i].ClientPath {
			t.Errorf("Clone() FolderRedirect[%d] = %v, want %v", i, clone.FolderRedirects[i], redirect)
		}
	}
}
