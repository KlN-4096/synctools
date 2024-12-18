package model_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"synctools/internal/model"
)

// setupTestFile 创建测试文件
func setupTestFile(t *testing.T, content []byte) (string, func()) {
	// 创建临时目录
	dir, err := os.MkdirTemp("", "fileinfo_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 创建测试文件
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, content, 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 返回清理函数
	cleanup := func() {
		os.RemoveAll(dir)
	}

	return path, cleanup
}

// TestFileInfo_Creation 测试创建文件信息对象
func TestFileInfo_Creation(t *testing.T) {
	content := []byte("test content")
	path, cleanup := setupTestFile(t, content)
	defer cleanup()

	// 获取文件信息
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("获取文件信息失败: %v", err)
	}

	testHash := "test-hash"
	modTime := fileInfo.ModTime().Unix()

	// 创建 FileInfo 对象
	info := &model.FileInfo{
		Path:         path,
		Hash:         testHash,
		Size:         fileInfo.Size(),
		ModTime:      modTime,
		IsDirectory:  fileInfo.IsDir(),
		RelativePath: "test.txt",
	}

	// 验证所有字段
	if info.Path != path {
		t.Errorf("Path = %v, want %v", info.Path, path)
	}
	if info.Hash != testHash {
		t.Errorf("Hash = %v, want %v", info.Hash, testHash)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("Size = %v, want %v", info.Size, len(content))
	}
	if info.ModTime != modTime {
		t.Errorf("ModTime = %v, want %v", info.ModTime, modTime)
	}
	if info.IsDirectory {
		t.Error("IsDirectory = true, want false")
	}
	if info.RelativePath != "test.txt" {
		t.Errorf("RelativePath = %v, want test.txt", info.RelativePath)
	}
}

// TestFileInfo_SpecialTypes 测试特殊文件类型
func TestFileInfo_SpecialTypes(t *testing.T) {
	// 创建临时目录
	dir, err := os.MkdirTemp("", "fileinfo_special_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	// 测试目录
	dirPath := filepath.Join(dir, "testdir")
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 获取目录信息
	dirStat, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("获取目录信息失败: %v", err)
	}

	dirInfo := &model.FileInfo{
		Path:         dirPath,
		Size:         dirStat.Size(),
		IsDirectory:  true,
		RelativePath: "testdir",
	}

	if !dirInfo.IsDirectory {
		t.Error("目录的 IsDirectory 应该为 true")
	}
	if dirInfo.Path != dirPath {
		t.Errorf("目录路径不匹配, got %s, want %s", dirInfo.Path, dirPath)
	}
	if dirInfo.Size != dirStat.Size() {
		t.Errorf("目录大小不匹配, got %d, want %d", dirInfo.Size, dirStat.Size())
	}
	if dirInfo.RelativePath != "testdir" {
		t.Errorf("目录相对路径不匹配, got %s, want testdir", dirInfo.RelativePath)
	}

	// 测试空文件
	emptyFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("创建空文件失败: %v", err)
	}

	emptyInfo := &model.FileInfo{
		Path:         emptyFile,
		Size:         0,
		IsDirectory:  false,
		RelativePath: "empty.txt",
	}

	if emptyInfo.Size != 0 {
		t.Error("空文件的大小应该为 0")
	}
	if emptyInfo.Path != emptyFile {
		t.Errorf("空文件路径不匹配, got %s, want %s", emptyInfo.Path, emptyFile)
	}
	if emptyInfo.IsDirectory {
		t.Error("空文件的 IsDirectory 应该为 false")
	}
	if emptyInfo.RelativePath != "empty.txt" {
		t.Errorf("空文件相对路径不匹配, got %s, want empty.txt", emptyInfo.RelativePath)
	}

	// 测试大文件
	bigFile := filepath.Join(dir, "big.dat")
	file, err := os.Create(bigFile)
	if err != nil {
		t.Fatalf("创建大文件失败: %v", err)
	}
	file.Close()

	size := int64(1024 * 1024) // 1MB
	if err := os.Truncate(bigFile, size); err != nil {
		t.Fatalf("调整文件大小失败: %v", err)
	}

	bigInfo := &model.FileInfo{
		Path:         bigFile,
		Size:         size,
		IsDirectory:  false,
		RelativePath: "big.dat",
	}

	if bigInfo.Size != size {
		t.Errorf("大文件的大小不正确, got %d, want %d", bigInfo.Size, size)
	}
	if bigInfo.Path != bigFile {
		t.Errorf("大文件路径不匹配, got %s, want %s", bigInfo.Path, bigFile)
	}
	if bigInfo.IsDirectory {
		t.Error("大文件的 IsDirectory 应该为 false")
	}
	if bigInfo.RelativePath != "big.dat" {
		t.Errorf("大文件相对路径不匹配, got %s, want big.dat", bigInfo.RelativePath)
	}
}

// TestFileInfo_Compare 测试文件信息比较
func TestFileInfo_Compare(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name     string
		info1    *model.FileInfo
		info2    *model.FileInfo
		wantSame bool
	}{
		{
			name: "完全相同",
			info1: &model.FileInfo{
				Path:         "/test/file1.txt",
				Hash:         "hash1",
				Size:         100,
				ModTime:      now,
				IsDirectory:  false,
				RelativePath: "file1.txt",
			},
			info2: &model.FileInfo{
				Path:         "/test/file1.txt",
				Hash:         "hash1",
				Size:         100,
				ModTime:      now,
				IsDirectory:  false,
				RelativePath: "file1.txt",
			},
			wantSame: true,
		},
		{
			name: "不同哈希",
			info1: &model.FileInfo{
				Hash:    "hash1",
				Size:    100,
				ModTime: now,
			},
			info2: &model.FileInfo{
				Hash:    "hash2",
				Size:    100,
				ModTime: now,
			},
			wantSame: false,
		},
		{
			name: "不同大小",
			info1: &model.FileInfo{
				Hash:    "hash1",
				Size:    100,
				ModTime: now,
			},
			info2: &model.FileInfo{
				Hash:    "hash1",
				Size:    200,
				ModTime: now,
			},
			wantSame: false,
		},
		{
			name: "不同修改时间",
			info1: &model.FileInfo{
				Hash:    "hash1",
				Size:    100,
				ModTime: now,
			},
			info2: &model.FileInfo{
				Hash:    "hash1",
				Size:    100,
				ModTime: now + 100,
			},
			wantSame: false,
		},
		{
			name: "一个是目录一个是文件",
			info1: &model.FileInfo{
				Hash:        "hash1",
				Size:        100,
				ModTime:     now,
				IsDirectory: true,
			},
			info2: &model.FileInfo{
				Hash:        "hash1",
				Size:        100,
				ModTime:     now,
				IsDirectory: false,
			},
			wantSame: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			same := tt.info1.Equal(tt.info2)
			if same != tt.wantSame {
				t.Errorf("Equal() = %v, want %v", same, tt.wantSame)
			}
		})
	}
}

// TestFileInfo_Hash 测试文件哈希计算
func TestFileInfo_Hash(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		wantErr bool
	}{
		{
			name:    "普通文本文件",
			content: []byte("test content"),
			wantErr: false,
		},
		{
			name:    "空文件",
			content: []byte{},
			wantErr: false,
		},
		{
			name:    "二进制文件",
			content: []byte{0x00, 0x01, 0x02, 0x03},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, cleanup := setupTestFile(t, tt.content)
			defer cleanup()

			hash1, err := model.CalculateFileHash(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateFileHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// 再次计算哈希，确保结果一致
				hash2, err := model.CalculateFileHash(path)
				if err != nil {
					t.Errorf("Second CalculateFileHash() failed: %v", err)
					return
				}

				if hash1 != hash2 {
					t.Errorf("Hash values not consistent: first = %v, second = %v", hash1, hash2)
				}
			}
		})
	}
}
