package message

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"synctools/codes/internal/interfaces"
)

// MessageSender 消息发送器
type MessageSender struct {
	logger interfaces.Logger
}

// NewMessageSender 创建新的消息发送器
func NewMessageSender(logger interfaces.Logger) *MessageSender {
	return &MessageSender{
		logger: logger,
	}
}

// SendMessage 发送消息到指定连接
func (s *MessageSender) SendMessage(conn net.Conn, msgType string, uuid string, payload interface{}) error {
	if conn == nil {
		return fmt.Errorf("连接为空")
	}

	// 设置写入超时
	if err := conn.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("设置写入超时失败: %v", err)
	}
	defer conn.SetWriteDeadline(time.Time{}) // 清除超时设置

	// 将payload转换为json.RawMessage
	var payloadJSON json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			s.logger.Error("序列化payload失败", interfaces.Fields{
				"error":   err,
				"payload": payload,
			})
			return fmt.Errorf("序列化payload失败: %v", err)
		}
		payloadJSON = json.RawMessage(data)
	}

	msg := &interfaces.Message{
		Type:    msgType,
		UUID:    uuid,
		Payload: payloadJSON,
	}

	// 先序列化整个消息，确保格式正确
	data, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("序列化消息失败", interfaces.Fields{
			"error": err,
			"msg":   msg,
		})
		return fmt.Errorf("序列化消息失败: %v", err)
	}

	s.logger.Debug("发送消息", interfaces.Fields{
		"type":    msgType,
		"uuid":    uuid,
		"payload": s.FormatPayload(payloadJSON),
		"data":    string(data),
	})

	// 写入数据
	if _, err := conn.Write(append(data, '\n')); err != nil {
		s.logger.Error("写入数据失败", interfaces.Fields{
			"error": err,
			"data":  string(data),
		})
		return fmt.Errorf("发送消息失败: %v", err)
	}

	return nil
}

// ReceiveMessage 从连接接收消息
func (s *MessageSender) ReceiveMessage(conn net.Conn) (*interfaces.Message, error) {
	if conn == nil {
		return nil, fmt.Errorf("连接为空")
	}

	// 设置读取超时
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return nil, fmt.Errorf("设置读取超时失败: %v", err)
	}
	defer conn.SetReadDeadline(time.Time{}) // 清除超时设置

	// 读取原始数据
	decoder := json.NewDecoder(conn)
	var rawData json.RawMessage
	if err := decoder.Decode(&rawData); err != nil {
		s.logger.Error("读取原始数据失败", interfaces.Fields{
			"error": err,
			"data":  string(rawData),
		})
		return nil, fmt.Errorf("接收消息失败: %v", err)
	}

	s.logger.Debug("接收原始数据", interfaces.Fields{
		"data": s.FormatPayload(rawData),
	})

	// 尝试解析消息
	var msg interfaces.Message
	if err := json.Unmarshal(rawData, &msg); err != nil {
		s.logger.Error("解析消息失败", interfaces.Fields{
			"error": err,
			"data":  string(rawData),
		})
		return nil, fmt.Errorf("解析消息失败: %v", err)
	}

	s.logger.Debug("接收消息", interfaces.Fields{
		"type":    msg.Type,
		"uuid":    msg.UUID,
		"payload": s.FormatPayload(msg.Payload),
	})

	return &msg, nil
}

// SendFile 发送文件
func (s *MessageSender) SendFile(conn net.Conn, uuid string, path string, progress chan<- interfaces.Progress) error {
	// 1. 发送文件信息
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	// 2. 读取整个文件内容
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %v", err)
	}

	// 3. 发送文件内容
	chunk := struct {
		Data []byte `json:"data"`
	}{
		Data: fileContent,
	}

	// 发送前更新进度为0%
	if progress != nil {
		progress <- interfaces.Progress{
			Total:     fileInfo.Size(),
			Current:   0,
			Speed:     0,
			Remaining: fileInfo.Size(),
		}
	}

	if err := s.SendMessage(conn, "file_data", uuid, chunk); err != nil {
		return fmt.Errorf("发送文件数据失败: %v", err)
	}

	// 发送完成后更新进度为100%
	if progress != nil {
		progress <- interfaces.Progress{
			Total:     fileInfo.Size(),
			Current:   fileInfo.Size(),
			Speed:     float64(fileInfo.Size()),
			Remaining: 0,
		}
	}

	return nil
}

// ReceiveFile 接收文件
func (s *MessageSender) ReceiveFile(conn net.Conn, destDir string, progress chan<- interfaces.Progress) error {
	// 1. 接收文件信息
	msg, err := s.ReceiveMessage(conn)
	if err != nil {
		return fmt.Errorf("接收文件信息失败: %v", err)
	}

	if msg.Type != "file" {
		return fmt.Errorf("收到意外的消息类型: %s", msg.Type)
	}

	var fileInfo struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
		MD5  string `json:"md5"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(msg.Payload, &fileInfo); err != nil {
		return fmt.Errorf("解析文件信息失败: %v", err)
	}

	s.logger.Debug("接收文件信息", interfaces.Fields{
		"name": fileInfo.Name,
		"path": fileInfo.Path,
		"size": s.FormatFileSize(fileInfo.Size),
	})

	// 开始接收前更新进度为0%
	if progress != nil {
		progress <- interfaces.Progress{
			Total:     fileInfo.Size,
			Current:   0,
			Speed:     0,
			Remaining: fileInfo.Size,
		}
	}

	// 2. 接收文件内容
	msg, err = s.ReceiveMessage(conn)
	if err != nil {
		return fmt.Errorf("接收文件内容失败: %v", err)
	}

	if msg.Type != "file_data" {
		return fmt.Errorf("收到意外的消息类型: %s", msg.Type)
	}

	var chunk struct {
		Data []byte `json:"data"`
	}
	if err := json.Unmarshal(msg.Payload, &chunk); err != nil {
		return fmt.Errorf("解析文件内容失败: %v", err)
	}

	// 3. 写入文件
	// 确保目标目录存在
	targetDir := filepath.Dir(destDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	s.logger.Debug("写入文件", interfaces.Fields{
		"dest_dir":   destDir,
		"target_dir": targetDir,
		"file_name":  fileInfo.Name,
		"file_path":  fileInfo.Path,
	})

	if err := os.WriteFile(destDir, chunk.Data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	// 接收完成后更新进度为100%
	if progress != nil {
		progress <- interfaces.Progress{
			Total:     fileInfo.Size,
			Current:   fileInfo.Size,
			Speed:     float64(fileInfo.Size),
			Remaining: 0,
		}
	}

	s.logger.Debug("文件接收完成", interfaces.Fields{
		"name":     fileInfo.Name,
		"path":     fileInfo.Path,
		"size":     s.FormatFileSize(fileInfo.Size),
		"received": s.FormatFileSize(int64(len(chunk.Data))),
	})

	return nil
}

// FormatPayload 格式化消息内容预览
func (s *MessageSender) FormatPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	previewLen := 500
	if len(payload) < previewLen {
		previewLen = len(payload)
	}
	return string(payload[:previewLen])
}

// formatFileSize 将字节大小转换为易读格式
func (s *MessageSender) FormatFileSize(size int64) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2fGB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2fMB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2fKB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%dB", size)
	}
}
