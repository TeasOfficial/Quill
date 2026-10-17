-- ============================================================
-- poke.lua — 戳一戳回复（下载进度 / 帮助信息）
-- 仅在下述群聊生效
-- ============================================================
local Quill = {}

Quill.PluginInfo = {
    name    = "poke",
    version = "1.0.0",
    author  = "Quill",
}

-- 允许使用此功能的群号，留空则全部群生效
local ALLOW_GROUPS = {}

local function fmtSize(bytes)
    if bytes >= 1048576 then
        return string.format("%.1f MB", bytes / 1048576)
    elseif bytes >= 1024 then
        return string.format("%d KB", math.floor(bytes / 1024))
    else
        return bytes .. " B"
    end
end

local function fmtSeconds(s)
    if s < 60 then
        return string.format("%ds", math.floor(s))
    elseif s < 3600 then
        local m = math.floor(s / 60)
        return string.format("%dm%ds", m, math.floor(s % 60))
    else
        local h = math.floor(s / 3600)
        local m = math.floor((s % 3600) / 60)
        return string.format("%dh%dm", h, m)
    end
end

Quill.on_notify = function(event)
    if event.sub_type ~= "poke" then
        return
    end
    if event.target_id ~= event.self_id then
        return
    end

    -- 群聊白名单检查
    if #ALLOW_GROUPS > 0 then
        local allowed = false
        for _, gid in ipairs(ALLOW_GROUPS) do
            if event.group_id == gid then
                allowed = true
                break
            end
        end
        if not allowed then
            return
        end
    end

    local tasks = util.download_list()
    local n = 0
    if util.IsValid(tasks) then
        for _, t in pairs(tasks) do
            if not t.done then n = n + 1 end
        end
    end

    if n == 0 then
        bot.send_group_msg(event.group_id, {
            bot.at(event.user_id),
            bot.text(" 当前没有正在下载的任务 |ω・)"),
        })
        return
    end

    local msg = { bot.at(event.user_id) }
    local lines = {}

    for _, t in pairs(tasks) do
        if t.done then
            goto next_task
        end
        local line
        local pct = string.format("%.1f%%", t.percent)
        local speed = "-"
        if t.elapsed > 0 and t.downloaded > 0 then
            speed = fmtSize(t.downloaded / t.elapsed) .. "/s"
        end
        local prog = fmtSize(t.downloaded)
        if t.total > 0 then
            prog = prog .. " / " .. fmtSize(t.total)
        end
        line = string.format(" 📥 %s — %s (%s %s 耗时%s)",
            t.name, pct, prog, speed, fmtSeconds(t.elapsed))
        table.insert(lines, line)
        ::next_task::
    end

    table.insert(msg, bot.text("\n" .. table.concat(lines, "\n")))
    bot.send_group_msg(event.group_id, msg)
end

return Quill
