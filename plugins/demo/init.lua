-- ============================================================
-- demo 插件 - 演示目录插件、include、WASM 模块、持久存储的用法
-- ============================================================

-- include 加载同目录或子目录下的 .lua 文件（会自动补 .lua）
-- 注意：include 只能在当前插件目录及其子目录内加载，不能跨目录
local utils = include("utils")          -- 加载 demo/utils.lua
include("sub/pic")                      -- 加载 demo/sub/pic.lua（直接注册事件）
local store = include("storage")        -- 加载 demo/storage.lua

local config = {
    prefix = "/demo",
}

return {
    on_group_message = function(event)
        local msg = event.raw_message

        -- 转发给存储插件（签到、积分等命令）
        if store and store.on_group_message then
            store.on_group_message(event)
        end

        -- 帮助
        if msg == config.prefix then
            bot.send_group_msg(event.group_id, {
                bot.at(event.user_id),
                bot.text(" 用法："),
                bot.text("\n签到        -- 每日签到+积分"),
                bot.text("\n我的积分    -- 查看积分"),
                bot.text("\n/demo time  -- 查看时间"),
                bot.text("\n/demo http  -- 测试 WASM 模块"),
                bot.text("\n/demo pic   -- 发张图"),
                bot.text("\n/demo file  -- 测试文件读写"),
            })
            return
        end

        -- 演示：调用 include 加载的 utils 模块
        if msg == config.prefix .. " time" then
            -- utils 是 include("utils") 返回的 Lua 表，可调用其方法
            bot.send_group_msg(event.group_id, {
                bot.text("当前时间：" .. utils.now()),
            })
            return
        end

        -- 演示：调用 WASM 模块
        -- module("http") 查找 modules/http/http.wasm，返回其导出函数表
        if msg == config.prefix .. " http" then
            local http = module("http")
            if http then
                -- 调用模块的 get 函数（对应 main.go 中的 //go:wasmexport get）
                local result = http.get("https://httpbin.org/get")
                bot.send_group_msg(event.group_id, {
                    bot.at(event.user_id),
                    bot.text(" 模块返回：" .. result),
                })
            else
                bot.send_group_msg(event.group_id, "HTTP 模块未加载，请检查 modules/http/http.wasm")
            end
            return
        end

        -- 演示：文件 I/O
        if msg == config.prefix .. " file" then
            bot.file.write("hello.txt", "Hello from Lua! " .. os.date())
            local content = bot.file.read("hello.txt")
            local files = bot.file.list("")
            local listStr = ""
            for _, f in ipairs(files) do
                listStr = listStr .. f .. " "
            end
            bot.send_group_msg(event.group_id, {
                bot.at(event.user_id),
                bot.text(" 文件已写入，内容：" .. content .. "\n目录文件：" .. listStr),
            })
            return
        end

        -- 演示：思考→撤回→输出（AI 接入模式）
        if msg == config.prefix .. " ai" then
            local placeholder = bot.send_group_msg(event.group_id, "AI 正在思考...")
            -- 模拟后端耗时（实际场景在这里调 AI API）
            local http = module("http")
            local result = ""
            if http then
                result = http.get("https://httpbin.org/get")
            end
            -- 撤回"正在思考"
            bot.delete_msg(placeholder)
            -- 输出结果
            bot.send_group_msg(event.group_id, {
                bot.reply(event.message_id),
                bot.text(" AI 回复：" .. result),
            })
            return
        end
    end,
}
