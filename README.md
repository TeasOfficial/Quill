<p align="center">
  <img src="https://opencode.ai/favicon.svg" width="64" height="64" alt="">
</p>

<p align="center">
  <samp>
    <b>Go</b> 宿主 &nbsp;·&nbsp; <b>Lua</b> 插件 &nbsp;·&nbsp; <b>WASM</b> 模块
  </samp>
</p>

<p align="center">
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26"></a>
  <a href="https://github.com/botuniverse/onebot-11"><img src="https://img.shields.io/badge/OneBot-v11-000?logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48Y2lyY2xlIGN4PSIxMiIgY3k9IjEyIiByPSIxMiIgZmlsbD0iIzU4QTZGRiIvPjwvc3ZnPg==" alt="OneBot v11"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green" alt="MIT"></a>
</p>

<h3 align="center">Quill</h3>
<p align="center">功能齐全的 QQ 机器人框架，用 Lua 写业务，用 WASM 扩展能力</p>

---

### 架构

```
 ┌─ OneBot v11 ────────────────────────────────────────┐
 │  HTTP API + WebSocket 双向通信                        │
 └─────────────────────────────────────────────────────┘
                          │
                          ▼
 ┌─ Quill ─────────────────────────────────────────────┐
 │                                                      │
 │   Go 宿主 · 单文件编译                                 │
 │                                                      │
 │   ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
 │   │ Lua 插件  │  │ WASM 模块 │  │  内嵌 Wiki :3081  │  │
 │   │ gopher-  │  │  wazero   │  │  goldmark 渲染    │  │
 │   │  lua     │  │  热重载    │  │  普通/专业 双模式  │  │
 │   └──────────┘  └──────────┘  └──────────────────┘  │
 │                                                      │
 │   • 彩色终端日志  • fsnotify 自动编译  • JSON 持久化   │
 └─────────────────────────────────────────────────────┘
```

### 功能一览

| 分类 | 功能 |
|------|------|
| 消息 | 纯文本、图片、语音、视频、AT、回复、合并转发、自定义消息段 |
| 事件 | 群消息、私聊、入群、退群、禁言、群文件上传、管理变更、撤回、戳一戳、好友添加、精华消息、好友/群请求、心跳 |
| 插件 | 嵌套目录、`init.lua` 入口、`include()` 模块化、多事件处理 |
| 存储 | `bot.db` 键值存储、`bot.file` 文件 I/O（沙箱内） |
| 模块 | WASM 热重载、HTTP 请求、多线程下载、解压（zip/tar/gz/bz2）、模块文档自动注册 |
| 连接 | 正向 WS 客户端 / 反向 WS 服务端 + HTTP 事件上报 |
| 体验 | 彩色终端日志、`NO_COLOR` 降级、内嵌 Wiki（普通/专业双模式） |

### 快速开始

**1. 配置 `.env`**

```env
ONEBOT_BASE_URL=http://gmod.ltd:3007
ONEBOT_WS_URL=http://gmod.ltd:12006
ONEBOT_TOKEN=tkidawn.3345
ONEBOT_WS_MODE=forward
```

**2. 写一个插件** — `plugins/hello.lua`

```lua
return {
    on_group_message = function(event)
        if event.raw_message == "你好" then
            bot.send_group_msg(event.group_id, {
                bot.at(event.user_id),
                bot.text(" 你也好呀~"),
            })
        end
    end,
}
```

**3. 编译并运行**

```powershell
go build -o release\quill.exe .
cd release; .\quill.exe
```

### 高级玩法

**目录插件** — 大型插件用 `include()` 拆分模块：

```lua
-- plugins/admin/init.lua
local utils = include("utils")     -- 加载 admin/utils.lua
local cfg   = include("sub/config") -- 加载 admin/sub/config.lua

return {
    on_group_message = function(event)
        if event.raw_message == "/info" then
            bot.send_group_msg(event.group_id, utils.stats())
        end
    end,
}
```

**WASM 模块** — 用 Go 写高性能扩展：

```go
//go:wasmimport quill http_request
func hostHTTPRequest(reqPtr uint32, reqLen uint32) uint64

//go:wasmexport get
func get(urlPtr uint32, urlLen uint32) uint64 { ... }
```

Lua 调用：

```lua
local http = module("http")
local resp = http.get("https://api.example.com/data")
```

修改 `main.go` 保存后自动编译 → 热载，无需重启。

**持久存储** — 签到/积分/游戏进度：

```lua
local db = bot.db
local data = db.get("game/" .. event.user_id) or { score = 0 }
data.score = data.score + 10
db.set("game/" .. event.user_id, data)
```

### 目录结构

```
 Quill/
 ├── main.go                 # 入口
 ├── internal/
 │   ├── onebot/             # OneBot v11 协议
 │   ├── plugin/             # Lua 插件引擎
 │   ├── module/             # WASM 运行时 & 构建
 │   ├── storage/            # JSON 持久化存储
 │   ├── config/             # .env 加载
 │   ├── docserver/          # 内嵌 Wiki 服务
 │   └── logc/               # 彩色终端日志
 ├── plugins/                # Lua 插件
 │   ├── ping.lua
 │   └── demo/
 │       ├── init.lua        # 目录插件入口
 │       ├── utils.lua       # 工具模块
 │       ├── storage.lua     # 签到示例
 │       └── sub/pic.lua     # 发图示例
 └── modules/
     ├── http/               # HTTP 请求
     ├── download/           # 多线程下载
     └── extract/            # 解压 (zip/tar/gz/bz2)
```

### OneBot v11 实现进度

| 分类 | 已完成 | 未完成 |
|------|--------|--------|
| 消息发送 | `send_group_msg` `send_private_msg` `delete_msg` | `send_msg` `get_msg` `get_forward_msg` |
| 消息段 | text image record video at reply forward node | — |
| 事件分发 | 17 种事件按类型独立路由 | — |
| 群管理 | — | 踢人、禁言、全员禁言、设管理员、群名片、群名、退出 |
| 请求处理 | 接收 friend/group request | 同意/拒绝请求 |
| 查询 | — | 登录信息、好友列表、群列表、群成员、陌生人、状态、版本 |
| 其他 | — | 好友赞、凭证、语音图片获取、快速操作 |

### 关于

本作品使用 [OpenCode](https://opencode.ai) 辅助开发。

MIT &copy; 2026 [Shinkarna](https://github.com/Shinkarna)
