package message

import (
	"encoding/json"
	"fmt"
	"io"
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

	// 将payload转换为json.RawMessage
	var payloadJSON json.RawMessage
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("序列化payload失败: %v", err)
		}
		payloadJSON = json.RawMessage(data)
	}

	msg := &interfaces.Message{
		Type:    msgType,
		UUID:    uuid,
		Payload: payloadJSON,
	}

	s.logger.Debug("发送消息", interfaces.Fields{
		"type":    msgType,
		"uuid":    uuid,
		"payload": payload,
	})

	encoder := json.NewEncoder(conn)
	return encoder.Encode(msg)
}

// SendMessageToAddr 发送消息到指定地址
func (s *MessageSender) SendMessageToAddr(addr string, msgType string, uuid string, payload interface{}) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("连接到 %s 失败: %v", addr, err)
	}
	defer conn.Close()

	return s.SendMessage(conn, msgType, uuid, payload)
}

// ReceiveMessage 从连接接收消息
func (s *MessageSender) ReceiveMessage(conn net.Conn) (*interfaces.Message, error) {
	if conn == nil {
		return nil, fmt.Errorf("连接为空")
	}

	var msg interfaces.Message
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&msg); err != nil {
		return nil, fmt.Errorf("接收消息失败: %v", err)
	}

	s.logger.Debug("接收消息", interfaces.Fields{
		"type":    msg.Type,
		"uuid":    msg.UUID,
		"payload": string(msg.Payload),
	})

	return &msg, nil
}

// SendInitMessage 发送初始化消息
func (s *MessageSender) SendInitMessage(conn net.Conn, uuid string) error {
	return s.SendMessage(conn, "init", uuid, nil)
}

// SendSyncRequest 发送同步请求
func (s *MessageSender) SendSyncRequest(conn net.Conn, uuid string, request *interfaces.SyncRequest) error {
	return s.SendMessage(conn, "sync_request", uuid, request)
}

// SendSyncResponse 发送同步响应
func (s *MessageSender) SendSyncResponse(conn net.Conn, uuid string, success bool, message string) error {
	response := struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}{
		Success: success,
		Message: message,
	}
	return s.SendMessage(conn, "sync_response", uuid, response)
}

// SendHeartbeat 发送心跳包
func (s *MessageSender) SendHeartbeat(conn net.Conn, uuid string) error {
	return s.SendMessage(conn, "heartbeat", uuid, nil)
}

// SendFile 发送文件
func (s *MessageSender) SendFile(conn net.Conn, uuid string, path string, progress chan<- interfaces.Progress) error {
	// 1. 发送文件信息
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %v", err)
	}

	info := interfaces.FileInfo{
		Path: filepath.Base(path),
		Size: fileInfo.Size(),
	}

	if err := s.SendMessage(conn, "file_info", uuid, info); err != nil {
		return fmt.Errorf("发送文件信息失败: %v", err)
	}

	// 2. 发送文件内容
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	buffer := make([]byte, 32*1024)
	totalWritten := int64(0)
	startTime := time.Now()

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取文件失败: %v", err)
		}

		// 发送文件块
		chunk := struct {
			Data []byte `json:"data"`
		}{
			Data: buffer[:n],
		}
		if err := s.SendMessage(conn, "file_data", uuid, chunk); err != nil {
			return fmt.Errorf("发送文件数据失败: %v", err)
		}

		totalWritten += int64(n)

		if progress != nil {
			elapsed := time.Since(startTime).Seconds()
			speed := float64(totalWritten) / elapsed
			remaining := int64((float64(fileInfo.Size()-totalWritten) / speed))

			progress <- interfaces.Progress{
				Total:     fileInfo.Size(),
				Current:   totalWritten,
				Speed:     speed,
				Remaining: remaining,
				FileName:  filepath.Base(path),
				Status:    "sending",
			}
		}
	}

	// 3. 发送文件结束标记
	return s.SendMessage(conn, "file_end", uuid, nil)
}

// ReceiveFile 接收文件
func (s *MessageSender) ReceiveFile(conn net.Conn, destDir string, progress chan<- interfaces.Progress) error {
	// 1. 接收文件信息
	msg, err := s.ReceiveMessage(conn)
	if err != nil {
		return fmt.Errorf("接收文件信息失败: %v", err)
	}
	if msg.Type != "file_info" {
		return fmt.Errorf("收到意外的消息类型: %s", msg.Type)
	}

	var fileInfo interfaces.FileInfo
	if err := json.Unmarshal(msg.Payload, &fileInfo); err != nil {
		return fmt.Errorf("解析文件信息失败: %v", err)
	}

	// 2. 创建目标文件
	destPath := filepath.Join(destDir, fileInfo.Path)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 3. 接收文件内容
	totalReceived := int64(0)
	startTime := time.Now()

	for {
		msg, err := s.ReceiveMessage(conn)
		if err != nil {
			return fmt.Errorf("接收文件数据失败: %v", err)
		}

		switch msg.Type {
		case "file_data":
			var chunk struct {
				Data []byte `json:"data"`
			}
			if err := json.Unmarshal(msg.Payload, &chunk); err != nil {
				return fmt.Errorf("解析文件数据失败: %v", err)
			}

			written, err := file.Write(chunk.Data)
			if err != nil {
				return fmt.Errorf("写入文件失败: %v", err)
			}

			totalReceived += int64(written)

			if progress != nil {
				elapsed := time.Since(startTime).Seconds()
				speed := float64(totalReceived) / elapsed

				progress <- interfaces.Progress{
					Total:     fileInfo.Size,
					Current:   totalReceived,
					Speed:     speed,
					Remaining: int64((float64(fileInfo.Size-totalReceived) / speed)),
					FileName:  fileInfo.Path,
					Status:    "receiving",
				}
			}

		case "file_end":
			return nil

		default:
			return fmt.Errorf("收到意外的消息类型: %s", msg.Type)
		}
	}
}
