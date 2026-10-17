<p align="center">
  <img src="https://opencode.ai/favicon.svg" width="64" height="64" alt="">
</p>

<p align="center">
  <samp>
    <b>Go</b> 宿主 &nbsp;·&nbsp; <b>Lua</b> 插件 &nbsp;·&nbsp; <b>内置工具集</b>
  </samp>
</p>

<p align="center">
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26"></a>
  <a href="https://github.com/botuniverse/onebot-11"><img src="https://img.shields.io/badge/OneBot-v11-000?logo=data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48Y2lyY2xlIGN4PSIxMiIgY3k9IjEyIiByPSIxMiIgZmlsbD0iIzU4QTZGRiIvPjwvc3ZnPg==" alt="OneBot v11"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-green" alt="MIT"></a>
</p>

<h3 align="center">Quill</h3>
<p align="center">功能齐全的 QQ 机器人框架，用 Lua 写业务，网络与存储能力开箱即用</p>

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
 │   ┌──────────┐  ┌──────────────┐  ┌──────────────┐  │
 │   │ Lua 插件  │  │  内置工具集   │  │  内嵌 Wiki    │  │
 │   │ gopher-  │  │ HTTP / 下载   │  │  :3081        │  │
 │   │  lua     │  │ 解压 / 压缩   │  │ 普通/专业双模式│  │
 │   └──────────┘  └──────────────┘  └──────────────┘  │
 │                                                      │
 │   • 彩色终端日志  • JSON 持久化  • 控制台指令          │
 └─────────────────────────────────────────────────────┘
```

内置工具集（HTTP 请求、多线程下载、压缩包解压）直接编译进宿主，插件调用 `util.*` 即可，**不再需要 WASM 或任何外部模块目录**。

### 功能一览

| 分类 | 功能 |
|------|------|
| 消息 | 纯文本、图片、语音、视频、AT、回复、合并转发、自定义消息段 |
| 事件 | 群消息、私聊、入群、退群、禁言、群文件上传、管理变更、撤回、戳一戳、好友添加、精华消息、好友/群请求、心跳 |
| 插件 | 嵌套目录、`init.lua` 入口、`include()` 模块化、多事件处理、控制台 `/reload` 热重载 |
| 网络 | `util.http_get/post/request`、多线程分段下载、zip/tar/gz/bz2 解压，全部内置 |
| 存储 | `bot.db` 键值持久化、`bot.file` 文件 I/O、`util.MemFS` 内存键值 |
| 连接 | 正向 WS 客户端 / 反向 WS 服务端 + HTTP 事件上报 |
| 体验 | 彩色终端日志、`NO_COLOR` 降级、内嵌 Wiki（普通/专业双模式） |

### 快速开始

**1. 配置 `.env`**

```env
ONEBOT_BASE_URL=http://localhost:3007
ONEBOT_WS_URL=http://localhost:12006
ONEBOT_TOKEN=your_token_here
ONEBOT_WS_MODE=forward
```

**2. 写一个插件** — `plugins/hello.lua`

```lua
local Quill = {}

Quill.PluginInfo = {
    name = "hello",
    version = "1.0.0",
}

Quill.on_group_message = function(event)
    if event.raw_message == "你好" then
        bot.send_group_msg(event.group_id, {
            bot.at(event.user_id),
            bot.text(" 你也好呀~"),
        })
    end
end

return Quill
```

> `PluginInfo` 中的 `name` 和 `version` 是必填项，缺失的插件会被拒绝加载。

**3. 编译并运行**

```powershell
go build -o release\quill.exe .
cd release; .\quill.exe
```

### 高级玩法

**目录插件** — 大型插件用 `include()` 拆分模块：

```lua
-- plugins/admin/init.lua
local utils = include("utils")      -- 加载 admin/utils.lua
local cfg   = include("sub/config") -- 加载 admin/sub/config.lua

return {
    PluginInfo = { name = "admin", version = "1.0.0" },
    on_group_message = function(event)
        if event.raw_message == "/info" then
            bot.send_group_msg(event.group_id, utils.stats())
        end
    end,
}
```

**内置网络能力** — 无需引入任何模块：

```lua
-- 同步 HTTP（返回 JSON 字符串）
local raw  = util.http_get("https://api.example.com/data")
local resp = util.JSONToTable(raw)   -- { ok = true, status = 200, body = "...", headers = {...} }

-- 带请求头 / body / 超时
util.http_request(util.TableToJSON({
    method = "POST",
    url    = "https://api.example.com/submit",
    headers = { ["Content-Type"] = "application/json" },
    body   = "{}",
    timeout = 10,
}))

-- 异步多线程下载（立即返回任务 ID）
local task = util.download_file("https://example.com/big.zip", "downloads/big.zip", 8)
local st = util.download_poll(task)  -- { done, percent, downloaded, total, error }
util.extract_archive("downloads/big.zip", "downloads/out")  -- zip/tar/gz/bz2
```

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
 ├── main.go                 # 入口：装配各组件、控制台指令
 ├── internal/
 │   ├── onebot/             # OneBot v11 客户端 + WS/HTTP 事件接收
 │   ├── plugin/             # Lua 引擎、bot/util API、事件分发、定时器
 │   ├── module/             # 内置 HTTP / 下载 / 解压实现
 │   ├── storage/            # JSON 持久化存储
 │   ├── config/             # .env 加载
 │   ├── docserver/          # 内嵌 Wiki（docs/** 由 go:embed 打包）
 │   ├── command/            # 控制台指令（help / reload / exit）
 │   └── logc/               # 彩色终端日志
 ├── plugins/                # Lua 插件
 │   ├── ping.lua
 │   ├── poke.lua
 │   ├── mapdl.lua
 │   └── demo/
 │       ├── init.lua        # 目录插件入口
 │       ├── utils.lua       # 工具模块
 │       ├── storage.lua     # 签到示例
 │       └── sub/pic.lua     # 发图示例
 └── release/                # 编译产物与运行时配置（不纳入版本控制）
```

### 实现进度

| 项目 | 状态 |
|------|------|
| OneBot v11 HTTP API | 34 个 action（消息、群管理、查询、请求处理、凭证） |
| 事件分发 | 18 个 `on_*` 处理函数，含 `on_notice` / `on_request` 泛型回退 |
| 消息段 | text / image / record / video / at / reply / forward / node / 自定义 |
| Lua `bot` API | 45 个成员 |
| `util` 工具集 | 37 个函数 + `MemFS` |
| 内嵌 Wiki | `internal/docserver/docs/` 下的 Markdown 自动渲染 |

### 注意事项

- 插件运行在宿主进程内，**不是安全沙箱**。Lua 标准库 `io` / `os` 可用，请只安装可信插件。
- 事件分发是串行的：插件内的 `util.Sleep` 与同步 HTTP 请求会阻塞后续事件的处理。
- `util.After` 定时器只在有事件到达时被驱动，不保证精确定时。

### 关于

本作品使用 [OpenCode](https://opencode.ai) 辅助开发。

MIT &copy; 2026 [Shinkarna](https://github.com/Shinkarna)
