package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// FileStorage 文件存储实现
type FileStorage struct {
	baseDir string
}

// NewFileStorage 创建新的文件存储
func NewFileStorage(baseDir string) (*FileStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %v", err)
	}

	return &FileStorage{
		baseDir: baseDir,
	}, nil
}

// Save 保存数据到文件
func (s *FileStorage) Save(key string, data interface{}) error {
	filePath := filepath.Join(s.baseDir, key)

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %v", err)
	}

	if err := ioutil.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// Load 从文件加载数据
func (s *FileStorage) Load(key string, data interface{}) error {
	filePath := filepath.Join(s.baseDir, key)

	jsonData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}

	if err := json.Unmarshal(jsonData, data); err != nil {
		return fmt.Errorf("反序列化数据失败: %v", err)
	}

	return nil
}

// Delete 删除文件
func (s *FileStorage) Delete(key string) error {
	filePath := filepath.Join(s.baseDir, key)

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("删除文件失败: %v", err)
	}

	return nil
}

// Exists 检查文件是否存在
func (s *FileStorage) Exists(key string) bool {
	filePath := filepath.Join(s.baseDir, key)
	_, err := os.Stat(filePath)
	return err == nil
}

// List 列出所有文件
func (s *FileStorage) List() ([]string, error) {
	files, err := ioutil.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %v", err)
	}

	var fileList []string
	for _, file := range files {
		if !file.IsDir() {
			fileList = append(fileList, file.Name())
		}
	}

	return fileList, nil
}
