-- ============================================================
-- ping.lua - 最简单的单文件插件示例
-- 返回函数表，定义事件处理函数即可
-- ============================================================

return {
    -- 收到群消息时被调用，event 是事件数据表
    on_group_message = function(event)
        -- 简单回复
        if event.raw_message == "ping" then
            bot.send_group_msg(event.group_id, "pong")
        end

        -- 复杂消息：AT + 文本
        if event.raw_message == "你好" then
            bot.send_group_msg(event.group_id, {
                bot.at(event.user_id),   -- AT 发送者
                bot.text(" 你好呀！"),     -- 文本消息
            })
        end
    end,
}
