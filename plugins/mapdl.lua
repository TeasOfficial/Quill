-- ============================================================
-- mapdl.lua — CS:S BHop 地图下载 / 删除插件
-- /mapdl <地图名>  下载并安装地图
-- /mapdel <地图名>  删除已安装地图
-- 使用内置 util.download_file + util.extract_archive
-- ============================================================

-- 本机路径通过 .env 配置，不要把目录写进仓库。
-- MAPDL_MAPS_DIR    CS:S 服务器的 cstrike/maps 目录（必填，不填则插件提示未配置）
-- MAPDL_TEMP_DIR    下载临时目录（可选，默认 mapdl_temp）
-- MAPDL_FASTDL_BASE 地图源（可选）
local FASTDL_BASE = util.GetEnv("MAPDL_FASTDL_BASE") or "https://main.fastdl.me/maps/"
local MAPS_DIR    = util.GetEnv("MAPDL_MAPS_DIR") or ""
local TEMP_DIR    = util.GetEnv("MAPDL_TEMP_DIR") or "mapdl_temp"
local CONFIGURED  = MAPS_DIR ~= ""

-- ============================================================
-- 内置模块 (util.download_file / util.extract_archive)
-- ============================================================

-- ============================================================
-- PluginInfo (Quill 新版必须)
-- ============================================================
local Quill = {}

Quill.PluginInfo = {
    name    = "mapdl",
    version = "2.0.0",
    author  = "TeasBunnyHop",
}

-- ============================================================
-- 工具函数
-- ============================================================

--- 格式化字节数为人类可读字符串
local function fmt_size(bytes)
    if bytes >= 1048576 then
        return string.format("%.1f MB", bytes / 1048576)
    elseif bytes >= 1024 then
        return string.format("%d KB", math.floor(bytes / 1024))
    else
        return bytes .. " B"
    end
end

--- 向触发消息的群发送 AT + 文本回复
local function reply(event, text)
    bot.send_group_msg(event.group_id, {
        bot.at(event.user_id),
        bot.text(text),
    })
end

-- ============================================================
-- 输入校验与清理
-- ============================================================

--- 验证地图名只包含合法字符 (a-z A-Z 0-9 _ -)
local function is_valid_mapname(name)
    if name == "" then
        return false
    end
    return name:match("^[a-zA-Z0-9_%-]+$") ~= nil
end

--- 清理地图名: 去掉 .bsp / .bz2 / .bsp.bz2 后缀, 去首尾空格
local function clean_mapname(raw)
    local name = raw
        :gsub("%.bsp%.bz2$", "")
        :gsub("%.bsp$", "")
        :gsub("%.bz2$", "")
        :match("^%s*(.-)%s*$")
    return name or ""
end

-- ============================================================
-- /mapdel — 删除已安装的地图
-- ============================================================

function Quill.on_group_message(event)
    local msg = event.raw_message
    if not msg:match("^/mapd[el]") then
        return
    end

    if not CONFIGURED then
        reply(event, " ⚠ 未配置 MAPDL_MAPS_DIR（在 .env 中设置 cstrike/maps 目录）")
        return
    end

    -- ==========================================
    -- /mapdel <地图名>
    -- ==========================================
    local del_name = msg:match("^/mapdel%s+(.+)$")
    if del_name then
        del_name = clean_mapname(del_name)

        if not is_valid_mapname(del_name) then
            reply(event, " ❌ 格式无效")
            return
        end

        local dp = MAPS_DIR .. "\\" .. del_name .. ".bsp"

        -- MAPS_DIR 在 data/ 沙盒外，需用 io.open
        local f = io.open(dp, "r")
        if not f then
            reply(event, " ⚠ " .. del_name .. " 未安装")
            util.Log("INFO", string.format("QQ:%d 删除 %s — 未安装",
                event.user_id, del_name))
            return
        end

        local kb = math.floor(f:seek("end") / 1024)
        f:close()

        if os.remove(dp) then
            reply(event, " ✅ " .. del_name .. " 已删除 (" .. kb .. " KB)")
            util.Log("INFO", string.format("QQ:%d 删除 %s — 已删除 %d KB",
                event.user_id, del_name, kb))
        else
            reply(event, " ❌ " .. del_name .. " 删除失败")
            util.Log("ERROR", string.format("QQ:%d 删除 %s — 删除失败 %d KB",
                event.user_id, del_name, kb))
        end
        return
    end

    -- ==========================================
    -- /mapdl (无参数 → 帮助)
    -- ==========================================
    if msg == "/mapdl" then
        reply(event, " 📦 /mapdl <地图名>\n🗑 /mapdel <地图名>")
        return
    end

    -- ==========================================
    -- /mapdl <地图名>
    -- ==========================================
    local raw_name = msg:match("^/mapdl%s+(.+)$")
    if not raw_name then
        return
    end

    local map_name = clean_mapname(raw_name)

    if not is_valid_mapname(map_name) then
        reply(event, " ❌ 格式无效，地图名只能包含字母、数字、_ 和 -")
        return
    end

    local url       = FASTDL_BASE .. map_name .. ".bsp.bz2"
    local bz2_path  = TEMP_DIR .. "\\" .. map_name .. ".bsp.bz2"
    local dest_path = MAPS_DIR .. "\\" .. map_name .. ".bsp"

    -- 确保临时目录存在
    os.execute('mkdir "' .. TEMP_DIR .. '" 2>nul')

    -- 检查是否已安装 (MAPS_DIR 在 data/ 外，需 io.open)
    do
        local f = io.open(dest_path, "r")
        if f then
            local kb = math.floor(f:seek("end") / 1024)
            f:close()
            reply(event, " ✅ " .. map_name .. " 已安装 (" .. kb .. " KB)")
            util.Log("INFO", string.format("QQ:%d 下载 %s — 已安装 %d KB",
                event.user_id, map_name, kb))
            return
        end
    end

    -- 清理残留临时文件
    os.remove(bz2_path)

    reply(event, " 📥 " .. map_name .. " 正在下载...")
    util.Log("INFO", string.format("QQ:%d 下载 %s — 下载中 %s",
        event.user_id, map_name, url))

    -- ==========================================
    -- Step 1: 异步下载 .bz2
    -- ==========================================
    local task_id = util.download_file(url, bz2_path, 8)

    local ctx = {
        event     = event,
        map_name  = map_name,
        bz2_path  = bz2_path,
        dest_path = dest_path,
        MAPS_DIR  = MAPS_DIR,
    }

    local function pollDownload()
        local status = util.download_poll(task_id)
        if not util.IsValid(status) then
            os.remove(bz2_path)
            return
        end

        if status.done then
            if status.error then
                reply(event, " ❌ " .. map_name .. " 下载失败: " .. status.error)
                util.Log("ERROR", string.format("QQ:%d 下载 %s — 下载失败: %s",
                    event.user_id, map_name, status.error))
                os.remove(bz2_path)
                return
            end

            util.Log("INFO", string.format("QQ:%d 下载 %s — 下载完成，开始解压",
                event.user_id, map_name))

            local ok, err = pcall(function()
                local ex_resp_str, ex_err = util.extract_archive(bz2_path, MAPS_DIR)
                if ex_err then error(ex_err) end
                local ex_result = util.JSONToTable(ex_resp_str)
                if not ex_result or not ex_result.ok then error("解压失败") end

                os.remove(bz2_path)

                local f = io.open(dest_path, "r")
                if f then
                    local kb = math.floor(f:seek("end") / 1024)
                    f:close()
                    reply(event, " ✅ " .. map_name .. " 已成功安装 (" .. kb .. " KB)")
                    util.Log("INFO", string.format("QQ:%d 下载 %s — 成功 %d KB",
                        event.user_id, map_name, kb))
                else
                    reply(event, " ❌ " .. map_name .. " 安装验证失败")
                    util.Log("ERROR", string.format("QQ:%d 下载 %s — 安装验证失败",
                        event.user_id, map_name))
                end
            end)
            if not ok then
                reply(event, " ❌ " .. map_name .. " 处理失败: " .. tostring(err))
                util.Log("ERROR", string.format("QQ:%d 下载 %s — 处理失败: %s",
                    event.user_id, map_name, tostring(err)))
                os.remove(bz2_path)
            end
        else
            util.After(2, pollDownload)
        end
    end

    util.After(0.5, pollDownload)
    return
end

return Quill
