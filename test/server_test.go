package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"synctools/pkg/common"
	"synctools/pkg/server"
)

func TestConfigLoading(t *testing.T) {
	// 创建测试配置目录
	testDir := filepath.Join(os.TempDir(), "synctools_test")
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	// 创建测试配置文件1
	config1 := common.SyncConfig{
		UUID:    "test123",
		Name:    "Test Config 1",
		Version: "1.0.0",
		Host:    "0.0.0.0",
		Port:    6666,
		SyncDir: "",
		IgnoreList: []string{
			".git",
			"*.tmp",
			"thumbs.db",
		},
	}

	// 创建测试配置文件2
	config2 := common.SyncConfig{
		UUID:    "test456",
		Name:    "Test Config 2",
		Version: "2.0.0",
		Host:    "0.0.0.0",
		Port:    6666,
		SyncDir: "C:\\test",
		IgnoreList: []string{
			".svn",
			"*.bak",
			".DS_Store",
		},
	}

	// 保存配置文件
	configPath1 := filepath.Join(testDir, "config_"+config1.UUID+".json")
	configPath2 := filepath.Join(testDir, "config_"+config2.UUID+".json")

	data1, _ := json.MarshalIndent(config1, "", "    ")
	data2, _ := json.MarshalIndent(config2, "", "    ")

	os.WriteFile(configPath1, data1, 0644)
	os.WriteFile(configPath2, data2, 0644)

	// 创建服务器实例
	s := &server.SyncServer{
		ConfigFile:   filepath.Join(testDir, "server_config.json"),
		Config:       config1,
		SelectedUUID: "test456", // 设置初始选中的UUID
	}

	// 测试加载配置
	if err := s.LoadAllConfigs(); err != nil {
		t.Errorf("加载配置失败: %v", err)
	}

	t.Logf("加载配置后的状态:")
	t.Logf("- 配置列表数量: %d", len(s.ConfigList))
	t.Logf("- 选中的UUID: %s", s.SelectedUUID)
	t.Logf("- 当前配置UUID: %s", s.Config.UUID)
	t.Logf("- 当前忽略列表: %v", s.Config.IgnoreList)

	// 验证配置列表
	if len(s.ConfigList) != 2 {
		t.Errorf("期望配置数量为2，实际为%d", len(s.ConfigList))
	}

	// 测试切换配置
	if err := s.LoadConfigByUUID("test456"); err != nil {
		t.Errorf("切换配置失败: %v", err)
	}

	t.Logf("切换配置后的状态:")
	t.Logf("- 选中的UUID: %s", s.SelectedUUID)
	t.Logf("- 当前配置UUID: %s", s.Config.UUID)
	t.Logf("- 当前忽略列表: %v", s.Config.IgnoreList)

	// 验证当前配置
	if s.Config.UUID != "test456" {
		t.Errorf("期望UUID为test456，实际为%s", s.Config.UUID)
	}

	if s.Config.Name != "Test Config 2" {
		t.Errorf("期望名称为Test Config 2，实际为%s", s.Config.Name)
	}

	// 验证忽略列表
	expectedIgnoreList := []string{".svn", "*.bak", ".DS_Store"}
	if !reflect.DeepEqual(s.Config.IgnoreList, expectedIgnoreList) {
		t.Errorf("忽略列表不匹配:\n期望: %v\n实际: %v", expectedIgnoreList, s.Config.IgnoreList)
	}

	// 测试修改忽略列表
	newIgnoreList := []string{".svn", "*.bak", ".DS_Store", "*.log"}
	s.Config.IgnoreList = newIgnoreList

	// 保存配置
	if err := s.SaveConfig(); err != nil {
		t.Errorf("保存配置失败: %v", err)
		t.Logf("保存配置时的状态:")
		t.Logf("- 选中的UUID: %s", s.SelectedUUID)
		t.Logf("- 当前配置UUID: %s", s.Config.UUID)
		t.Logf("- 当前忽略列表: %v", s.Config.IgnoreList)
	}

	// 重新加载并验证
	if err := s.LoadConfigByUUID("test456"); err != nil {
		t.Errorf("重新加载配置失败: %v", err)
	}

	t.Logf("重新加载后的状态:")
	t.Logf("- 选中的UUID: %s", s.SelectedUUID)
	t.Logf("- 当前配置UUID: %s", s.Config.UUID)
	t.Logf("- 当前忽略列表: %v", s.Config.IgnoreList)

	// 验证忽略列表是否正确保存和加载
	if !reflect.DeepEqual(s.Config.IgnoreList, newIgnoreList) {
		t.Errorf("保存后的忽略列表不匹配:\n期望: %v\n实际: %v", newIgnoreList, s.Config.IgnoreList)
	}

	// 测试空忽略列表
	s.Config.IgnoreList = nil
	if err := s.SaveConfig(); err != nil {
		t.Errorf("保存空忽略列表失败: %v", err)
	}

	// 重新加载并验证空忽略列表是否被正确初始化为空切片
	if err := s.LoadConfigByUUID("test456"); err != nil {
		t.Errorf("重新加载配置失败: %v", err)
	}

	if s.Config.IgnoreList == nil {
		t.Error("忽略列表不应该为nil")
	}

	if len(s.Config.IgnoreList) != 0 {
		t.Errorf("期望空忽略列表，实际为: %v", s.Config.IgnoreList)
	}
}
