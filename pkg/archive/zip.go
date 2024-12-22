package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Error 压缩操作错误
type Error struct {
	Op      string // 操作名称
	Path    string // 文件路径
	Message string // 错误消息
	Err     error  // 原始错误
}

// Error 实现error接口
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s [%s]: %s: %v", e.Op, e.Path, e.Message, e.Err)
	}
	return fmt.Sprintf("%s [%s]: %s", e.Op, e.Path, e.Message)
}

// Progress 压缩进度信息
type Progress struct {
	CurrentFile   string    // 当前处理的文件
	TotalFiles    int       // 总文件数
	ProcessedNum  int       // 已处理文件数
	TotalSize     int64     // 总大小
	ProcessedSize int64     // 已处理大小
	StartTime     time.Time // 开始时间
	Speed         float64   // 处理速度 (bytes/s)
}

// CompressFiles 压缩文件到ZIP
func CompressFiles(srcPath, zipPath string, ignoreList []string) (*Progress, error) {
	// 验证源路径
	if _, err := os.Stat(srcPath); err != nil {
		return nil, &Error{
			Op:      "CompressFiles",
			Path:    srcPath,
			Message: "源路径无效",
			Err:     err,
		}
	}

	// 创建ZIP文件
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return nil, &Error{
			Op:      "CompressFiles",
			Path:    zipPath,
			Message: "创建ZIP文件失败",
			Err:     err,
		}
	}
	defer zipFile.Close()

	// 创建ZIP写入器
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 创建进度信息
	progress := &Progress{
		StartTime: time.Now(),
	}

	// 遍历源目录
	err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return &Error{
				Op:      "CompressFiles",
				Path:    path,
				Message: "遍历目录失败",
				Err:     err,
			}
		}

		// 获取相对路径
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return &Error{
				Op:      "CompressFiles",
				Path:    path,
				Message: "获取相对路径失败",
				Err:     err,
			}
		}

		// 检查是否在忽略列表中
		for _, ignore := range ignoreList {
			matched, err := filepath.Match(ignore, relPath)
			if err != nil {
				return &Error{
					Op:      "CompressFiles",
					Path:    path,
					Message: "匹配忽略规则失败",
					Err:     err,
				}
			}
			if matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 更新进度信息
		progress.CurrentFile = relPath
		progress.TotalFiles++
		progress.TotalSize += info.Size()

		// 如果是目录，跳过
		if info.IsDir() {
			return nil
		}

		// 创建文件头
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return &Error{
				Op:      "CompressFiles",
				Path:    path,
				Message: "创建文件头失败",
				Err:     err,
			}
		}

		// 设置压缩方法
		header.Method = zip.Deflate
		// 设置相对路径
		header.Name = relPath

		// 创建文件写入器
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return &Error{
				Op:      "CompressFiles",
				Path:    path,
				Message: "创建文件写入器失败",
				Err:     err,
			}
		}

		// 打开源文件
		file, err := os.Open(path)
		if err != nil {
			return &Error{
				Op:      "CompressFiles",
				Path:    path,
				Message: "打开源文件失败",
				Err:     err,
			}
		}
		defer file.Close()

		// 复制文件内容
		written, err := io.Copy(writer, file)
		if err != nil {
			return &Error{
				Op:      "CompressFiles",
				Path:    path,
				Message: "写入文件内容失败",
				Err:     err,
			}
		}

		// 更新进度信息
		progress.ProcessedNum++
		progress.ProcessedSize += written
		progress.Speed = float64(progress.ProcessedSize) / time.Since(progress.StartTime).Seconds()

		return nil
	})

	if err != nil {
		return progress, err
	}

	return progress, nil
}

// DecompressFiles 解压ZIP文件
func DecompressFiles(zipPath, destPath string) (*Progress, error) {
	// 验证ZIP文件
	if _, err := os.Stat(zipPath); err != nil {
		return nil, &Error{
			Op:      "DecompressFiles",
			Path:    zipPath,
			Message: "ZIP文件无效",
			Err:     err,
		}
	}

	// 打开ZIP文件
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, &Error{
			Op:      "DecompressFiles",
			Path:    zipPath,
			Message: "打开ZIP文件失败",
			Err:     err,
		}
	}
	defer reader.Close()

	// 创建进度信息
	progress := &Progress{
		StartTime:  time.Now(),
		TotalFiles: len(reader.File),
	}

	// 计算总大小
	for _, file := range reader.File {
		progress.TotalSize += int64(file.UncompressedSize64)
	}

	// 确保目标目录存在
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return nil, &Error{
			Op:      "DecompressFiles",
			Path:    destPath,
			Message: "创建目标目录失败",
			Err:     err,
		}
	}

	// 遍历压缩文件
	for _, file := range reader.File {
		progress.CurrentFile = file.Name

		// 构建完整路径
		path := filepath.Join(destPath, file.Name)

		// 如果是目录，创建它
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(path, file.Mode()); err != nil {
				return nil, &Error{
					Op:      "DecompressFiles",
					Path:    path,
					Message: "创建目录失败",
					Err:     err,
				}
			}
			continue
		}

		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, &Error{
				Op:      "DecompressFiles",
				Path:    path,
				Message: "创建父目录失败",
				Err:     err,
			}
		}

		// 创建文件
		outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return nil, &Error{
				Op:      "DecompressFiles",
				Path:    path,
				Message: "创建文件失败",
				Err:     err,
			}
		}

		// 打开压缩文件
		rc, err := file.Open()
		if err != nil {
			outFile.Close()
			return nil, &Error{
				Op:      "DecompressFiles",
				Path:    file.Name,
				Message: "打开压缩文件失败",
				Err:     err,
			}
		}

		// 复制文件内容
		written, err := io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return nil, &Error{
				Op:      "DecompressFiles",
				Path:    path,
				Message: "写入文件内容失败",
				Err:     err,
			}
		}

		// 更新进度信息
		progress.ProcessedNum++
		progress.ProcessedSize += written
		progress.Speed = float64(progress.ProcessedSize) / time.Since(progress.StartTime).Seconds()
	}

	return progress, nil
}

// ValidateZip 验证ZIP文件完整性
func ValidateZip(zipPath string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return &Error{
			Op:      "ValidateZip",
			Path:    zipPath,
			Message: "打开ZIP文件失败",
			Err:     err,
		}
	}
	defer reader.Close()

	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return &Error{
				Op:      "ValidateZip",
				Path:    file.Name,
				Message: "打开压缩文件失败",
				Err:     err,
			}
		}

		_, err = io.Copy(io.Discard, rc)
		rc.Close()

		if err != nil {
			return &Error{
				Op:      "ValidateZip",
				Path:    file.Name,
				Message: "读取文件内容失败",
				Err:     err,
			}
		}
	}

	return nil
}
