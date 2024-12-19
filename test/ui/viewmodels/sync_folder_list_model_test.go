package viewmodels_test

import (
	"os"
	"path/filepath"
	"testing"

	"synctools/internal/model"
)

func TestSyncFolderListModel_ValidityColumn(t *testing.T) {
	viewModel, _, _, tempDir := setupTest(t)
	defer cleanupTest(tempDir)

	// 设置 UI 控件
	nameEdit := NewMockLineEdit()
	versionEdit := NewMockLineEdit()
	hostEdit := NewMockLineEdit()
	portEdit := NewMockLineEdit()
	syncDirEdit := NewMockLineEdit()
	configTable := NewMockTableView()
	redirectTable := NewMockTableView()
	syncFolderTable := NewMockTableView()

	viewModel.SetupUI(
		configTable,
		redirectTable,
		nil, // StatusBar
		nameEdit,
		versionEdit,
		hostEdit,
		portEdit,
		syncDirEdit,
		nil, // ignoreEdit
		syncFolderTable,
	)

	// 创建一个有效的文件夹
	validFolderPath := filepath.Join(tempDir, "valid-folder")
	if err := os.MkdirAll(validFolderPath, 0755); err != nil {
		t.Fatalf("创建有效文件夹失败: %v", err)
	}

	// 创建一个测试配置
	config := &model.Config{
		UUID:    "test-uuid",
		Name:    "test-config",
		Version: "1.0",
		Host:    "localhost",
		Port:    8080,
		SyncDir: tempDir,
		SyncFolders: []model.SyncFolder{
			{Path: "valid-folder", SyncMode: "mirror"},   // 有效路径
			{Path: "invalid-folder", SyncMode: "mirror"}, // 无效路径
		},
	}

	// 保存并加载配置
	if err := viewModel.Save(config); err != nil {
		t.Fatalf("保存配置失败: %v", err)
	}
	if err := viewModel.LoadConfig(config.UUID); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 获取同步文件夹列表模型
	model := viewModel.GetSyncFolderListModel()

	// 测试列数
	t.Run("列数", func(t *testing.T) {
		if model.ColumnCount() != 3 {
			t.Errorf("列数不正确，期望 3，实际 %d", model.ColumnCount())
		}
	})

	// 测试有效文件夹显示
	t.Run("有效文件夹显示", func(t *testing.T) {
		// 验证路径
		if path := model.Value(0, 0); path != "valid-folder" {
			t.Errorf("路径不正确，期望 valid-folder，实际 %v", path)
		}
		// 验证有效性标记
		if mark := model.Value(0, 2); mark != "√" {
			t.Errorf("有效性标记不正确，期望 √，实际 %v", mark)
		}
	})

	// 测试无效文件夹显示
	t.Run("无效文件夹显示", func(t *testing.T) {
		// 验证路径
		if path := model.Value(1, 0); path != "invalid-folder" {
			t.Errorf("路径不正确，期望 invalid-folder，实际 %v", path)
		}
		// 验证有效性标记
		if mark := model.Value(1, 2); mark != "×" {
			t.Errorf("有效性标记不正确，期望 ×，实际 %v", mark)
		}
	})

	// 测试文件夹状态变化
	t.Run("文件夹状态变化", func(t *testing.T) {
		// 创建之前无效的文件夹
		invalidFolderPath := filepath.Join(tempDir, "invalid-folder")
		if err := os.MkdirAll(invalidFolderPath, 0755); err != nil {
			t.Fatalf("创建文件夹失败: %v", err)
		}

		// 验证有效性标记变为√
		if mark := model.Value(1, 2); mark != "√" {
			t.Errorf("文件夹创建后有效性标记不正确，期望 √，实际 %v", mark)
		}

		// 删除文件夹
		if err := os.RemoveAll(invalidFolderPath); err != nil {
			t.Fatalf("删除文件夹失败: %v", err)
		}

		// 验证有效性标记变为×
		if mark := model.Value(1, 2); mark != "×" {
			t.Errorf("文件夹删除后有效性标记不正确，期望 ×，实际 %v", mark)
		}
	})

	// 测试添加新文件夹
	t.Run("添加新文件夹", func(t *testing.T) {
		// 添加一个无效的文件夹
		if err := viewModel.AddSyncFolder("new-folder", "mirror"); err != nil {
			t.Fatalf("添加文件夹失败: %v", err)
		}

		// 验证有效性标记为×
		if mark := model.Value(2, 2); mark != "×" {
			t.Errorf("新添加的无效文件夹有效性标记不正确，期望 ×，实际 %v", mark)
		}

		// 创建该文件夹
		newFolderPath := filepath.Join(tempDir, "new-folder")
		if err := os.MkdirAll(newFolderPath, 0755); err != nil {
			t.Fatalf("创建文件夹失败: %v", err)
		}

		// 验证有效性标记变为√
		if mark := model.Value(2, 2); mark != "√" {
			t.Errorf("文件夹创建后有效性标记不正确，期望 √，实际 %v", mark)
		}
	})
}
