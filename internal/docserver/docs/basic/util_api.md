# util API 参考

`util` 是全局可用的工具集，所有插件中直接使用，无需 `include()`。

---

## JSON

### util.JSONToTable

JSON 字符串 → Lua 表。

```lua
util.JSONToTable(json_string)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | table | 解析后的 Lua 表 |
| 失败 | nil, string | nil + 错误信息 |

```lua
local t = util.JSONToTable('{"name":"Alice","age":18}')
print(t.name, t.age)  -- Alice  18
```

支持嵌套对象和数组。

---

### util.TableToJSON

Lua 表 → JSON 字符串。

```lua
util.TableToJSON(table)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | JSON 字符串 |
| 失败 | nil, string | nil + 错误信息 |

```lua
local json = util.TableToJSON({a = 1, b = {x = 2}})
-- → '{"a":1,"b":{"x":2}}'
```

---

## 编码

### util.Base64Encode

Base64 编码。

```lua
util.Base64Encode(string)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | Base64 编码字符串 |

```lua
local enc = util.Base64Encode("Hello")
-- → "SGVsbG8="
```

### util.Base64Decode

Base64 解码。

```lua
util.Base64Decode(string)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | 解码后的原始字符串 |
| 失败 | nil, string | nil + 错误信息 |

---

### util.URLEncode

URL 编码（percent-encoding）。

```lua
util.URLEncode(string)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | URL 编码字符串 |

```lua
local enc = util.URLEncode("你好 world")
-- → "%E4%BD%A0%E5%A5%BD+world"
```

### util.URLDecode

URL 解码。

```lua
util.URLDecode(string)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | 原始字符串 |
| 失败 | nil, string | nil + 错误信息 |

---

## 哈希 & 校验

### util.MD5

```lua
util.MD5(string)
```

返回 32 位小写十六进制字符串。

```lua
util.MD5("hello")  -- → "5d41402abc4b2a76b9719d911017c592"
```

### util.SHA1

```lua
util.SHA1(string)
```

返回 40 位小写十六进制字符串。

### util.SHA256

```lua
util.SHA256(string)
```

返回 64 位小写十六进制字符串。

### util.SHA512

```lua
util.SHA512(string)
```

返回 128 位小写十六进制字符串。

### util.CRC32

```lua
util.CRC32(string)
```

返回 8 位大写十六进制（IEEE CRC32）。

```lua
util.CRC32("hello")  -- → "3610A686"
```

常用于文件完整性校验：

```lua
local data = bot.file.read("downloads/file.bin")
local checksum = util.CRC32(data)
if checksum ~= expected then
    bot.send_group_msg(gid, "文件校验失败！")
end
```

---

## 加密

### util.AESEncrypt

AES-256-CBC 加密。

```lua
util.AESEncrypt(plaintext, key)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `plaintext` | string | 明文 |
| `key` | string | 任意长度字符串，内部 SHA256 派生为 32 字节密钥 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | Base64 编码的密文（含 16 字节随机 IV） |
| 失败 | nil, string | nil + 错误信息 |

```lua
local cipher = util.AESEncrypt("敏感数据", "my-secret-key")
-- → "kFT/Gz...base64..."
```

> 每次加密结果不同（随机 IV），密钥 **不应硬编码**，用 `util.GetEnv("MY_KEY")` 从 `.env` 读取。

---

### util.AESDecrypt

AES-256-CBC 解密。

```lua
util.AESDecrypt(ciphertext, key)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `ciphertext` | string | AESEncrypt 返回的 Base64 密文 |
| `key` | string | 与加密时相同的密钥 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | 明文 |
| 失败 | nil, string | nil + 错误信息 |

```lua
local plain = util.AESDecrypt(cipher, util.GetEnv("MY_KEY"))
```

---

## 校验

### util.IsValid

判断变量是否为有效值（非 nil、非 false）。

```lua
util.IsValid(value)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 有效 | boolean | `true`（0、空串、空表都算有效） |
| 无效 | boolean | `false`（nil 或 false） |

### util.IsUTF8

```lua
util.IsUTF8(string)
```

判断字符串是否为合法 UTF-8 编码。

### util.IsJSON

```lua
util.IsJSON(string)
```

判断字符串是否为合法 JSON。

### util.IsBase64

```lua
util.IsBase64(string)
```

判断字符串是否为合法 Base64 编码。

### util.IsSafePath

```lua
util.IsSafePath(path)
```

判断路径是否在 `data/` 内且无 `..`。

---

## 环境

### util.GetEnv

读取 `.env` 环境变量。

```lua
util.GetEnv(key, default?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `key` | string | 变量名 |
| `default` | string | 可选，不存在时的默认值 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 存在 | string | 变量值 |
| 不存在 | nil 或默认值 | |

> `ONEBOT_*` 开头的变量返回 `nil`，防止插件泄露敏感配置。

```lua
local token = util.GetEnv("MY_API_TOKEN")
local limit = util.GetEnv("MY_LIMIT", "100")
```

---

## 时间 & 定时器

### util.Sleep

阻塞延迟。

```lua
util.Sleep(seconds)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `seconds` | number | 秒数，支持小数（0.5 = 500ms） |

> ⚠️ 阻塞期间所有事件停止处理。一般用 `util.After` 代替。

### util.After

非阻塞延迟回调。

```lua
util.After(seconds, callback)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `seconds` | number | 秒数 |
| `callback` | function | 到期后执行的函数 |

| 返回 | 类型 | 说明 |
|------|------|------|
| number | 定时器 ID | 用于 `CancelTimer` |

```lua
local tid = util.After(3, function()
    bot.delete_msg(message_id)
end)
```

> 回调在下一次收到任意事件时执行，最长延迟 = seconds + 事件间隔。

### util.CancelTimer

取消定时器。

```lua
util.CancelTimer(id)
```

```lua
local tid = util.After(10, function()
    bot.send_group_msg(gid, "超时了")
end)
-- 条件满足，取消
util.CancelTimer(tid)
```

---

## 日志

异步日志，不阻塞主线程。输出到控制台，带时间戳和级别标签。

### util.Log

```lua
util.Log(level, message)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `level` | string | `"INFO"` `"WARN"` `"ERROR"` `"DEBUG"` |
| `message` | string | 日志内容 |

```lua
util.Log("INFO", "插件启动完成")
util.Log("WARN", "API 响应超时，使用缓存数据")
util.Log("ERROR", "数据库写入失败: " .. err)
util.Log("DEBUG", "事件数据: " .. util.TableToJSON(event))
```

输出格式：`2026-07-05 22:45:01 [INFO ] 插件启动完成`

> 256 缓冲区异步写入，不用 `print()` 可免阻塞主线程。

### util.UUID

生成 UUID v4（RFC 4122）。

```lua
util.UUID()
-- → "550e8400-e29b-41d4-a716-446655440000"
```

基于 `crypto/rand`，用于生成唯一 ID。

### util.PrintTable

格式化打印表为字符串（调试用）。

```lua
util.PrintTable(table)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| string | 多行缩进的格式化字符串 |

```lua
local t = {name = "Bob", items = {"x", "y", {z = 1}}}
print(util.PrintTable(t))
-- → {
--     ["items"] = {
--       [1] = x,
--       [2] = y,
--       [3] = {
--         ["z"] = 1
--       }
--     },
--     ["name"] = Bob
--   }
```

> 嵌套超过 8 层自动截断为 `{...}` 防无限递归。单表最多 32 键，总输出上限 8KB。

### util.TableKeys

列出表的所有键名，不递归。

```lua
util.TableKeys(table)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| table | 键名数组 | `{"a", "b", "c"}` |

```lua
local keys = util.TableKeys(_G)
-- → {"_G", "bot", "util", "include", ...}
```

### util.TableLen

返回表的键数量。

```lua
util.TableLen(table)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| number | 键总数 |

### util.TableGet

按点路径安全取值，路径中任意层为 nil 不会报错。

```lua
util.TableGet(table, "a.b.c")
```

| 返回 | 类型 | 说明 |
|------|------|------|
| any | 路径末端的值 | 或 nil |

```lua
local v = util.TableGet(event, "sender.nickname")
-- 等价于 event.sender.nickname 但防 nil 崩溃
```

---

## 网络

### util.DNSResolve

域名解析，返回单个 IP（v4 优先）。

```lua
util.DNSResolve(domain)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | IP 地址 |
| 失败 | nil, string | nil + 错误信息 |

### util.DNSResolveALL

域名解析，返回全部 IP，按类型分索引。

```lua
util.DNSResolveALL(domain)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| table | `{[0]=v4, [1]=v6}` | 无对应类型时键不存在 |

### util.http_get

HTTP GET 请求，返回 JSON 响应字符串。

```lua
util.http_get(url)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| string | JSON 响应 | `{"ok":true,"status":200,"body":"...","headers":{...}}` |

```lua
local resp = util.JSONToTable(util.http_get("https://httpbin.org/get"))
if resp.ok then
    print(resp.body)
end
```

### util.http_post

HTTP POST 请求。

```lua
util.http_post(url)
```

返回值同 `http_get`。

### util.http_request

完整参数 HTTP 请求，传入 JSON 字符串。

```lua
util.http_request(reqJSON)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `reqJSON` | string | `{"method":"POST","url":"...","headers":{},"body":"...","timeout":10}` |

### util.download_file

异步多线程文件下载，支持断点续传。**立即返回任务 ID**，不阻塞事件循环。

```lua
local task_id = util.download_file(url, save_path, threads?)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| string | 任务 ID | 用于 `download_poll()` 查询进度 |

`threads` 默认 4，可选。

```lua
local task_id = util.download_file("https://example.com/file.zip", "data/dl/file.zip", 8)
```

> 下载在后台 goroutine 中执行，不阻塞其他事件（如戳一戳）。

### util.download_poll

查询单个下载任务进度。

```lua
util.download_poll(task_id)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| table | 任务状态 | `{done, total, downloaded, percent, elapsed, error}` |
| nil | — | 任务不存在 |

```lua
local function waitDownload(task_id)
    local t = util.download_poll(task_id)
    if not t then return end
    if t.done then
        if t.error then
            print("下载失败: " .. t.error)
        else
            print("下载完成: " .. t.total)
        end
    else
        util.After(1, function() waitDownload(task_id) end)
    end
end

local tid = util.download_file(url, path, 8)
util.After(0.5, function() waitDownload(tid) end)
```

### util.download_list

返回所有下载任务（包括已完成和进行中）。

```lua
util.download_list()
```

| 返回 | 类型 | 说明 |
|------|------|------|
| table | 任务数组 | `{[1]={id,name,total,downloaded,percent,elapsed,done,error}, ...}` |

### util.extract_archive

解压压缩包，支持 zip / tar / tar.gz / tar.bz2 / gz / bz2。返回 JSON 响应字符串。

```lua
local result, err = util.extract_archive(file, dest_dir)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | `{"ok":true,"dest":"...","count":42}` |
| 失败 | nil, string | nil + 错误信息 |

```lua
local result, err = util.extract_archive("data/downloads/file.tar.gz", "data/extracted/")
```

内置路径穿越防护，自动拒绝写入目标目录外的文件。

---

## 内存存储

`util.MemFS` 是进程内存键值存储，支持二进制数据，零磁盘 I/O。

### util.MemFS.Write

```lua
util.MemFS.Write(name, data)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `name` | string | 键名 |
| `data` | string | 值（支持二进制） |

### util.MemFS.Read

```lua
util.MemFS.Read(name)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 存在 | string | 值 |
| 不存在 | nil | |

### util.MemFS.Delete

```lua
util.MemFS.Delete(name)
```

### util.MemFS.Exists

```lua
util.MemFS.Exists(name)  -- → true/false
```

### util.MemFS.List

```lua
util.MemFS.List()  -- → {"cache1", "cache2", ...}
```

### util.MemFS.Clear

```lua
util.MemFS.Clear()  -- 清空所有数据
```

### util.MemFS.Dump

内存 → 磁盘持久化。

```lua
util.MemFS.Dump(name, path)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `name` | string | MemFS 中的键 |
| `path` | string | 目标磁盘路径（在 `data/` 内） |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

### util.MemFS.Load

磁盘 → 内存加载。

```lua
util.MemFS.Load(path)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | MemFS 中的键名（文件基础名） |
| 失败 | nil, string | nil + 错误信息 |
