# 插件开发

## 内置函数

HTTP 请求、多线程下载、压缩包解压等常用能力已整合为 `util` 函数：

```lua
-- HTTP 请求（同步，即时返回）
local resp = util.http_get("https://httpbin.org/get")
local resp = util.http_post("https://httpbin.org/post")
local resp = util.http_request('{"method":"GET","url":"...","headers":{},"timeout":10}')

-- 异步下载（立即返回任务 ID，不阻塞事件循环）
local task_id = util.download_file(url, save_path, 8)
-- 轮询等待完成
local function poll()
    local t = util.download_poll(task_id)
    if t and t.done then
        if t.error then print("失败: " .. t.error) else print("完成") end
    else
        util.After(1, poll)
    end
end
util.After(0.5, poll)

-- 解压（同步）
local result, err = util.extract_archive(file, dest_dir)

-- 列出下载进度
local tasks = util.download_list()
```

HTTP / 解压返回 JSON 字符串，用 `util.JSONToTable` 解析。详见 [util API 参考](util_api)。

## Lua 插件

所有插件用纯 Lua 编写，放在 `plugins/` 目录中，通过 `include()` 加载子模块。

## 目录结构

```
plugins/my_plugin.lua        # 单文件插件
plugins/my_plugin/           # 目录插件
├── init.lua                 # 入口
├── utils.lua                # include("utils")
└── sub/extra.lua            # include("sub/extra")
```
