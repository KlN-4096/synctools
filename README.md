# SyncTools

一个基于 Go 语言开发的文件同步工具，提供服务端和客户端，具有图形用户界面。

## 功能特点

- 服务器-客户端架构
- 图形用户界面（基于 lxn/walk）
- 文件同步功能
  - 支持多文件夹同步
  - 支持文件夹重定向
  - 支持多种同步模式（mirror/push）
  - 支持版本控制
- 错误日志记录
- 崩溃恢复机制
- 调试模式

## 技术栈

- Go 语言 (1.21+)
- lxn/walk (GUI框架)
- lxn/win (Windows API)

## 系统要求

- 操作系统：Windows
- 架构：支持 32 位和 64 位

## 安装和使用

### 从源码编译

1. 克隆仓库
```bash
git clone https://github.com/YOUR_USERNAME/synctools.git
```

2. 进入项目目录
```bash
cd synctools
```

3. 编译服务端和客户端
```bash
go build ./cmd/server
go build ./cmd/client
```

### 使用预编译版本

直接运行 `server.exe`（服务端）或 `client.exe`（客户端）。

## 配置说明

### 服务端配置

- 同步目录：选择要同步的根目录
- 端口：设置服务器监听端口（默认 6666）
- 同步文件夹：可添加多个子文件夹进行同步
- 文件夹重定向：可设置服务端和客户端的文件夹映射关系
- 版本号：用于控制客户端同步行为

### 客户端配置

- 服务器地址：连接的服务器 IP 地址
- 端口：连接的服务器端口
- 同步目录：本地同步根目录

## 开发说明

### 项目结构

```
synctools/
├── cmd/                    # 主程序入口
│   ├── server/            # 服务端
│   └── client/            # 客户端
├── pkg/                    # 包目录
│   ├── server/            # 服务端逻辑
│   ├── handlers/          # 处理器
│   └── common/            # 公共代码
├── logs/                   # 日志目录
├── go.mod                 # Go 模块文件
└── app.manifest          # Windows 应用程序清单
```

### 主要功能

1. 文件同步
   - 支持增量同步
   - 文件哈希比对
   - 自动跳过重复文件

2. 版本控制
   - 版本不同时可选择删除多余文件
   - 版本相同时保留客户端文件

3. 日志系统
   - 详细的运行日志
   - 崩溃日志记录
   - 调试模式支持

## 许可证

本项目采用 GNU General Public License v3.0 许可证。详情请见 [LICENSE](LICENSE) 文件。 