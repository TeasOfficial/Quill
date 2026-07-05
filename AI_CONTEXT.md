# AI Context — Quill Bot 插件 & 模块开发指南

> 供 AI 辅助工具阅读。包含完整的 Lua API、WASM 模块规范、事件系统和最佳实践。

---

## 基本信息

- 项目：Go 1.26.4 编译的单文件 QQ 机器人（OneBot v11）
- 插件：Lua 5.1（gopher-lua），放 `plugins/` 目录
- 模块：Go 编译为 WASM（wazero），放 `modules/<name>/`
- 协议：OneBot v11 HTTP API + WebSocket
- 运行：`cd release; .\quill.exe`
- 文档：内嵌 Wiki `http://localhost:3081`

---

## Lua 插件 API

### 插件结构

```lua
-- plugins/my_plugin.lua
return {
    on_group_message = function(event)
        if event.raw_message == "ping" then
            bot.send_group_msg(event.group_id, "pong")
        end
    end,
}
```

目录插件用 `init.lua` 入口，同目录其他 `.lua` 用 `include("filename")` 加载。

#### include() 详解

```lua
-- plugins/admin/init.lua
local utils = include("utils")         -- 自动补 .lua → 加载 admin/utils.lua
local sub   = include("sub/helper")    -- 子目录 → admin/sub/helper.lua

return {
    on_group_message = function(event)
        bot.send_group_msg(event.group_id, utils.stats())
    end,
}
```

- `include("name")` 自动补 `.lua` 后缀
- 路径相对于调用者所在目录
- 不能跨越到插件目录外（安全检查）
- 一次加载，多次调用返回缓存

#### 返回值约定

Lua API 统一约定（遵循 Lua 的 nil+err 惯例）：

```
单值（成功）
  bot.send_group_msg(gid, msg)  →  123456     （message_id）
  bot.db.get("key")             →  {a=1}      （或 nil）

双值（失败时）
  bot.file.read("missing")      →  nil, "open file: ..."
  util.AESDecrypt(bad, key)     →  nil, "invalid padding"
  util.JSONToTable("{bad")      →  nil, "unexpected EOF"

单值（失败时也返回单值，用特殊标记）
  util.IsJSON("bad")            →  false
  util.IsSafePath("../etc")     →  false
  bot.file.exists("missing")    →  false
  module("not_loaded")          →  nil
```

WASM 模块的返回值一律是 **JSON 字符串**，约定 `{"ok":true, ...}` 或 `{"ok":false,"error":"..."}`。

### bot API

#### 消息发送

```lua
bot.send_group_msg(group_id, "text")             -- 纯文本
bot.send_private_msg(user_id, "text")            -- 私聊
bot.send_msg("group", group_id, "text")          -- 通用
bot.send_msg("private", user_id, "text")

-- 消息段数组
bot.send_group_msg(gid, {
    bot.text("文字"),
    bot.image("https://..."),
    bot.at(user_id),
    bot.reply(message_id),
})

-- 高级
bot.forward("forward_msg_id")                    -- 引用合并转发
bot.node(user_id, "昵称", "内容或消息段数组")     -- 合并转发节点
bot.delete_msg(message_id)                        -- 撤回
bot.get_msg(message_id)                           -- 查消息，返回 JSON
bot.get_forward_msg("forward_id")                 -- 查合并转发
```

#### 消息段速查

```
bot.text("str")    bot.image("url")    bot.record("url")
bot.video("url")   bot.at(qq)          bot.reply(msg_id)
bot.segment("type", data_table)
```

### 事件处理

18 种事件，按类型独立分发。没有对应 handler 时回退到泛型的 `on_notice` / `on_request`。

```lua
on_group_message     -- 群消息
on_private_message   -- 私聊

on_group_increase    -- 入群        on_group_decrease  -- 退群
on_group_ban         -- 禁言/解禁   on_group_admin     -- 管理员变更
on_group_upload      -- 群文件上传   on_group_recall    -- 群消息撤回
on_friend_add        -- 好友添加     on_friend_recall   -- 好友撤回
on_notify            -- 戳一戳/运气王/荣誉
on_group_card        -- 群名片变更   on_essence         -- 精华消息
on_friend_request    -- 好友请求     on_group_request   -- 加群邀请
on_meta_event        -- 心跳/生命周期

-- 泛型回退
on_notice            -- 所有 notice 事件的兜底
on_request           -- 所有 request 事件的兜底
```

### 事件字段

```
event.post_type    -- "message" | "notice" | "request" | "meta_event"
event.sub_type     -- 子类型
event.user_id      event.group_id   event.message_id
event.raw_message  event.comment    event.flag
event.target_id    event.operator_id
event.sender = {user_id, nickname, card, role}
event.file = {id, name, size, busid}        -- 仅 upload 事件
```

### util 工具集

全局可用，无需引入。

```
JSONToTable(str)           → table        TableToJSON(tbl)            → string
Base64Encode(str)          → string       Base64Decode(str)           → string
MD5(str)                   → hex          SHA1(str) SHA256(str) SHA512(str)
CRC32(str)                 → hex          UUID()                      → "xxxxxxxx-..."
URLEncode(str)             → string       URLDecode(str)              → string
AESEncrypt(plain, key)     → base64       AESDecrypt(cipher, key)     → string
IsUTF8(str)    IsJSON(str)    IsBase64(str)    IsSafePath(str)
IsValid(v)                  → bool        GetEnv(key, default?)
Sleep(seconds)              -- 阻塞       After(seconds, fn)          -- 非阻塞 → id
CancelTimer(id)             -- 取消定时器
PrintTable(tbl)             → string
```

> AES 的 key **不硬编码**在源码中，用 `util.GetEnv("MY_KEY")` 从 .env 读。  
> `GetEnv` 禁止读 `ONEBOT_*` 前缀的变量。  
> `IsSafePath` 已在内部检查 `..` 和符号链接。

### 持久存储

```lua
bot.db.set("key", value)      -- 保存，value 支持嵌套 table
bot.db.get("key")             -- 读取，不存在返回 nil
bot.db.del("key")             -- 删除

-- 示例
local data = bot.db.get("signin/" .. event.user_id) or {count = 0}
data.count = data.count + 1
bot.db.set("signin/" .. event.user_id, data)
```

### 文件 I/O

```lua
bot.file.read("subdir/file.txt")   → string         -- 路径相对 data/
bot.file.write("path", content)    → true
bot.file.exists("path")            → bool
bot.file.delete("path")            → true
bot.file.list("dir")               → {"a.txt", "b.json"}
```

`..` 被拒绝，符号链接会额外解析真实路径再校验。  
`ONEBOT_FILE_UNSAFE=true` 解除沙箱，可读写任意路径（⚠️ 高危）。

### 调用 WASM 模块

```lua
local http = module("http")
if http then
    local resp = http.get("https://api.example.com/data")   -- → JSON
end

local dl = module("download")
dl.dl('{"url":"...","path":"downloads/file.bin","threads":4}')

local ex = module("extract")
ex.extract('{"file":"downloads/file.zip","dest":"downloads/extracted"}')
```

`module("name")` 查找 `modules/<name>/module.json` 中声明的模块名。未加载返回 `nil`。

### 延迟执行

```lua
util.Sleep(3)                           -- 阻塞 3 秒，期间 bot 无响应
local id = util.After(3, function()     -- 非阻塞，返回定时器 ID
    bot.delete_msg(placeholder)
end)
util.CancelTimer(id)                    -- 取消
```

---

## WASM 模块开发

### 目录结构

```
modules/my_mod/
├── module.json      -- {name, description, version, author}
├── main.go          -- Go 源码
├── README.md        -- 自动注册到 Wiki
└── my_mod.wasm      -- 自动编译生成
```

### main.go 模板

```go
package main

func main() {}

//go:wasmexport malloc                   // 必须：Host 调用以分配内存
func malloc(size uint32) *byte {
    buf := make([]byte, size)
    return &buf[0]
}

//go:wasmexport my_func                   // 导出给 Lua 调用
func my_func(ptr uint32, len uint32) uint64 {
    input := readString(ptr, len)         // 从 WASM 内存读参数
    result := process(input)
    return writeString(result)            // 写回结果
}
// readString / writeString 用 unsafe.Pointer 在 WASM 内存内读写
```

### 输入输出格式

输入：任意字符串（单参数直接传递，多参数以空格拼接，table 以 JSON 传递）  
输出：JSON 字符串或纯文本

### 调用 Host API

通过 `//go:wasmimport quill <func>` 调用宿主能力：

```
http_request(ptr, len) uint64    → JSON    -- HTTP 请求
http_download(ptr, len) uint64   → JSON    -- 多线程下载
archive_extract(ptr, len) uint64 → JSON    -- 解压
```

输入为 JSON string，输出为带 `{"ok":true/false, ...}` 的 JSON。

### WASM 内存协议

Host 与 WASM 模块通过**线性内存**传递数据，函数签名约定：

```
// 导出函数签名
//go:wasmexport my_func
func my_func(inputPtr uint32, inputLen uint32) uint64

// 返回值编码
packed := resultPtr << 32 | resultLen
```

Host 调模块时：
1. 调用模块的 `malloc(size)` 在 WASM 内存分配空间 → 得 `ptr`
2. 通过 `mem.Write(ptr, data)` 写入参数
3. 调用 `fn(ptr, len)` 执行业务逻辑
4. 返回值是 `packed = (resultPtr << 32) | resultLen`
5. 通过 `mem.Read(resultPtr, resultLen)` 读取结果

模块调 Host 时互为镜像。你的模块导出函数的参数即 Host 传入的 `(ptr, len)`。

**WASM 模块必须导出的函数：**

```
malloc(size uint32) *byte      -- ① 内存分配（Host 调用）
free(ptr uint32)               -- ② 内存释放（Host 可选调用）
具体业务函数                      -- ③ 每个导出一个功能
```

### 完整 WASM 模板（含不安全指针辅助）

```go
package main

import (
    "encoding/json"
    "unsafe"
)

func main() {}

//go:wasmexport malloc
func malloc(size uint32) *byte {
    buf := make([]byte, size)
    return &buf[0]
}

//go:wasmexport free
func free(ptr uint32) {
    // wasip1 下自动 GC，占位即可
}

//go:wasmimport quill http_request
func hostHTTPRequest(reqPtr uint32, reqLen uint32) uint64

//go:wasmexport my_func
func my_func(ptr uint32, len uint32) uint64 {
    input := readString(ptr, len)
    // 处理逻辑
    return writeString(result)
}

func readString(ptr, len uint32) string {
    p := (*byte)(unsafe.Pointer(uintptr(ptr)))
    return string(unsafe.Slice(p, len))
}

func writeString(s string) uint64 {
    b := []byte(s)
    p := malloc(uint32(len(b)))
    copy(unsafe.Slice(p, uint32(len(b))), b)
    return uint64(uintptr(unsafe.Pointer(p)))<<32 | uint64(len(b))
}
```

### 编译

保存 `main.go` 后 1.5 秒自动触发：
```
GOOS=wasip1 GOARCH=wasm go build -o name.wasm ./main.go
```

编译成功自动热载，无需重启 bot。

### module.json 示例

```json
{
    "name": "http",
    "description": "HTTP 请求模块",
    "version": "1.0.0",
    "author": "作者名"
}
```

### 模块文档

模块目录下的 `README.md` 自动注册到 Wiki 侧边栏"模块文档"。其他 `.md` 文件以子路径注册（`/module/<name>/<filename>`）。

---

## 错误处理

### Lua 层：双返回值模式

大多数 API 失败时返回 `(nil, error_message)`：

```lua
-- 推荐模式
local data, err = bot.file.read("config.json")
if not data then
    bot.send_group_msg(gid, "读取失败: " .. err)
    return
end

-- Lua 原生 pcall
local ok, result = pcall(function()
    return risky_operation()
end)
if not ok then
    -- result 是错误信息
end
```

### WASM 层：JSON 状态码

所有 Host API 返回 `{"ok": true/false, "error": "..."}` 格式。WASM 模块也应遵循此约定，Lua 侧通过解析 JSON 判断：

```lua
local raw = module("http").get("https://bad.url")
local resp = util.JSONToTable(raw)
if not resp.ok then
    bot.send_group_msg(gid, "请求失败: " .. resp.error)
end
```

### 超时保护

单次事件 dispatch（事件 handler + 所有定时器回调）上限 5 秒，超时自动中止，Lua 代码会收到 context cancelled 错误，不影响后续事件。

---

## 安全 & 限制

| 项 | 说明 |
|------|------|
| Lua 超时 | 单次事件分发上限 5 秒，超时自动中止 |
| 文件沙箱 | `bot.file` 限定在 `data/` 内，防 `..` 和符号链接 |
| 环境隔离 | `util.GetEnv` 不泄露 `ONEBOT_*` |
| Host 回调 | 4 个回调均 panic-safe，崩溃不连锁 |
| 日志截断 | 单条日志上限 500 字符 |

---

## 快速检查清单（AI 辅助使用）

创建插件时：
- [ ] 文件放在 `plugins/` 下，返回函数表
- [ ] 事件 handler 名从上面 18 种中选
- [ ] 用了 AES 密钥则通过 `util.GetEnv` 读取
- [ ] 不是路径相关操作则路径在 `data/` 内

创建 WASM 模块时：
- [ ] 目录包含 `main.go` + `module.json`
- [ ] `//go:wasmexport malloc` 必须存在
- [ ] 如需文件/网络，通过 `//go:wasmimport quill <func>` 调用 Host API
- [ ] 添加 `README.md` 自动生成文档
