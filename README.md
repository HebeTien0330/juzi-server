# juzi-server

基于 Go 和 Origin v2 的分布式游戏服务端框架骨架，提供网关接入、连接会话、协议路由、在线对象、数据存盘和跨服 RPC 等通用能力。游戏协议和玩法由具体项目接入；本仓库默认不包含完整的游戏登录与玩法实现。

Origin 是底层 Node/Service 运行与集群通信引擎。本仓库通过 Go `replace` 指令使用 `framework/` 中的 Origin v2 源码，而不是从远程模块下载该依赖。感谢 [Origin](https://github.com/duanhf2012/origin) 项目。

## 目录

- [功能概览](#功能概览)
- [架构与进程](#架构与进程)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [接入游戏逻辑](#接入游戏逻辑)
- [协议说明](#协议说明)
- [目录结构](#目录结构)
- [Origin 与许可证](#origin-与许可证)

## 功能概览

- 基于 Origin v2 的多 Node 服务进程和服务发现。
- WebSocket 网关、HTTP 版本查询、连接心跳、重连与断线宽限。
- 登录后将客户端连接绑定到 Game，并将游戏协议转发到对应服务。
- Protobuf 消息路由、在线 Actor、增量状态同步。
- MySQL 数据持久化、Redis 客户端初始化、跨服 RPC 和切服接口。

这是可扩展的服务端基础框架，不是开箱即用的完整游戏服务器：游戏登录、账号校验、玩法协议和业务数据模型需要在业务代码中实现。`module/`、`proto/game/` 默认不含具体玩法；`battle`、`user` 服务目前是预留扩展点。

## 架构与进程

同一个二进制根据 NodeId 启动不同进程。服务发现的 Master 是 Gate，启动时应先启动 Gate。

| NodeId | 服务 | 职责 | 监听地址（默认配置） |
| --- | --- | --- | --- |
| `1001` | Gate | HTTP 版本查询、WebSocket 接入、会话管理和消息转发 | RPC `8011`、HTTP `9402`、WS `9031` |
| `2001` | Center | Game 间的内部 RPC | RPC `8021` |
| `3001` | Game | 在线对象、协议路由和模块存盘 | RPC `8031` |
| `4001` | Battle | 预留服务 | RPC `8041` |
| `5001` | User | 预留服务 | RPC `8051` |

```text
客户端 -- GET /version/ --> Gate -- 返回 WebSocket 地址
客户端 -- WebSocket /connect/ --> Gate -- 登录后转发 --> Game
                                               Game -- 跨服 RPC --> Center
```

以上监听地址来自仓库默认配置，仅适用于本机开发；部署时请按实际网络拓扑修改配置。

## 技术栈

| 项目 | 说明 |
| --- | --- |
| 语言 | Go 1.21+ |
| 服务端引擎 | Origin v2，源码位于 `framework/` |
| 网络与协议 | WebSocket、HTTP、Protocol Buffers |
| 数据存储 | MySQL、Redis |

## 快速开始

### 环境准备

- Go 1.21 或更高版本。
- MySQL 和 Redis。
- 仅在需要重新生成 Protobuf 代码时：`protoc` 和 `protoc-gen-go`。

### 配置

默认配置位于 `config/config.json` 和 `config/cluster/`。启动前按需修改数据库、Redis、监听地址及集群发现配置。示例使用本机地址和开发配置，请勿直接作为生产环境配置，也不要将真实密码提交到仓库。

创建默认数据库（实际库名以配置为准）：

```sql
CREATE DATABASE IF NOT EXISTS gamedata CHARACTER SET utf8mb4;
```

### 编译并启动

在仓库根目录执行：

```bash
go build -o bin/juzi-server .
./bin/juzi-server -start nodeid=1001
./bin/juzi-server -start nodeid=2001
./bin/juzi-server -start nodeid=3001
```

每条启动命令在独立终端运行。Gate 必须先于其他进程启动；Battle 和 User 是可选的预留服务。程序启动时会连接 MySQL、执行表结构迁移并初始化 Redis，因此这些服务必须可用。

客户端通过 `http://127.0.0.1:9402/version/` 获取版本与 WebSocket 地址；默认版本为 `0.0.1`，WebSocket 路径为 `/connect/`。客户端协议定义需与服务端 `pb/` 中的框架协议保持一致。

### 重新生成 Protobuf

如果修改了 `proto/pb/` 下的 `.proto` 文件，可在仓库根目录执行：

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
bash proto/genPb/genAll.sh
```

还需确保 `protoc` 已安装并可从 `PATH` 调用。生成脚本会写入仓库中的生成代码。

## 接入游戏逻辑

在业务包的 `init` 中注册协议和处理函数。需要在登录前转发的消息，应显式加入允许列表；登录成功后，由 Game 创建在线对象并绑定客户端会话。

```go
func init() {
    protocol.AllowBeforeLogin("C2SLogin")
    module.Register(func(router *protocol.Router) {
        router.Register("C2SLogin", &pb.C2SLogin{}, onLogin)
    })
}
```

登录处理器验证账号后，调用 Game 的 `CompleteLogin` 完成在线对象创建和 Gate 会话绑定：

```go
actor, token, err := gameService.CompleteLogin(clientId, userId, account)
if err != nil {
    return err
}
actor.Publish("player", playerView)
```

`onLogin`、`gameService` 和 `playerView` 需由游戏业务实现。将生成的 `sessionToken` 返回客户端，用于重连。模块存盘实现 `persistence.Persistable`，并通过 `actor.SetModuleFactory` 注册；跨服服务通过 `center.Register` 注册，切服使用 `Game.SwitchServer`。

## 协议说明

客户端消息帧格式为 `[2 字节 MsgType 大端序] + Protobuf`。

| MsgType | 消息 |
| --- | --- |
| `MsgReq` | `TransferMessageReq` |
| `MsgRes` | `TransferMessageRsp` |
| `MsgNotify` | `NotifyMessageInf` |

Gate 处理心跳和重连；`seqId = 1` 保留给 `C2SHeartbeat`。`C2SLogout`、`C2SReconnect` 和内部 `C2SDisconnect` 由 Game 的会话逻辑处理。登录协议由游戏实现，并通过 `protocol.AllowBeforeLogin` 配置为登录前可转发消息。

断线后 Gate 和 Game 会在内存中保留约 60 秒的会话路由。客户端可使用 `sessionToken` 发送 `C2SReconnect`；超过宽限时间后会话将被清理并登出。

增量同步通过 `Remote` 表示：路径映射为 JSON，删除值表示为字符串 `"null"`。游戏对在线对象调用 `Publish(root, value)`；响应中包含 `remote` 字段时会自动携带差量。

## 目录结构

```text
├── app/            Gate、Game、Center、Battle、User 服务
├── core/           Actor、配置、数据库、协议路由、同步等基础组件
├── framework/      Origin v2 引擎源码及其许可证
├── module/         游戏协议注册入口
├── network/        WebSocket 与 HTTP 入口
├── persistence/    持久化接口与模型
├── proto/          Protobuf 源文件和生成脚本
├── pb/             生成的 Protobuf Go 代码
├── config/         服务、集群和存储配置
└── main.go         程序入口
```

## Origin 与许可证

本仓库在 `go.mod` 中将 `github.com/duanhf2012/origin/v2` 替换为本地 `framework/` 目录，因此构建依赖仓库内的 Origin 源码。Origin 项目地址：[duanhf2012/origin](https://github.com/duanhf2012/origin)。

`framework/LICENSE` 是随 Origin 源码提供的 Apache License 2.0 许可证，适用于其所覆盖的内容。它不自动决定本仓库其余代码的许可证。向 GitHub 公开发布前，请确认你有权发布仓库中的全部代码，并为本项目补充明确的根目录许可证文件；同时保留 Origin 源码中的版权和许可证声明。
