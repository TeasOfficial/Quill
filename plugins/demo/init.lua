local Quill = {}

Quill.PluginInfo = {
    name = "demo",
    version = "1.0.0",
    author = "Quill",
}

local utils = include("utils")
include("sub/pic")
local store = include("storage")

local config = {
    prefix = "/demo",
}

Quill.on_group_message = function(event)
    local msg = event.raw_message

    if store and store.on_group_message then
        store.on_group_message(event)
    end

    if msg == config.prefix then
        bot.send_group_msg(event.group_id, {
            bot.at(event.user_id),
            bot.text(" 用法："),
            bot.text("\n签到        -- 每日签到+积分"),
            bot.text("\n我的积分    -- 查看积分"),
            bot.text("\n/demo time  -- 查看时间"),
            bot.text("\n/demo http  -- 测试内置 HTTP 工具"),
            bot.text("\n/demo pic   -- 发张图"),
            bot.text("\n/demo file  -- 测试文件读写"),
        })
        return
    end

    if msg == config.prefix .. " time" then
        bot.send_group_msg(event.group_id, {
            bot.text("当前时间：" .. utils.now()),
        })
        return
    end

    if msg == config.prefix .. " http" then
        local result = util.http_get("https://httpbin.org/get")
        bot.send_group_msg(event.group_id, {
            bot.at(event.user_id),
            bot.text(" 模块返回：" .. (result or "nil")),
        })
        return
    end

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

    if msg == config.prefix .. " ai" then
        local placeholder = bot.send_group_msg(event.group_id, "AI 正在思考...")
        local result = util.http_get("https://httpbin.org/get") or ""
        bot.delete_msg(placeholder)
        bot.send_group_msg(event.group_id, {
            bot.reply(event.message_id),
            bot.text(" AI 回复：" .. result),
        })
        return
    end
end

return Quill
