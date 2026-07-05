-- ============================================================
-- demo/storage.lua - 数据持久化示例：签到和积分系统
-- 通过 bot.db.set/get 保存用户数据到 data/ 目录
-- ============================================================

return {
    on_group_message = function(event)
        local db = bot.db
        local uid = "sign/" .. event.group_id .. "/" .. event.user_id  -- 每个群独立
        local user = db.get(uid)

        -- 首次使用：初始化用户数据
        if not user then
            user = {
                points = 0,          -- 积分
                last_sign = "",      -- 最后签到日期
                sign_count = 0,      -- 累计签到天数
            }
        end

        local today = os.date("%Y-%m-%d")

        -- 签到
        if event.raw_message == "签到" then
            if user.last_sign == today then
                bot.send_group_msg(event.group_id, {
                    bot.at(event.user_id),
                    bot.text(" 今天已经签到过了！当前积分: " .. user.points),
                })
            else
                -- 连续签到奖励
                local bonus = 10
                if user.last_sign == os.date("%Y-%m-%d", os.time() - 86400) then
                    bonus = bonus + 5  -- 连续签到加 5
                end
                user.points = user.points + bonus
                user.sign_count = user.sign_count + 1
                user.last_sign = today
                db.set(uid, user)  -- ❖ 保存到文件

                bot.send_group_msg(event.group_id, {
                    bot.at(event.user_id),
                    bot.text(" 签到成功！+" .. bonus .. " 积分，连续 " .. user.sign_count .. " 天，总积分: " .. user.points),
                })
            end
            return
        end

        -- 查看积分
        if event.raw_message == "我的积分" then
            bot.send_group_msg(event.group_id, {
                bot.at(event.user_id),
                bot.text(" 积分: " .. user.points .. "，已签到 " .. user.sign_count .. " 天"),
            })
            return
        end

        -- 积分排行榜（从所有用户数据中计算）
        if event.raw_message == "积分排行" then
            -- 读取该群所有签到数据
            bot.send_group_msg(event.group_id, "积分排行榜暂未实现，请期待后续版本！")
            return
        end
    end,
}
