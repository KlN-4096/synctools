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

// CompareFiles 比较源目录和目标目录的文件差异
// 返回需要在目标目录进行的操作列表
func CompareFiles(srcDir, dstDir string, ignoreList []string) ([]FileDiff, error) {
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
			return err
		}

		// 获取相对路径
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
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
		return nil, fmt.Errorf("遍历源目录失败: %v", err)
	}

	// 获取目标目录的文件列表
	dstFiles := make(map[string]os.FileInfo)
	err = filepath.Walk(dstDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// 获取相对路径
		relPath, err := filepath.Rel(dstDir, path)
		if err != nil {
			return err
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
		return nil, fmt.Errorf("遍历目标目录失败: %v", err)
	}

	// 比较文件
	for path, srcInfo := range srcFiles {
		dstInfo, exists := dstFiles[path]
		if !exists {
			// 目标目录不存在此文件，需要添加
			hash, err := CalculateFileHash(filepath.Join(srcDir, path))
			if err != nil {
				return nil, fmt.Errorf("计算文件哈希失败 [%s]: %v", path, err)
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
				return nil, fmt.Errorf("计算源文件哈希失败 [%s]: %v", path, err)
			}
			// 计算目标文件哈希
			dstHash, err := CalculateFileHash(filepath.Join(dstDir, path))
			if err != nil {
				return nil, fmt.Errorf("计算目标文件哈希失败 [%s]: %v", path, err)
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
	for _, diff := range diffs {
		srcPath := filepath.Join(srcDir, diff.Path)
		dstPath := filepath.Join(dstDir, diff.Path)

		switch diff.Action {
		case FileActionAdd, FileActionUpdate:
			// 确保目标目录存在
			if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
				return fmt.Errorf("创建目标目录失败 [%s]: %v", diff.Path, err)
			}
			// 复制文件
			if err := CopyFile(srcPath, dstPath); err != nil {
				return fmt.Errorf("复制文件失败 [%s]: %v", diff.Path, err)
			}
			// 保持文件时间一致
			if err := os.Chtimes(dstPath, diff.FileInfo.ModTime(), diff.FileInfo.ModTime()); err != nil {
				return fmt.Errorf("设置文件时间失败 [%s]: %v", diff.Path, err)
			}

		case FileActionDelete:
			// 删除文件
			if err := os.Remove(dstPath); err != nil {
				if !os.IsNotExist(err) {
					return fmt.Errorf("删除文件失败 [%s]: %v", diff.Path, err)
				}
			}
		}
	}
	return nil
}

// CopyFile 复制文件
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// CalculateFileHash 计算文件的哈希值
func CalculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CleanupTempFiles 清理临时文件
func CleanupTempFiles(dir string, maxAge time.Duration) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取目录失败: %v", err)
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
			os.Remove(path)
		}
	}

	return nil
}
