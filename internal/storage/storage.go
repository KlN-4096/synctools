/*
Package storage 实现了数据持久化存储功能。

文件作用：
- 提供统一的存储接口
- 实现基于文件系统的存储
- 支持多种数据类型的存储和加载
- 管理配置文件的持久化

主要类型：
- Storage: 存储接口定义
- FileStorage: 基于文件系统的存储实现

主要方法：
- NewFileStorage: 创建新的文件存储实例
- Save: 保存数据到存储
- Load: 从存储加载数据
- Delete: 删除存储的数据
- List: 列出所有存储的数据项
*/

package storage

import (
	"encoding/json"
	"io"
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
	filePath := filepath.Join(s.baseDir, key)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// 根据数据类型选择保存方式
	switch v := data.(type) {
	case []byte:
		// 二进制数据直接写入
		return os.WriteFile(filePath, v, 0644)
	case io.Reader:
		// 从 Reader 读取数据
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, v)
		return err
	default:
		// 其他类型序列化为 JSON
		content, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		return os.WriteFile(filePath, content, 0644)
	}
}

// Load 从文件加载数据
func (s *FileStorage) Load(key string, data interface{}) error {
	filePath := filepath.Join(s.baseDir, key)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// 根据数据类型选择加载方式
	switch v := data.(type) {
	case *[]byte:
		// 二进制数据直接复制
		*v = make([]byte, len(content))
		copy(*v, content)
		return nil
	case io.Writer:
		// 写入到 Writer
		_, err = v.Write(content)
		return err
	default:
		// 其他类型从 JSON 反序列化
		return json.Unmarshal(content, data)
	}
}

// Delete 删除文件
func (s *FileStorage) Delete(key string) error {
	return os.Remove(filepath.Join(s.baseDir, key))
}

// List 列出目录中的所有文件
func (s *FileStorage) List() ([]string, error) {
	var files []string

	// 遍历目录
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 获取相对路径
		relPath, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return err
		}

		// 统一使用斜杠作为分隔符
		relPath = filepath.ToSlash(relPath)
		files = append(files, relPath)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
