package storage_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"synctools/internal/storage"
)

// TestData 测试数据结构
type TestData struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// setupTestDir 创建测试目录
func setupTestDir(t *testing.T) (string, func()) {
	// 创建临时测试目录
	testDir, err := os.MkdirTemp("", "storage_test_*")
	if err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 返回清理函数
	cleanup := func() {
		os.RemoveAll(testDir)
	}

	return testDir, cleanup
}

// TestFileStorage_Save 测试文件保存
func TestFileStorage_Save(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	tests := []struct {
		name    string
		key     string
		data    TestData
		wantErr bool
	}{
		{
			name: "保存新文件到空目录",
			key:  "test1.json",
			data: TestData{Name: "test1", Value: 1},
		},
		{
			name: "保存文件到已存在目录",
			key:  "test2.json",
			data: TestData{Name: "test2", Value: 2},
		},
		{
			name: "保存包含特殊字符的文件名",
			key:  "test#3.json",
			data: TestData{Name: "test3", Value: 3},
		},
	}

	s := storage.NewFileStorage(testDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 保存数据
			err := s.Save(tt.key, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// 验证文件是否存在
			filePath := filepath.Join(testDir, tt.key)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				t.Errorf("文件未创建: %s", filePath)
				return
			}

			// 读取文件内容并验证
			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Errorf("读取文件失败: %v", err)
				return
			}

			var savedData TestData
			if err := json.Unmarshal(content, &savedData); err != nil {
				t.Errorf("解析文件内容失败: %v", err)
				return
			}

			if savedData != tt.data {
				t.Errorf("保存的数据不匹配, got = %v, want = %v", savedData, tt.data)
			}
		})
	}
}

// TestFileStorage_Load 测试文件加载
func TestFileStorage_Load(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 准备测试数据
	testData := TestData{Name: "test", Value: 123}
	if err := s.Save("test.json", testData); err != nil {
		t.Fatalf("准备测试数据失败: %v", err)
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name: "加载存在的文件",
			key:  "test.json",
		},
		{
			name:    "加载不存在的文件",
			key:     "notexist.json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var loadedData TestData
			err := s.Load(tt.key, &loadedData)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if loadedData != testData {
				t.Errorf("加载的数据不匹配, got = %v, want = %v", loadedData, testData)
			}
		})
	}
}

// TestFileStorage_Delete 测试文件删除
func TestFileStorage_Delete(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 准备测试数据
	testData := TestData{Name: "test", Value: 123}
	if err := s.Save("test.json", testData); err != nil {
		t.Fatalf("准备测试数据失败: %v", err)
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name: "删除存在的文件",
			key:  "test.json",
		},
		{
			name:    "删除不存在的文件",
			key:     "notexist.json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Delete(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// 验证文件是否已删除
			filePath := filepath.Join(testDir, tt.key)
			if _, err := os.Stat(filePath); !os.IsNotExist(err) {
				t.Errorf("文件未被删除: %s", filePath)
			}
		})
	}
}

// TestFileStorage_List 测试文件列表
func TestFileStorage_List(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 测试空目录
	t.Run("列出空目录", func(t *testing.T) {
		files, err := s.List()
		if err != nil {
			t.Errorf("List() error = %v", err)
			return
		}
		if len(files) != 0 {
			t.Errorf("空目录应该返回空列表, got %v", files)
		}
	})

	// 准备测试数据
	testFiles := []struct {
		key  string
		data TestData
	}{
		{"test1.json", TestData{Name: "test1", Value: 1}},
		{"test2.json", TestData{Name: "test2", Value: 2}},
		{"test3.json", TestData{Name: "test3", Value: 3}},
	}

	for _, tf := range testFiles {
		if err := s.Save(tf.key, tf.data); err != nil {
			t.Fatalf("准备测试数据失败: %v", err)
		}
	}

	// 测试非空目录
	t.Run("列出包含多个文件的目录", func(t *testing.T) {
		files, err := s.List()
		if err != nil {
			t.Errorf("List() error = %v", err)
			return
		}

		if len(files) != len(testFiles) {
			t.Errorf("文件数量不匹配, got = %d, want = %d", len(files), len(testFiles))
			return
		}

		// 验证文件列表
		for _, tf := range testFiles {
			found := false
			for _, f := range files {
				if f == tf.key {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("未找到文件: %s", tf.key)
			}
		}
	})
}

// TestFileStorage_SaveWithPermissions 测试文件保存权限
func TestFileStorage_SaveWithPermissions(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	// 创建只读目录
	readOnlyDir := filepath.Join(testDir, "readonly")
	if err := os.MkdirAll(readOnlyDir, 0444); err != nil {
		t.Fatalf("创建只读目录失败: %v", err)
	}

	tests := []struct {
		name    string
		dir     string
		key     string
		data    TestData
		wantErr bool
	}{
		{
			name:    "保存到只读目录",
			dir:     readOnlyDir,
			key:     "test.json",
			data:    TestData{Name: "test", Value: 1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := storage.NewFileStorage(tt.dir)
			err := s.Save(tt.key, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestFileStorage_SaveLargeFile 测试大文件保存
func TestFileStorage_SaveLargeFile(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	// 生成大文件数据
	generateLargeData := func(size int) []byte {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}
		return data
	}

	tests := []struct {
		name     string
		key      string
		dataSize int
		wantErr  bool
	}{
		{
			name:     "保存1MB文件",
			key:      "1mb.dat",
			dataSize: 1 * 1024 * 1024,
		},
		{
			name:     "保存10MB文件",
			key:      "10mb.dat",
			dataSize: 10 * 1024 * 1024,
		},
	}

	s := storage.NewFileStorage(testDir)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试数据
			data := generateLargeData(tt.dataSize)

			// 记录开始时间
			start := time.Now()

			// 保存文件
			err := s.Save(tt.key, data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Save() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// 记录耗时
			duration := time.Since(start)
			t.Logf("保存 %s 耗时: %v", tt.key, duration)

			if tt.wantErr {
				return
			}

			// 验证文件大小
			filePath := filepath.Join(testDir, tt.key)
			info, err := os.Stat(filePath)
			if err != nil {
				t.Errorf("获取文件信息失败: %v", err)
				return
			}

			if info.Size() != int64(tt.dataSize) {
				t.Errorf("文件大小不匹配, got = %d, want = %d", info.Size(), tt.dataSize)
			}

			// 验证文件内容
			savedData, err := os.ReadFile(filePath)
			if err != nil {
				t.Errorf("读取文件失败: %v", err)
				return
			}

			if !bytes.Equal(savedData, data) {
				t.Error("文件内容不匹配")
			}
		})
	}
}

// TestFileStorage_LoadCorruptedFile 测试加载损坏的文件
func TestFileStorage_LoadCorruptedFile(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 创建损坏的JSON文件
	corruptedJSON := []byte(`{"name": "test", "value": 123`) // 缺少结束括号
	if err := os.WriteFile(filepath.Join(testDir, "corrupted.json"), corruptedJSON, 0644); err != nil {
		t.Fatalf("创建损坏的JSON文件失败: %v", err)
	}

	// 创建空文件
	emptyFile := filepath.Join(testDir, "empty.json")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("创建空文件失败: %v", err)
	}

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "加载损坏的JSON文件",
			key:     "corrupted.json",
			wantErr: true,
		},
		{
			name:    "加载空文件",
			key:     "empty.json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data TestData
			err := s.Load(tt.key, &data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestFileStorage_ListWithSubdirectories 测试包含子目录的列表
func TestFileStorage_ListWithSubdirectories(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 创建测试目录结构
	dirs := []string{
		"subdir1",
		"subdir2",
		filepath.Join("subdir1", "subsubdir"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(testDir, dir), 0755); err != nil {
			t.Fatalf("创建子目录失败: %v", err)
		}
	}

	// 在不同目录中创建文件
	files := []struct {
		path string
		data TestData
	}{
		{
			path: "root.json",
			data: TestData{Name: "root", Value: 1},
		},
		{
			path: filepath.ToSlash(filepath.Join("subdir1", "file1.json")),
			data: TestData{Name: "file1", Value: 2},
		},
		{
			path: filepath.ToSlash(filepath.Join("subdir2", "file2.json")),
			data: TestData{Name: "file2", Value: 3},
		},
		{
			path: filepath.ToSlash(filepath.Join("subdir1", "subsubdir", "file3.json")),
			data: TestData{Name: "file3", Value: 4},
		},
	}

	for _, f := range files {
		if err := s.Save(f.path, f.data); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	// 测试列表
	t.Run("列出包含子目录的目录", func(t *testing.T) {
		fileList, err := s.List()
		if err != nil {
			t.Errorf("List() error = %v", err)
			return
		}

		// 验证文件数量
		if len(fileList) != len(files) {
			t.Errorf("文件数量不匹配, got = %d, want = %d", len(fileList), len(files))
			return
		}

		// 验证是否包含所有文件
		for _, f := range files {
			found := false
			for _, listed := range fileList {
				if listed == f.path {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("未找到文件: %s", f.path)
			}
		}
	})
}

// TestFileStorage_LoadPermissionRestricted 测试加载权限受限的文件
func TestFileStorage_LoadPermissionRestricted(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 创建测试文件
	testData := TestData{Name: "test", Value: 123}
	if err := s.Save("test.json", testData); err != nil {
		t.Fatalf("准备测试数据失败: %v", err)
	}

	// 修改文件权限为不可读
	filePath := filepath.Join(testDir, "test.json")
	if err := os.Chmod(filePath, 0000); err != nil {
		t.Fatalf("修改文件权限失败: %v", err)
	}

	// 尝试加载权限受限的文件
	var loadedData TestData
	err := s.Load("test.json", &loadedData)
	if err == nil {
		t.Error("期望加载权限受限的文件时返回错误")
	}

	// 恢复文件权限以便清理
	os.Chmod(filePath, 0644)
}

// TestFileStorage_DeletePermissionRestricted 测试删除权限受限的文件
func TestFileStorage_DeletePermissionRestricted(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 创建测试文件
	testData := TestData{Name: "test", Value: 123}
	if err := s.Save("test.json", testData); err != nil {
		t.Fatalf("准备测试数据失败: %v", err)
	}

	// 修改文件权限为不可写
	filePath := filepath.Join(testDir, "test.json")
	if err := os.Chmod(filePath, 0444); err != nil {
		t.Fatalf("修改文件权限失败: %v", err)
	}

	// 尝试删除权限受限的文件
	err := s.Delete("test.json")
	if err == nil {
		t.Error("期望删除权限受限的文件时返回错误")
	}

	// 恢复文件权限以便清理
	os.Chmod(filePath, 0644)
}

// TestFileStorage_DeleteDirectory 测试删除目录
func TestFileStorage_DeleteDirectory(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 创建测试目录
	subDir := filepath.Join(testDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 尝试删除目录
	err := s.Delete("subdir")
	if err == nil {
		t.Error("期望删除目录时返回错误")
	}
}

// TestFileStorage_ListPermissionRestricted 测试列出权限受限的目录
func TestFileStorage_ListPermissionRestricted(t *testing.T) {
	testDir, cleanup := setupTestDir(t)
	defer cleanup()

	s := storage.NewFileStorage(testDir)

	// 创建测试目录和文件
	restrictedDir := filepath.Join(testDir, "restricted")
	if err := os.MkdirAll(restrictedDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	testData := TestData{Name: "test", Value: 123}
	if err := s.Save(filepath.Join("restricted", "test.json"), testData); err != nil {
		t.Fatalf("准备测试数据失败: %v", err)
	}

	// 修改目录权限为不可读
	if err := os.Chmod(restrictedDir, 0000); err != nil {
		t.Fatalf("修改目录权限失败: %v", err)
	}

	// 尝试列出权限受限的目录
	_, err := s.List()
	if err == nil {
		t.Error("期望列出权限受限的目录时返回错误")
	}

	// 恢复目录权限以便清理
	os.Chmod(restrictedDir, 0755)
}
