# 插件开发

## 插件写法

每个插件是 `plugins/` 下的 `.lua` 文件，**必须返回一个包含 `PluginInfo` 的表**。

`PluginInfo` 必须包含 `name`（插件名）和 `version`（版本号），可选 `author`（作者）和 `update_url`（更新地址）。

### 单文件插件

```lua
-- plugins/hello.lua
local Quill = {}

Quill.PluginInfo = {
    name = "hello",
    version = "1.0.0",
    author = "你的名字",        -- 可选
    update_url = "..."          -- 可选
}

Quill.on_group_message = function(event)
    if event.raw_message == "你好" then
        bot.send_group_msg(event.group_id, "你也好！")
    end
end

return Quill
```

### 目录插件（init.lua）

目录里有 `init.lua` 时，bot 自动加载它作为入口，同目录其他 `.lua` 不自动加载。

```
plugins/
└── admin/
    ├── init.lua      ← 入口，自动加载
    ├── utils.lua     ← 由 init.lua include
    └── config.lua    ← 由 init.lua include
```

```lua
-- plugins/admin/init.lua
local Quill = {}

Quill.PluginInfo = {
    name = "admin",
    version = "1.0.0",
    author = "你的名字",
}

local utils = include("utils")
local cfg = include("config")

Quill.on_group_message = function(event)
    if event.raw_message == "/info" then
        bot.send_group_msg(event.group_id, utils.stats())
    end
end

return Quill
```

`include("xxx")` 自动补 `.lua`，支持子目录：`include("sub/file")`。不能加载到插件目录外。

## 调用内置函数

HTTP 请求、下载、解压等已内置为 `util` 函数，无需额外加载：

```lua
local resp = util.http_get("https://api.example.com/data")
bot.send_group_msg(event.group_id, "返回 " .. resp)
```

参见 [util API 参考](util_api) 中的「网络操作」章节。

## 事件类型

| 处理函数 | 触发时机 |
|----------|----------|
| `on_group_message` | 群消息 |
| `on_private_message` | 私聊 |
| `on_group_increase` | 有人入群 |
| `on_group_decrease` | 有人退群 |
| `on_group_ban` | 群禁言 / 解禁 |
| `on_group_admin` | 管理员变更 |
| `on_group_upload` | 群文件上传 |
| `on_group_recall` | 群消息撤回 |
| `on_group_card` | 群名片变更 |
| `on_friend_add` | 新好友添加 |
| `on_friend_recall` | 好友消息撤回 |
| `on_notify` | 戳一戳 / 运气王 / 荣誉 |
| `on_essence` | 精华消息 |
| `on_friend_request` | 加好友请求 |
| `on_group_request` | 加群 / 邀请 |
| `on_meta_event` | 心跳 / 生命周期 |
| `on_notice` | 以上 notice 事件的泛型回退 |
| `on_request` | 以上 request 事件的泛型回退 |

## 事件数据字段

`event` 是一个表，根据事件类型包含不同字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `event.post_type` | string | message / notice / request / meta_event |
| `event.sub_type` | string | 子类型（ban、poke 等） |
| `event.user_id` | number | 触发者 QQ |
| `event.group_id` | number | 群号 |
| `event.message_id` | number | 消息 ID |
| `event.raw_message` | string | 消息原文 |
| `event.comment` | string | 验证消息（request 事件） |
| `event.flag` | string | 请求标识（用于处理请求） |
| `event.target_id` | number | 戳一戳目标 |
| `event.operator_id` | number | 操作者（禁言/管理 等） |
| `event.file` | table | 文件信息（上传事件）`{id, name, size, busid}` |
| `event.sender` | table | 发送者 `{user_id, nickname, card, role}` |

## 内置工具 util

`util` 是全局可用的工具表，无需引入。

| 方法 | 说明 |
|------|------|
| `util.JSONToTable(str)` | JSON 字符串 → Lua 表 |
| `util.TableToJSON(tbl)` | Lua 表 → JSON 字符串 |
| `util.PrintTable(tbl)` | 格式化打印表（调试用） |
| `util.CRC32(str)` | IEEE CRC32 校验（返回 8 位十六进制） |
| `util.Base64Encode(str)` | Base64 编码 |
| `util.Base64Decode(str)` | Base64 解码 |
| `util.MD5(str)` | MD5 哈希（32 位小写十六进制） |
| `util.SHA256(str)` | SHA-256 哈希（64 位小写十六进制） |
| `util.SHA512(str)` | SHA-512 哈希（128 位小写十六进制） |
| `util.IsUTF8(str)` | 字符串是否为合法 UTF-8 |
| `util.IsJSON(str)` | 字符串是否为合法 JSON |
| `util.IsBase64(str)` | 字符串是否为合法 Base64 |
| `util.IsSafePath(str)` | 路径是否在 data/ 内且无 `..` |
| `util.IsValid(v)` | 变量是否为有效值（非 nil 非 false） |
| `util.UUID()` | 生成 UUID v4 |
| `util.URLEncode(str)` | URL 编码 |
| `util.URLDecode(str)` | URL 解码 |
| `util.AESEncrypt(plain, key)` | AES-256-CBC 加密（返回 Base64） |
| `util.AESDecrypt(cipher, key)` | AES-256-CBC 解密 |
| `util.GetEnv(key, default?)` | 读取 .env 环境变量，可设默认值 |
| `util.Sleep(seconds)` | 延迟指定秒数后继续执行（**阻塞**） |
| `util.After(seconds, fn)` | 延迟指定秒数后执行回调（**非阻塞**），返回定时器 ID |
| `util.CancelTimer(id)` | 取消指定 ID 的定时器 |
| `util.http_get(url)` | HTTP GET 请求，返回 JSON 响应字符串 |
| `util.http_post(url)` | HTTP POST 请求，返回 JSON 响应字符串 |
| `util.http_request(req)` | 完整参数 HTTP 请求，传入 JSON |
| `util.download_file(url, path, threads?)` | 异步多线程下载，返回任务 ID |
| `util.download_poll(task_id)` | 查询下载任务进度 |
| `util.download_list()` | 列出所有下载任务 |
| `util.extract_archive(file, dest)` | 解压 (zip/tar/gz/bz2)，同步，含路径穿越防护 |
| `util.extract_async(file, dest)` | 异步解压，返回任务 ID（大压缩包用这个） |
| `util.extract_poll(task_id)` | 查询解压进度 |

> ⚠️ `AESEncrypt` / `AESDecrypt` 的 key **不应硬编码在插件源码中**。建议用 `util.GetEnv("MY_KEY")` 从 `.env` 读取。

```lua
-- JSON ↔ Table
local t = util.JSONToTable('{"name":"Alice","age":18}')
local json = util.TableToJSON({a = 1, b = {x = 2}})

-- 表格式化输出
local text = util.PrintTable({name = "Bob", items = {"x", "y"}})
-- → {
--     ["items"] = {
--       [1] = x,
--       [2] = y
--     },
--     ["name"] = Bob
--   }

-- CRC32 校验
local checksum = util.CRC32("hello")
-- → "3610A686"
```
