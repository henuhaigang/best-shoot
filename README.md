# Tunnel Project

一个轻量级的 TCP 反向隧道系统，支持多路复用和自动重连。

## 功能特性

- **TCP 反向隧道** — 将内网服务暴露到公网，无需修改防火墙规则
- **多路复用** — 单个控制连接上承载多个隧道，每个隧道支持多个并发连接
- **自动重连** — 客户端断线后自动重连，指数退避（1s → 30s 上限）
- **心跳检测** — 客户端定期发送心跳，服务端超时自动断开（默认 90s）
- **结构化日志** — 支持 DEBUG/INFO/WARN/ERROR 四个级别，带字段标记
- **配置灵活** — 支持 JSON 配置文件和环境变量，开箱即用的默认值

## 架构

```
┌──────────────────────────────────────────────────────────┐
│                        Server                            │
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │ClientManager│  │TunnelManager │  │  ProxyListener  │  │
│  │  客户端管理  │  │  隧道管理     │  │  外部连接代理    │  │
│  └─────────────┘  └──────────────┘  └─────────────────┘  │
│         │                │                   │           │
│         └────────┬───────┘                   │           │
│            Session (控制连接)                  │           │
│                  │                           │           │
└──────────────────┼───────────────────────────┼───────────┘
                   │                           │
          ┌────────┴────────┐          ┌───────┴────────┐
          │   TCP 连接       │          │  外部用户连接   │
          └────────┬────────┘          └───────┬────────┘
                   │                           │
┌──────────────────┼───────────────────────────┼───────────┐
│                  │      Client               │           │
│            Session                    LocalConn          │
│         (控制连接)                  (本地服务连接)         │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

## 快速开始

### 编译

```bash
go build -o tunnel-server ./cmd/server/
go build -o tunnel-client ./cmd/client/
```

### 启动服务端

```bash
./tunnel-server
# 默认监听 :7700
```

### 启动客户端

```bash
./tunnel-client -id my-node -server your-server:7700
```

### 环境变量配置

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `TUNNEL_LISTEN` | 服务端监听地址 | `:7700` |
| `TUNNEL_SERVER` | 客户端连接的服务器地址 | `127.0.0.1:7700` |
| `TUNNEL_CLIENT_ID` | 客户端标识符 | — |
| `TUNNEL_HEARTBEAT_INTERVAL` | 心跳间隔 | `30s` |
| `TUNNEL_HEARTBEAT_TIMEOUT` | 心跳超时（服务端） | `90s` |
| `TUNNEL_RECONNECT_BASE` | 重连基础延迟 | `1s` |
| `TUNNEL_RECONNECT_MAX` | 重连最大延迟 | `30s` |
| `TUNNEL_LOG_LEVEL` | 日志级别 | `info` |

### JSON 配置文件

**服务端配置 (server.json):**
```json
{
  "listen_addr": ":7700",
  "heartbeat_timeout": "90s",
  "heartbeat_interval": "30s",
  "log_level": "info"
}
```

**客户端配置 (client.json):**
```json
{
  "server_addr": "your-server:7700",
  "client_id": "my-node",
  "reconnect_base": "1s",
  "reconnect_max": "30s",
  "heartbeat_interval": "30s",
  "log_level": "info"
}
```

## 项目结构

```
tunnel-project/
├── cmd/
│   ├── server/          # 服务端入口
│   └── client/          # 客户端入口
├── protocol/            # 二进制协议层
│   ├── message.go       # 消息类型与序列化
│   ├── encoder.go       # 编码器（带缓冲写入）
│   ├── decoder.go       # 解码器（带缓冲读取）
│   └── session.go       # 双向会话抽象
├── server/              # 服务端逻辑
│   ├── server.go        # 连接管理、消息路由
│   ├── session.go       # 服务端会话（含心跳追踪）
│   ├── client_manager.go# 客户端注册管理
│   └── tunnel_manager.go# 隧道生命周期管理
├── client/              # 客户端逻辑
│   └── client.go        # 转发、重连、心跳
├── tunnel/              # 隧道与代理
│   ├── tunnel.go        # 隧道绑定与消息分发
│   └── proxy_listener.go# 外部连接代理
├── config/              # 配置加载
│   └── config.go        # JSON + 环境变量
└── logger/              # 结构化日志
    └── logger.go        # 分级日志器
```

## 协议格式

每个消息使用 9 字节固定头 + 可变负载：

```
[Type: 1字节][TunnelID: 4字节][Length: 4字节][Payload: N字节]
```

| 类型 | 值 | 方向 | 说明 |
|------|----|------|------|
| Register | 0x01 | C→S | 客户端注册 |
| RegisterAck | 0x02 | S→C | 注册确认 |
| OpenTunnel | 0x03 | S→C | 请求打开隧道 |
| OpenTunnelAck | 0x04 | C→S | 隧道已打开 |
| Data | 0x05 | ↔ | 数据传输 |
| CloseTunnel | 0x06 | ↔ | 关闭隧道 |
| Heartbeat | 0x07 | ↔ | 心跳保活 |

## 测试

```bash
go test ./...
```

测试覆盖 2000+ 行，包括：
- 协议编解码往返测试
- 客户端注册与多隧道测试
- 数据转发端到端测试
- 断线重连测试
- 心跳超时测试
- 并发安全测试

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
