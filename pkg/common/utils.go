package common

import (
	"archive/zip"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateUUID 生成UUID
func GenerateUUID() (string, error) {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return "", fmt.Errorf("生成UUID失败: %v", err)
	}
	return hex.EncodeToString(uuid), nil
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// IsFileExists 检查文件是否存在
func IsFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetWorkDir 获取工作目录
func GetWorkDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取程序路径失败: %v", err)
	}
	return filepath.Dir(exePath), nil
}

// JoinPath 连接路径
func JoinPath(elem ...string) string {
	return filepath.Join(elem...)
}

// CalculateFileHash 计算文件MD5哈希值
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

// ZipOptions 压缩选项
type ZipOptions struct {
	ExcludePatterns []string // 排除的文件模式
	IncludePatterns []string // 包含的文件模式
	MaxSize         int64    // 最大大小限制
	TempDir         string   // 临时目录
}

// CreateZipPackage 创建文件夹的压缩包
func CreateZipPackage(sourcePath string, targetPath string, options *ZipOptions) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 创建zip文件
	zipFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("创建zip文件失败: %v", err)
	}
	defer zipFile.Close()

	// 创建zip写入器
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 遍历源目录
	err = filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过目录本身
		if info.IsDir() {
			return nil
		}

		// 获取相对路径
		relPath, err := filepath.Rel(sourcePath, path)
		if err != nil {
			return fmt.Errorf("获取相对路径失败: %v", err)
		}

		// 创建文件头
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("创建文件头失败: %v", err)
		}
		header.Name = relPath
		header.Method = zip.Deflate

		// 写入文件
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("创建zip文件头失败: %v", err)
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("打开文件失败: %v", err)
		}
		defer file.Close()

		if _, err := io.Copy(writer, file); err != nil {
			return fmt.Errorf("写入文件失败: %v", err)
		}

		return nil
	})

	return err
}

// ExtractZipPackage 解压压缩包到指定目录
func ExtractZipPackage(zipPath string, targetDir string) error {
	// 打开zip文件
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开zip文件失败: %v", err)
	}
	defer reader.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 解压文件
	for _, file := range reader.File {
		// 构建完整的目标路径
		targetPath := filepath.Join(targetDir, file.Name)

		// 确保目标路径在目标目录内
		if !isSubPath(targetDir, targetPath) {
			return fmt.Errorf("非法的文件路径: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			// 创建目录
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("创建目录失败: %v", err)
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建父目录失败: %v", err)
		}

		// 创建目标文件
		targetFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("创建目标文件失败: %v", err)
		}

		// 打开源文件
		sourceFile, err := file.Open()
		if err != nil {
			targetFile.Close()
			return fmt.Errorf("打开源文件失败: %v", err)
		}

		// 复制内容
		_, err = io.Copy(targetFile, sourceFile)
		sourceFile.Close()
		targetFile.Close()

		if err != nil {
			return fmt.Errorf("复制文件内容失败: %v", err)
		}
	}

	return nil
}

// CalculateFileMD5 计算文件的MD5值
func CalculateFileMD5(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算MD5失败: %v", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// isSubPath 检查子路径是否在父路径下
func isSubPath(parent, sub string) bool {
	rel, err := filepath.Rel(parent, sub)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && !strings.HasPrefix(rel, "..")
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
