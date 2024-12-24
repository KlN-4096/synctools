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
	filePath := filepath.Join(s.baseDir, filepath.Clean(key))

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 处理不同类型的数据
	var fileData []byte
	switch v := data.(type) {
	case []byte:
		fileData = v
	case string:
		fileData = []byte(v)
	default:
		// 如果是其他类型，尝试JSON序列化
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("序列化数据失败: %v", err)
		}
		fileData = jsonData
	}

	if err := ioutil.WriteFile(filePath, fileData, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// Load 从文件加载数据
func (s *FileStorage) Load(key string, data interface{}) error {
	filePath := filepath.Join(s.baseDir, filepath.Clean(key))

	fileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}

	// 处理不同类型的数据
	switch v := data.(type) {
	case *[]byte:
		*v = fileData
	case *string:
		*v = string(fileData)
	default:
		// 如果是其他类型，尝试JSON反序列化
		if err := json.Unmarshal(fileData, data); err != nil {
			return fmt.Errorf("反序列化数据失败: %v", err)
		}
	}

	return nil
}

// Delete 删除文件
func (s *FileStorage) Delete(key string) error {
	filePath := filepath.Join(s.baseDir, filepath.Clean(key))

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("删除文件失败: %v", err)
	}

	return nil
}

// Exists 检查文件是否存在
func (s *FileStorage) Exists(key string) bool {
	filePath := filepath.Join(s.baseDir, filepath.Clean(key))
	_, err := os.Stat(filePath)
	return err == nil
}

// List 列出所有文件
func (s *FileStorage) List() ([]string, error) {
	var fileList []string
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(s.baseDir, path)
			if err != nil {
				return fmt.Errorf("获取相对路径失败: %v", err)
			}
			fileList = append(fileList, filepath.ToSlash(relPath))
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("遍历目录失败: %v", err)
	}

	return fileList, nil
}
