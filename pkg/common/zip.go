package common

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ExtractZipPackage 解压ZIP文件到指定目录
func ExtractZipPackage(zipPath, destPath string) error {
	// 打开ZIP文件
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开ZIP文件失败: %v", err)
	}
	defer reader.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %v", err)
	}

	// 遍历ZIP文件中的所有文件
	for _, file := range reader.File {
		// 构建完整的目标路径
		path := filepath.Join(destPath, file.Name)

		// 如果是目录，创建它
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return fmt.Errorf("创建目录失败 [%s]: %v", file.Name, err)
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("创建父目录失败 [%s]: %v", file.Name, err)
		}

		// 创建目标文件
		dstFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf("创建目标文件失败 [%s]: %v", file.Name, err)
		}

		// 打开源文件
		srcFile, err := file.Open()
		if err != nil {
			dstFile.Close()
			return fmt.Errorf("打开源文件失败 [%s]: %v", file.Name, err)
		}

		// 复制文件内容
		if _, err := io.Copy(dstFile, srcFile); err != nil {
			dstFile.Close()
			srcFile.Close()
			return fmt.Errorf("复制文件内容失败 [%s]: %v", file.Name, err)
		}

		// 关闭文件
		dstFile.Close()
		srcFile.Close()
	}

	return nil
}
