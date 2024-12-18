package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Storage 存储接口
type Storage interface {
	// Save 保存数据到存储
	Save(key string, data interface{}) error

	// Load 从存储加载数据
	Load(key string, data interface{}) error

	// Delete 从存储删除数据
	Delete(key string) error

	// List 列出所有存储的键
	List() ([]string, error)
}

// FileStorage 基于文件的存储实现
type FileStorage struct {
	baseDir string
}

// NewFileStorage 创建新的文件存储
func NewFileStorage(baseDir string) *FileStorage {
	return &FileStorage{
		baseDir: baseDir,
	}
}

// Save 保存数据到文件
func (s *FileStorage) Save(key string, data interface{}) error {
	// 确保目录存在
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}

	// 序列化数据
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(filepath.Join(s.baseDir, key), content, 0644)
}

// Load 从文件加载数据
func (s *FileStorage) Load(key string, data interface{}) error {
	// 读取文件
	content, err := os.ReadFile(filepath.Join(s.baseDir, key))
	if err != nil {
		return err
	}

	// 反序列化数据
	return json.Unmarshal(content, data)
}

// Delete 删除文件
func (s *FileStorage) Delete(key string) error {
	return os.Remove(filepath.Join(s.baseDir, key))
}

// List 列出目录中的所有文件
func (s *FileStorage) List() ([]string, error) {
	var files []string

	// 读取目录
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	// 收集文件名
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	return files, nil
}
