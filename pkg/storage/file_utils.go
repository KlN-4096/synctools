package storage

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"synctools/internal/interfaces"
	"synctools/pkg/errors"
)

// FileAction 文件操作类型
type FileAction string

const (
	FileActionAdd    FileAction = "add"    // 添加文件
	FileActionDelete FileAction = "delete" // 删除文件
	FileActionUpdate FileAction = "update" // 更新文件
)

// FileDiff 文件差异信息
type FileDiff struct {
	Path     string      // 文件路径
	Action   FileAction  // 操作类型
	FileInfo os.FileInfo // 文件信息
	Hash     string      // 文件哈希值
}

// CompareFiles 比较源目录和目标目录的文件差异
func CompareFiles(srcDir, dstDir string, ignoreList []string) ([]FileDiff, error) {
	// 验证源目录
	if err := ValidatePath(srcDir, true); err != nil {
		return nil, errors.NewStorageError("CompareFiles", "源目录无效", err)
	}

	// 验证目标目录
	if err := ValidatePath(dstDir, true); err != nil {
		if !os.IsNotExist(err) {
			return nil, errors.NewStorageError("CompareFiles", "目标目录无效", err)
		}
	}

	var diffs []FileDiff

	// 创建忽略文件匹配器
	ignoreMatches := make(map[string]bool)
	for _, pattern := range ignoreList {
		ignoreMatches[pattern] = true
	}

	// 获取源目录的文件列表
	srcFiles := make(map[string]os.FileInfo)
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.NewStorageError("CompareFiles", "遍历源目录失败", err)
		}

		// 获取相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return errors.NewStorageError("CompareFiles", "获取相对路径失败", err)
		}

		// 检查是否在忽略列表中
		if ignoreMatches[relPath] || ignoreMatches[info.Name()] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			srcFiles[relPath] = info
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 获取目标目录的文件列表
	dstFiles := make(map[string]os.FileInfo)
	err = filepath.Walk(dstDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return errors.NewStorageError("CompareFiles", "遍历目标目录失败", err)
		}

		// 获取相对路径
		relPath, err := filepath.Rel(dstDir, path)
		if err != nil {
			return errors.NewStorageError("CompareFiles", "获取相对路径失败", err)
		}

		// 检查是否在忽略列表中
		if ignoreMatches[relPath] || ignoreMatches[info.Name()] {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			dstFiles[relPath] = info
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 比较文件
	for path, srcInfo := range srcFiles {
		dstInfo, exists := dstFiles[path]
		if !exists {
			// 目标目录不存在此文件，需要添加
			hash, err := CalculateFileHash(filepath.Join(srcDir, path))
			if err != nil {
				return nil, errors.NewStorageError("CompareFiles", "计算文件哈希失败", err)
			}
			diffs = append(diffs, FileDiff{
				Path:     path,
				Action:   FileActionAdd,
				FileInfo: srcInfo,
				Hash:     hash,
			})
			continue
		}

		// 文件存在，检查是否需要更新
		if srcInfo.ModTime().Unix() != dstInfo.ModTime().Unix() ||
			srcInfo.Size() != dstInfo.Size() {
			// 计算源文件哈希
			srcHash, err := CalculateFileHash(filepath.Join(srcDir, path))
			if err != nil {
				return nil, errors.NewStorageError("CompareFiles", "计算源文件哈希失败", err)
			}
			// 计算目标文件哈希
			dstHash, err := CalculateFileHash(filepath.Join(dstDir, path))
			if err != nil {
				return nil, errors.NewStorageError("CompareFiles", "计算目标文件哈希失败", err)
			}
			// 只有当哈希值不同时才需要更新
			if srcHash != dstHash {
				diffs = append(diffs, FileDiff{
					Path:     path,
					Action:   FileActionUpdate,
					FileInfo: srcInfo,
					Hash:     srcHash,
				})
			}
		}
		delete(dstFiles, path)
	}

	// 处理需要删除的文件
	for path, info := range dstFiles {
		diffs = append(diffs, FileDiff{
			Path:     path,
			Action:   FileActionDelete,
			FileInfo: info,
		})
	}

	return diffs, nil
}

// CalculateFileHash 计算文件的MD5哈希值
func CalculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", errors.NewStorageError("CalculateFileHash", "打开文件失败", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", errors.NewStorageError("CalculateFileHash", "计算哈希失败", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ValidatePath 验证路径是否有效
func ValidatePath(path string, mustExist bool) error {
	if path == "" {
		return errors.NewStorageError("ValidatePath", "路径为空", nil)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) && !mustExist {
			return nil
		}
		return errors.NewStorageError("ValidatePath", "获取路径信息失败", err)
	}

	if !info.IsDir() {
		return errors.NewStorageError("ValidatePath", "路径不是目录", nil)
	}

	return nil
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return errors.NewStorageError("EnsureDir", "创建目录失败", err)
	}
	return nil
}

// GetFileInfo 获取文件信息
func GetFileInfo(path string) (*interfaces.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, errors.NewStorageError("GetFileInfo", "获取文件信息失败", err)
	}

	hash := ""
	if !info.IsDir() {
		hash, err = CalculateFileHash(path)
		if err != nil {
			return nil, err
		}
	}

	return &interfaces.FileInfo{
		Path:         path,
		Hash:         hash,
		Size:         info.Size(),
		ModTime:      info.ModTime().Unix(),
		IsDirectory:  info.IsDir(),
		RelativePath: filepath.Base(path),
	}, nil
}
