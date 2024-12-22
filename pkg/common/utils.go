/*
文件作用:
- 提供通用工具函数
- 实现文件操作功能
- 提供加密和解密方法
- 处理路径和目录操作

主要方法:
- CalculateFileHash: 计算文件哈希值
- CompressFiles: 压缩文件
- DecompressFiles: 解压文件
- ValidatePath: 验证路径有效性
- EnsureDir: 确保目录存在
*/

package common

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
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

// FileError 文件操作错误
type FileError struct {
	Op      string // 操作名称
	Path    string // 文件路径
	Message string // 错误消息
	Err     error  // 原始错误
}

// Error 实现error接口
func (e *FileError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s [%s]: %s: %v", e.Op, e.Path, e.Message, e.Err)
	}
	return fmt.Sprintf("%s [%s]: %s", e.Op, e.Path, e.Message)
}

// PathError 路径操作错误
type PathError struct {
	Op      string // 操作名称
	Path    string // 路径
	Message string // 错误消息
	Err     error  // 原始错误
}

// Error 实现error接口
func (e *PathError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s [%s]: %s: %v", e.Op, e.Path, e.Message, e.Err)
	}
	return fmt.Sprintf("%s [%s]: %s", e.Op, e.Path, e.Message)
}

// CompareFiles 比较源目录和目标目录的文件差异
func CompareFiles(srcDir, dstDir string, ignoreList []string) ([]FileDiff, error) {
	// 验证源目录
	if err := ValidatePath(srcDir, true); err != nil {
		return nil, &PathError{
			Op:      "CompareFiles",
			Path:    srcDir,
			Message: "源目录无效",
			Err:     err,
		}
	}

	// 验证目标目录
	if err := ValidatePath(dstDir, true); err != nil {
		if !os.IsNotExist(err) {
			return nil, &PathError{
				Op:      "CompareFiles",
				Path:    dstDir,
				Message: "目标目录无效",
				Err:     err,
			}
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
			return &FileError{
				Op:      "CompareFiles",
				Path:    path,
				Message: "遍历源目录失败",
				Err:     err,
			}
		}

		// 获取相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return &PathError{
				Op:      "CompareFiles",
				Path:    path,
				Message: "获取相对路径失败",
				Err:     err,
			}
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
			return &FileError{
				Op:      "CompareFiles",
				Path:    path,
				Message: "遍历目标目录失败",
				Err:     err,
			}
		}

		// 获取相对路径
		relPath, err := filepath.Rel(dstDir, path)
		if err != nil {
			return &PathError{
				Op:      "CompareFiles",
				Path:    path,
				Message: "获取相对路径失败",
				Err:     err,
			}
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
				return nil, &FileError{
					Op:      "CompareFiles",
					Path:    path,
					Message: "计算文件哈希失败",
					Err:     err,
				}
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
				return nil, &FileError{
					Op:      "CompareFiles",
					Path:    path,
					Message: "计算源文件哈希失败",
					Err:     err,
				}
			}
			// 计算目标文件哈希
			dstHash, err := CalculateFileHash(filepath.Join(dstDir, path))
			if err != nil {
				return nil, &FileError{
					Op:      "CompareFiles",
					Path:    path,
					Message: "计算目标文件哈希失败",
					Err:     err,
				}
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

	// 剩余的目标文件需要删除
	for path, info := range dstFiles {
		diffs = append(diffs, FileDiff{
			Path:     path,
			Action:   FileActionDelete,
			FileInfo: info,
		})
	}

	return diffs, nil
}

// SyncFiles 根据差异列表同步文件
func SyncFiles(srcDir, dstDir string, diffs []FileDiff) error {
	// 验证源目录
	if err := ValidatePath(srcDir, true); err != nil {
		return &PathError{
			Op:      "SyncFiles",
			Path:    srcDir,
			Message: "源目录无效",
			Err:     err,
		}
	}

	// 验证目标目录
	if err := ValidatePath(dstDir, true); err != nil {
		if !os.IsNotExist(err) {
			return &PathError{
				Op:      "SyncFiles",
				Path:    dstDir,
				Message: "目标目录无效",
				Err:     err,
			}
		}
	}

	for _, diff := range diffs {
		srcPath := filepath.Join(srcDir, diff.Path)
		dstPath := filepath.Join(dstDir, diff.Path)

		switch diff.Action {
		case FileActionAdd, FileActionUpdate:
			// 确保目标目录存在
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return &FileError{
					Op:      "SyncFiles",
					Path:    dstPath,
					Message: "创建目标目录失败",
					Err:     err,
				}
			}
			// 复制文件
			if err := CopyFile(srcPath, dstPath); err != nil {
				return &FileError{
					Op:      "SyncFiles",
					Path:    diff.Path,
					Message: "复制文件失败",
					Err:     err,
				}
			}
			// 保持文件时间一致
			if err := os.Chtimes(dstPath, diff.FileInfo.ModTime(), diff.FileInfo.ModTime()); err != nil {
				return &FileError{
					Op:      "SyncFiles",
					Path:    diff.Path,
					Message: "设置文件时间失败",
					Err:     err,
				}
			}

		case FileActionDelete:
			// 删除文件
			if err := os.Remove(dstPath); err != nil {
				if !os.IsNotExist(err) {
					return &FileError{
						Op:      "SyncFiles",
						Path:    diff.Path,
						Message: "删除文件失败",
						Err:     err,
					}
				}
			}
		}
	}
	return nil
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	// 验证源文件
	if err := ValidatePath(src, false); err != nil {
		return &FileError{
			Op:      "CopyFile",
			Path:    src,
			Message: "源文件无效",
			Err:     err,
		}
	}

	// 打开源文件
	srcFile, err := os.Open(src)
	if err != nil {
		return &FileError{
			Op:      "CopyFile",
			Path:    src,
			Message: "打开源文件失败",
			Err:     err,
		}
	}
	defer srcFile.Close()

	// 创建目标文件
	dstFile, err := os.Create(dst)
	if err != nil {
		return &FileError{
			Op:      "CopyFile",
			Path:    dst,
			Message: "创建目标文件失败",
			Err:     err,
		}
	}
	defer dstFile.Close()

	// 复制文件内容
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return &FileError{
			Op:      "CopyFile",
			Path:    dst,
			Message: "复制文件内容失败",
			Err:     err,
		}
	}

	return nil
}

// CalculateFileHash 计算文件的哈希值
func CalculateFileHash(path string) (string, error) {
	// 验证文件
	if err := ValidatePath(path, false); err != nil {
		return "", &FileError{
			Op:      "CalculateFileHash",
			Path:    path,
			Message: "文件无效",
			Err:     err,
		}
	}

	// 打开文件
	file, err := os.Open(path)
	if err != nil {
		return "", &FileError{
			Op:      "CalculateFileHash",
			Path:    path,
			Message: "打开文件失败",
			Err:     err,
		}
	}
	defer file.Close()

	// 计算哈希值
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", &FileError{
			Op:      "CalculateFileHash",
			Path:    path,
			Message: "计算哈希值失败",
			Err:     err,
		}
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CleanupTempFiles 清理临时文件
func CleanupTempFiles(dir string, maxAge time.Duration) error {
	// 验证目录
	if err := ValidatePath(dir, true); err != nil {
		return &PathError{
			Op:      "CleanupTempFiles",
			Path:    dir,
			Message: "目录无效",
			Err:     err,
		}
	}

	// 读取目录内容
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return &PathError{
			Op:      "CleanupTempFiles",
			Path:    dir,
			Message: "读取目录失败",
			Err:     err,
		}
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > maxAge {
			path := filepath.Join(dir, entry.Name())
			if err := os.Remove(path); err != nil {
				return &FileError{
					Op:      "CleanupTempFiles",
					Path:    path,
					Message: "删除文件失败",
					Err:     err,
				}
			}
		}
	}

	return nil
}

// ValidatePath 验证路径
func ValidatePath(path string, isDir bool) error {
	if path == "" {
		return fmt.Errorf("路径为空")
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fmt.Errorf("获取路径信息失败: %v", err)
	}

	if isDir != info.IsDir() {
		if isDir {
			return fmt.Errorf("路径不是目录")
		}
		return fmt.Errorf("路径不是文件")
	}

	return nil
}

// GetRelativePath 获取相对路径
func GetRelativePath(basePath, targetPath string) (string, error) {
	// 验证基础路径
	if err := ValidatePath(basePath, true); err != nil {
		return "", &PathError{
			Op:      "GetRelativePath",
			Path:    basePath,
			Message: "基础路径无效",
			Err:     err,
		}
	}

	// 获取相对路径
	relPath, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return "", &PathError{
			Op:      "GetRelativePath",
			Path:    targetPath,
			Message: "获取相对路径失败",
			Err:     err,
		}
	}

	return relPath, nil
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	// 验证路径
	if path == "" {
		return &PathError{
			Op:      "EnsureDir",
			Path:    path,
			Message: "路径为空",
		}
	}

	// 创建目录
	if err := os.MkdirAll(path, 0755); err != nil {
		return &PathError{
			Op:      "EnsureDir",
			Path:    path,
			Message: "创建目录失败",
			Err:     err,
		}
	}

	return nil
}

// IsPathExists 检查路径是否存在
func IsPathExists(path string) (bool, error) {
	if path == "" {
		return false, &PathError{
			Op:      "IsPathExists",
			Path:    path,
			Message: "路径为空",
		}
	}

	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, &PathError{
		Op:      "IsPathExists",
		Path:    path,
		Message: "检查路径失败",
		Err:     err,
	}
}
