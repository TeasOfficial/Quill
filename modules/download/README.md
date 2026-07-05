# 下载模块

多线程文件下载，支持断点续传。

## Lua 调用

```lua
local dl = module("download")

local r = dl.dl('{"url":"https://example.com/file.tar.bz2","path":"downloads/file.tar.bz2","threads":8}')
-- → {"ok":true,"path":"downloads/file.tar.bz2","size":52428800}
```

## 参数

| 字段 | 必填 | 说明 |
|------|------|------|
| `url` | 是 | 下载地址 |
| `path` | 是 | 保存路径 |
| `threads` | 否 | 线程数，默认 4 |

文件 > 5MB 时自动多线程下载，否则流式单线程。
