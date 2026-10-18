# AI Context — Quill Bot 插件开发指南

> 供 AI 辅助工具阅读。包含完整的 Lua API、内置网络能力、事件系统和已知限制。

---

## 基本信息

- 项目：Go 1.26 编译的单文件 QQ 机器人（OneBot v11）
- 插件：Lua 5.1（gopher-lua），放 `plugins/` 目录
- 内置能力：HTTP 请求、多线程下载、压缩包解压直接编译进宿主，插件调用 `util.*` 即可
- 协议：OneBot v11 HTTP API + WebSocket
- 运行：`cd release; .\quill.exe`
- 文档：内嵌 Wiki `http://localhost:3081`

---

## Lua 插件 API

### 插件结构

```lua
-- plugins/my_plugin.lua
local Quill = {}

Quill.PluginInfo = {
    name = "my_plugin",      -- 必填
    version = "1.0.0",       -- 必填
    author = "作者",          -- 可选
    update_url = "..."        -- 可选
}

Quill.on_group_message = function(event)
    if event.raw_message == "ping" then
        bot.send_group_msg(event.group_id, "pong")
    end
end

return Quill
```

> 缺少 `PluginInfo`、或缺少 `name`/`version` 的插件会被拒绝加载。
> **文件结尾必须有 `return`**：没有返回值的 `.lua` 文件会被跳过并记录日志（不会导致进程退出）。

目录插件用 `init.lua` 入口，同目录其他 `.lua` 用 `include("filename")` 加载。

#### include() 详解

```lua
-- plugins/admin/init.lua
local utils = include("utils")         -- 自动补 .lua → 加载 admin/utils.lua
local sub   = include("sub/helper")    -- 子目录 → admin/sub/helper.lua

return {
    PluginInfo = { name = "admin", version = "1.0.0" },
    on_group_message = function(event)
        bot.send_group_msg(event.group_id, utils.stats())
    end,
}
```

- `include("name")` 自动补 `.lua` 后缀
- 路径相对于调用者所在目录
- 不能跨越到插件目录外（安全检查）
- **无缓存**：每次 `include()` 都会重新读取并执行目标文件，多个调用点会重复执行副作用
- 被 include 的文件应当 `return` 一个模块表

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

内置网络函数（见下）
  util.http_get(url)            →  JSON 字符串，永远返回字符串，用 ok 字段判断成败
  util.extract_archive(f, d)    →  成功为 JSON 字符串，失败为 nil, err
```

### bot API

`bot` 是全局表，共 46 个成员（含可选的 `bot.sql`）。

#### 消息发送

```lua
bot.send_group_msg(group_id, "text")             -- 纯文本 → message_id
bot.send_private_msg(user_id, "text")
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
bot.get_msg(message_id)                           -- 查消息，返回 JSON 字符串
bot.get_forward_msg("forward_id")                 -- 查合并转发，返回 JSON 字符串
```

#### 消息段构造器

```
bot.text("str")    bot.image("url")    bot.record("url")
bot.video("url")   bot.at(qq)          bot.reply(msg_id)
bot.forward("id")  bot.node(uid, "昵称", content)
bot.segment("type", data_table)         -- 自定义消息段
```

#### 群管理

```lua
bot.set_group_kick(gid, uid, reject_add_request?)
bot.set_group_ban(gid, uid, duration_seconds)   -- 0 表示解除
bot.set_group_whole_ban(gid, enable)
bot.set_group_admin(gid, uid, enable)
bot.set_group_card(gid, uid, card)
bot.set_group_name(gid, name)
bot.set_group_special_title(gid, uid, title, duration)
bot.set_group_leave(gid, is_dismiss?)
```

#### 请求处理与查询

```lua
bot.set_friend_add_request(flag, approve, remark?)
bot.set_group_add_request(flag, sub_type, approve, reason?)
bot.send_like(uid, times?)

-- 查询类均返回 JSON 字符串
bot.get_status()  bot.get_version()  bot.get_login_info()
bot.get_stranger_info(uid, no_cache?)
bot.get_friend_list()  bot.get_group_list()
bot.get_group_info(gid, no_cache?)   bot.get_group_member_info(gid, uid, no_cache?)
bot.get_group_member_list(gid)       bot.get_group_honor_info(gid, type?)
bot.get_cookies(domain)  bot.get_credentials(domain)  bot.get_csrf_token()
bot.get_record(file, out_format?)    bot.get_image(file)
bot.can_send_image()  bot.can_send_record()
```

### 事件处理

18 个处理函数，按类型独立分发。没有对应 handler 时回退到泛型的 `on_notice` / `on_request`。

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
on_offline_file      -- 离线文件     on_client_status   -- 客户端状态

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
event.self_id      event.time
event.sender = {user_id, nickname, card, role}
event.file = {id, name, size, busid}        -- 仅 upload / offline_file 事件
```

> 事件中**不包含**结构化的消息段数组，只能通过 `event.raw_message`（字符串）判断内容。

### util 工具集

全局可用，无需引入。共 39 个函数 + `MemFS`。

```
JSONToTable(str)           → table        TableToJSON(tbl)            → string
PrintTable(tbl)            → string       TableGet(tbl, "a.b.c")      → value
TableKeys(tbl)             → {keys}       TableLen(tbl)               → count
Base64Encode(str)          → string       Base64Decode(str)           → string
MD5(str)                   → hex          SHA1(str) SHA256(str) SHA512(str)
CRC32(str)                 → hex          UUID()                      → "xxxxxxxx-..."
URLEncode(str)             → string       URLDecode(str)              → string
AESEncrypt(plain, key)     → base64       AESDecrypt(cipher, key)     → string
IsUTF8(str)    IsJSON(str)    IsBase64(str)    IsSafePath(str)
IsValid(v)                  → bool        GetEnv(key, default?)
Sleep(seconds)              -- 阻塞       After(seconds, fn)          -- 返回 id
CancelTimer(id)             -- 取消定时器
DNSResolve(domain)          → ip           DNSResolveALL(domain)       → {[0]=v4, [1]=v6}
Log(level_str, msg)                        -- 异步日志（通道满时丢弃）
MemFS.Write/Read/Delete/Exists/List/Clear/Dump/Load  -- 内存键值存储
```

> AES 的 key **不硬编码**在源码中，用 `util.GetEnv("MY_KEY")` 从 .env 读。
> `GetEnv` 禁止读 `ONEBOT_*` 前缀的变量（但 Lua 标准库 `os.getenv` 不受此限制，见「安全 & 限制」）。
> `IsSafePath` 检查 `..` 与符号链接。

### 内置网络能力

全部由宿主实现，已注册进 `util`。返回值约定：HTTP 类返回 JSON 字符串，形如
`{"ok":true,"status":200,"body":"...","headers":{...}}` 或 `{"ok":false,"error":"..."}`。

```lua
-- 同步 HTTP GET（默认 60 秒超时；受 5 秒事件上限约束）
local raw  = util.http_get("https://api.example.com/data")
local resp = util.JSONToTable(raw)
if resp.ok then
    local body = resp.body
end

-- 同步 HTTP POST（注意：只接受 URL，无 body 参数）
util.http_post(url)

-- 完整控制：method / url / headers / body / timeout(秒)
util.http_request(util.TableToJSON({
    method  = "POST",
    url     = "https://api.example.com/submit",
    headers = { ["Content-Type"] = "application/json" },
    body    = '{"a":1}',
    timeout = 10,
}))

-- 异步多线程下载：立即返回任务 ID（字符串），不阻塞事件循环
local id  = util.download_file("https://example.com/big.zip", "downloads/big.zip", 8)
local st  = util.download_poll(id)     -- 任务不存在返回 nil
-- st = { id, name, done, total, downloaded, elapsed, percent, error? }
local all = util.download_list()       -- 数组表，列出近期任务

-- 解压：成功返回 JSON 字符串 {"ok":true,"dest":...,"count":n}，失败返回 nil, err
-- 同步版本会占用事件循环，仅适合小压缩包
local ok, err = util.extract_archive("downloads/big.zip", "downloads/out")

-- 大压缩包用异步版本（不占用事件循环）
local ex = util.extract_async("downloads/big.zip", "downloads/out")
local st = util.extract_poll(ex)       -- { id, file, dest, done, count, elapsed, error? }
```

支持的压缩格式：`.zip`、`.tar`、`.tar.gz`/`.tgz`、`.tar.bz2`/`.tbz2`、`.gz`、`.bz2`。
解压内置路径穿越防护，拒绝逃出目标目录的条目。

> `download_file` 的目标路径**不受** `data/` 沙箱限制，请只下载可信来源的 URL。

### 持久存储

```lua
bot.db.set("key", value)      -- 保存，value 支持嵌套 table，落盘为 data/<key>.json
bot.db.get("key")             -- 读取，不存在返回 nil
bot.db.del("key")             -- 删除

local data = bot.db.get("signin/" .. event.user_id) or {count = 0}
data.count = data.count + 1
bot.db.set("signin/" .. event.user_id, data)
```

key 中 `..` 会被拒绝；key 可包含 `/` 以生成子目录。

### SQL（可选）

配置 `ONEBOT_SQL_DRIVER=sqlite|mysql` 后，`bot.sql` 可用；未配置时该字段不存在。

```lua
-- exec -> 受影响行数, 自增 ID
local n, id = bot.sql.exec("INSERT INTO points (qq, score) VALUES (?, ?)", {event.user_id, 10})

-- query -> 数组表（每行一个 table）
local rows = bot.sql.query("SELECT qq, score FROM points ORDER BY score DESC LIMIT ?", {10})

-- query_row -> 第一行或 nil
local row = bot.sql.query_row("SELECT score FROM points WHERE qq = ?", {event.user_id})
local score = row and row.score or 0

-- 事务：正常结束提交，抛错回滚
local ok, err = bot.sql.tx(function(tx)
    tx.exec("UPDATE points SET score = score + ? WHERE qq = ?", {1, event.user_id})
end)
```

- 参数用 `?` 占位，驱动会做类型转换（整数、浮点、字符串、布尔）
- **Lua 数组里的 nil 会截断长度**，SQL NULL 请用 `bot.sql.NULL`：`{1, bot.sql.NULL, 3}`
- 查询结果里的 NULL 是 Lua `nil`
- SQL 在 5 秒事件预算内执行，慢查询会被中断
- SQLite 默认 `data/bot.db`，开启 WAL；MySQL 需要提供 `ONEBOT_SQL_DSN`

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

> 沙箱只约束 `bot.file` 与 `util.MemFS.*`。Lua 标准库 `io` / `os` 未被限制。

### 延迟执行

```lua
util.Sleep(3)                           -- 阻塞 3 秒，期间 bot 无响应
local id = util.After(3, function()     -- 非阻塞，返回定时器 ID
    bot.delete_msg(placeholder)
end)
util.CancelTimer(id)
```

定时器由后台循环驱动（约 200ms 粒度），无需等待新事件；每个回调同样受 5 秒执行上限约束。
回调内可以再次调用 `util.After` 实现轮询，不会被丢弃。

---

## Go 侧扩展内置能力

内置能力位于 `internal/module/builtin.go`，通过 `RegisterModuleUtil(L, util)` 注册到 `util` 表。

新增一个函数：

```go
func builtinSomething(L *lua.LState) int {
    arg := L.CheckString(1)
    // ... 实现
    L.Push(lua.LString(result))   // 成功：压入 1 个值
    return 1
    // 失败：L.Push(lua.LNil); L.Push(lua.LString(err)); return 2
}
```

然后在 `RegisterModuleUtil` 中注册：

```go
utilTbl.RawSetString("something", L.NewFunction(builtinSomething))
```

注意：

- 内置函数在宿主进程内同步执行，且运行于事件分发锁内，耗时操作会阻塞整个 bot
- 新增/修改内置能力需要重新 `go build` 并重启，**没有热重载**
- 若返回 JSON，请遵循 `{"ok":true,...}` / `{"ok":false,"error":"..."}` 约定

---

## 错误处理

### Lua 层：双返回值模式

```lua
local data, err = bot.file.read("config.json")
if not data then
    bot.send_group_msg(gid, "读取失败: " .. err)
    return
end

local ok, result = pcall(function()
    return risky_operation()
end)
if not ok then
    -- result 是错误信息
end
```

### 内置网络层：JSON 状态码

```lua
local resp = util.JSONToTable(util.http_get("https://bad.url"))
if not resp.ok then
    bot.send_group_msg(gid, "请求失败: " .. resp.error)
end
```

### 超时与阻塞

每个事件 handler 与每个定时器回调共享一个 **5 秒**执行预算；超时后 gopher-lua 会中断该段 Lua 代码，
记录错误日志并跳过该事件剩余的插件，然后继续处理后续事件。

`util.http_get` / `http_post` 默认超时 60 秒，`util.Sleep` 会被超时打断，`bot.*` 的 HTTP 调用也
跟随同一个预算取消。需要更久的任务请用 `util.download_file` / `util.extract_async`（异步）。

事件通过队列进入分发协程（上限 256），插件执行**不会阻塞 WebSocket 读取**；
队列持续溢出时会丢弃新事件并限频告警。

---

## 安全 & 限制

| 项 | 说明 |
|------|------|
| 插件隔离 | **无沙箱**。插件与宿主同进程，Lua `io`/`os`/`debug` 标准库可用 |
| 文件沙箱 | 仅 `bot.file` 与 `util.MemFS` 限定在 `data/`；`util.download_file` 可写任意路径 |
| 环境隔离 | `util.GetEnv` 不返回 `ONEBOT_*`；但 `os.getenv` 可直接读取 |
| 事件并发 | 串行分发；单次 handler / 定时器 5 秒上限 |
| HTTP | 默认校验证书；`ONEBOT_HTTP_INSECURE=true` 可关闭（高危，仅限自签环境） |
| 日志截断 | 单条日志上限 500 字符 |

---

## 已知限制

| 现象 | 说明 |
|------|------|
| 事件分发串行 | 同一时间一个 handler 执行，其他事件在队列中排队（上限 256） |
| 定时器精度 | 后台循环每 200ms 检查一次，触发时间有 0~200ms 偏差 |
| 插件无沙箱 | 等同本机代码，只安装可信插件 |
| 同步解压会阻塞 | `extract_archive` 执行期间事件循环停摆，大包请用 `extract_async` |

---

## 快速检查清单

创建插件时：

- [ ] 文件放在 `plugins/` 下，以 `return` 返回表
- [ ] `PluginInfo` 包含 `name` 和 `version`
- [ ] 事件 handler 名从上面 18 种中选
- [ ] 用了 AES 密钥则通过 `util.GetEnv` 读取
- [ ] 路径相关操作用 `bot.file`（限定 `data/`）
- [ ] 避免单次 handler 长时间阻塞（5 秒上限），耗时任务用 `util.download_file`
