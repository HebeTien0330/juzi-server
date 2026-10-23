# juzi-server

通用游戏服务端框架。业务玩法由具体游戏自己加，公共能力是接入、会话、路由、存盘和跨服调用。

同一份二进制按 NodeId 拉起不同进程。进程之间用 [origin](https://github.com/duanhf2012/origin) RPC 组网。协议与 `juzi-clinet` 的 `proto/pb` 对齐。

## 技术栈

| 项 | 说明 |
| --- | --- |
| 语言 | Go 1.21 |
| 引擎 | origin v2，本地目录 `framework/` |
| 协议 | Protobuf 3，生成代码在 `pb/` |
| 网关 | WebSocket `/connect/`，HTTP `/version/` |
| 存储 | MySQL（身份与模块存盘）+ Redis |

## 进程

| NodeId | 名称 | 职责 | 监听 |
| --- | --- | --- | --- |
| `1001` | gate | HTTP 版本查询、WebSocket、会话与转发 | RPC `8011`，HTTP `9402`，WS `9031` |
| `2001` | center | 跨 Game 的内部 RPC | `8021` |
| `3001` | game | 在线对象、协议路由、模块存盘 | `8031` |
| `4001` / `5001` | battle / user | 空 Service，按需启动 | `8041` / `8051` |

服务发现的 Master 是 Gate。Gate 要先于其他进程启动。

```
客户端
  │  GET /version/          校验 version，取得 WS 地址
  │  WebSocket /connect/    [2 字节 MsgType 大端] + Protobuf
  ▼
Gate
  │  心跳、重连、断线宽限、推送
  │  其余 TransferMessageReq 转到绑定的 Game
  ▼
Game                        按 msgName 调 Handler
  │
  ▼
Center                      ServiceName.MethodName
```

## 目录

```
├── main.go
├── app/            gate、game、center、battle、user
├── network/        WebSocket 与 HTTP 入口
├── core/           box、配置、数据库、路由、同步、在线对象
├── persistence/    Persistable、账号表、模块表
├── module/         游戏协议注册点，默认空
├── proto/pb        与客户端共用的框架协议，common / public / internal
├── proto/game      游戏协议，默认空
├── framework/      origin
└── config/
```

## 框架协议

报文是 `[2 字节 MsgType 大端] + Protobuf`。

| MsgType | 消息 |
| --- | --- |
| `MsgReq` | `TransferMessageReq` |
| `MsgRes` | `TransferMessageRsp` |
| `MsgNotify` | `NotifyMessageInf` |

Gate 自己处理心跳和重连。`seqId = 1` 只留给 `C2SHeartbeat`。

`C2SLogout`、`C2SReconnect`、内部的 `C2SDisconnect` 由 Game 的会话处理。登录协议不在框架里。游戏登记为登录前可转发的消息，成功后调用 `CompleteLogin`。

断线后 Gate 和 Game 保留约 60 秒内存会话。客户端用 `sessionToken` 发 `C2SReconnect`。超时后框架登出并清会话。

增量同步是 `Remote`：点路径到 JSON，删除值为 `"null"`。游戏对在线对象调用 `Publish(root, value)`，响应里若有 `remote` 字段会自动带上差量。

## 游戏怎么接

在 `module` 的 `init` 里注册协议，并把登录消息标成登录前可转发：

```go
func init() {
    protocol.AllowBeforeLogin("C2SLogin")
    gate.SetAccountResolver(func(msgName string, payload []byte) string {
        if msgName != "C2SLogin" {
            return ""
        }
        req := &pb.C2SLogin{}
        if err := proto.Unmarshal(payload, req); err != nil {
            return ""
        }
        return req.GetAccount()
    })
    module.Register(func(router *protocol.Router) {
        router.Register("C2SLogin", &pb.C2SLogin{}, onLogin)
    })
}
```

登录成功时，从已启动的 Game 上完成绑定：

```go
g := node.GetService("Game").(*game.Game)
actor, token, err := g.CompleteLogin(clientId, userId, account)
actor.Publish("player", playerView)
```

`S2CLogin` 的 4–6 号字段留给游戏自己的登录态，与客户端 `session.proto` 的预留一致。`sessionToken` 放在游戏回包里返回给客户端。

模块存盘实现 `persistence.Persistable`，用 `actor.SetModuleFactory` 挂上。跨服服务用 `center.Register`。切服用 `Game.SwitchServer`。内存状态通过 `SetSnapshotHooks` 带过去，已落库的数据由目标进程重新加载。

## 本地启动

准备 MySQL 和 Redis，库名与 `config/config.json` 一致：

```sql
CREATE DATABASE IF NOT EXISTS gamedata CHARACTER SET utf8mb4;
```

```bash
bash proto/genPb/genAll.sh
go build -o bin/juzi-server .
./bin/juzi-server -start nodeid=1001
./bin/juzi-server -start nodeid=2001
./bin/juzi-server -start nodeid=3001
```

客户端 `gateAddr` 指向 `http://127.0.0.1:9402/version/`，`version` 为 `0.0.1`。WebSocket 路径是返回地址上的 `/connect/`。
