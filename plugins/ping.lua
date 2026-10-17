local Quill = {}

Quill.PluginInfo = {
    name = "ping",
    version = "1.0.0",
    author = "Quill",
}

Quill.on_group_message = function(event)
    if event.raw_message == "ping" then
        bot.send_group_msg(event.group_id, "pong")
    end

    if event.raw_message == "你好" then
        bot.send_group_msg(event.group_id, {
            bot.at(event.user_id),
            bot.text(" 你好呀！"),
        })
    end
end

return Quill
