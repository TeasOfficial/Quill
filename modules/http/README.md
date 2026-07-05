# HTTP 模块

提供 HTTP 请求能力，支持 GET / POST / 自定义请求。

## Lua 调用

```lua
local http = module("http")

-- GET
local resp = http.get("https://httpbin.org/get")

-- POST
local resp = http.post("https://httpbin.org/post")

-- 自定义请求
local resp = http.request('{"method":"DELETE","url":"https://api.example.com/item/1","headers":{"Authorization":"Bearer xxx"}}')
```

## 返回格式

```json
{"ok":true,"status":200,"body":"...","headers":{"Content-Type":"..."}}
{"ok":false,"error":"connection refused"}
```
