-- ============================================================
-- demo/sub/pic.lua - 发图功能
-- 被 init.lua 通过 include("sub/pic") 加载
-- 子目录下的插件也可以直接注册事件处理函数
-- ============================================================

return {
    on_group_message = function(event)
        if event.raw_message == "/demo pic" then
            bot.send_group_msg(event.group_id, {
                bot.text("来张图："),
                -- 使用 bot.image() 构造图片消息段
                bot.image("https://picsum.photos/400/300"),
            })
        end
    end,
}
