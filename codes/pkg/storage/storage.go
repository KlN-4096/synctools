package storage

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"synctools/codes/internal/interfaces"
)

// FileAction 文件操作类型
type FileAction string

const (
	FileActionAdd    FileAction = "add"    // 添加文件
	FileActionDelete FileAction = "delete" // 删除文件
	FileActionUpdate FileAction = "update" // 更新文件
)

// FileStorage 文件存储实现
type FileStorage struct {
	baseDir string
	logger  interfaces.Logger
}

// NewFileStorage 创建新的文件存储
func NewFileStorage(baseDir string, logger interfaces.Logger) (*FileStorage, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %v", err)
	}

	return &FileStorage{
		baseDir: baseDir,
		logger:  logger,
	}, nil
}

// BaseDir 获取基础目录
func (s *FileStorage) BaseDir() string {
	return s.baseDir
}

// Save 保存数据到文件
func (s *FileStorage) Save(key string, data interface{}) error {
	// 统一使用 / 作为路径分隔符
	key = filepath.ToSlash(key)
	// 转换为系统路径
	filePath := filepath.Join(s.baseDir, key)

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
			s.logger.Error("序列化数据失败", interfaces.Fields{
				"error": err,
			})
			return fmt.Errorf("序列化数据失败: %v", err)
		}
		fileData = jsonData
	}

	s.logger.Debug("保存文件", interfaces.Fields{
		"path":     filePath,
		"baseDir":  s.baseDir,
		"key":      key,
		"dataSize": len(fileData),
	})

	// 确保目标目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		s.logger.Error("创建目录失败", interfaces.Fields{
			"dir":   dir,
			"error": err,
		})
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, fileData, 0644); err != nil {
		s.logger.Error("写入文件失败", interfaces.Fields{
			"path":  filePath,
			"error": err,
		})
		return fmt.Errorf("写入文件失败: %v", err)
	}

	s.logger.Debug("文件保存成功", interfaces.Fields{
		"path": filePath,
		"size": len(fileData),
	})
	return nil
}

// Load 从文件加载数据
func (s *FileStorage) Load(key string, data interface{}) error {
	// 统一使用 / 作为路径分隔符
	key = filepath.ToSlash(key)
	// 清理路径
	key = filepath.Clean(key)
	// 转换为系统路径
	filePath := filepath.Join(s.baseDir, key)

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

// List 列出所有文件
func (s *FileStorage) List() ([]string, error) {
	var fileList []string
	err := filepath.Walk(s.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// 获取相对路径
			relPath, err := filepath.Rel(s.baseDir, path)
			if err != nil {
				return err
			}
			// 统一使用 / 作为路径分隔符
			relPath = filepath.ToSlash(relPath)
			fileList = append(fileList, relPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("遍历目录失败: %v", err)
	}
	return fileList, nil
}
