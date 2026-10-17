# 消息发送

## 纯文本

```lua
bot.send_group_msg(group_id, "你好世界")
bot.send_private_msg(user_id, "私聊消息")

-- 通用发送
bot.send_msg("group", group_id, "群消息")
bot.send_msg("private", user_id, "私聊消息")
```

## 消息管理

```lua
-- 撤回消息
bot.delete_msg(message_id)

-- 查询消息
local data = bot.get_msg(message_id)

-- 查询合并转发
local fwd = bot.get_forward_msg("forward_id")
```

## 文字 + 图片

```lua
bot.send_group_msg(group_id, {
    bot.text("请看这张图："),
    bot.image("https://picsum.photos/400/300"),
})
```

## AT 某人

```lua
bot.send_group_msg(group_id, {
    bot.at(user_id),
    bot.text(" 你好！"),
})
```

## 发送语音

```lua
bot.send_private_msg(user_id, {
    bot.record("https://example.com/audio.mp3"),
})
```

## 消息段速查

| 函数 | 用途 | 示例 |
|------|------|------|
| `bot.text("内容")` | 文字 | `bot.text("你好")` |
| `bot.image("地址")` | 图片 | `bot.image("https://...")` |
| `bot.record("地址")` | 语音 | `bot.record("https://...")` |
| `bot.video("地址")` | 视频 | `bot.video("https://...")` |
| `bot.at(QQ号)` | @人 | `bot.at(123456)` |
| `bot.reply(消息ID)` | 回复消息 | `bot.reply(msg_id)` |
| `bot.forward("合并转发ID")` | 引用已有合并转发 | `bot.forward("msg_id")` |
| `bot.node(QQ号, "昵称", 内容)` | 合并转发节点 | 见下方示例 |
| `bot.segment("类型", data)` | 自定义 | `bot.segment("face", {id="178"})` |

## 合并转发

```lua
-- 自定义合并转发（伪造聊天记录）
bot.send_group_msg(group_id, {
    bot.node(123456, "Alice", "今天天气真好"),
    bot.node(789012, "Bob", bot.text("是啊")),
    bot.node(123456, "Alice", {
        bot.text("发张图看看："),
        bot.image("https://example.com/photo.jpg"),
    }),
})
```

## 回复消息

```lua
bot.send_group_msg(event.group_id, {
    bot.reply(event.message_id),
    bot.text("收到"),
})
```

## 群管理

```lua
-- 踢人（reject_add_request 可选，默认 false，用于拒绝加群请求）
bot.set_group_kick(group_id, user_id)
bot.set_group_kick(group_id, user_id, true)

-- 禁言（秒，0 = 解禁）
bot.set_group_ban(group_id, user_id, 60)      -- 禁言 1 分钟
bot.set_group_ban(group_id, user_id, 3600)    -- 禁言 1 小时
bot.set_group_ban(group_id, user_id, 0)       -- 解禁

-- 全员禁言
bot.set_group_whole_ban(group_id, true)        -- 开启
bot.set_group_whole_ban(group_id, false)       -- 关闭

-- 管理员
bot.set_group_admin(group_id, user_id, true)   -- 设为管理
bot.set_group_admin(group_id, user_id, false)  -- 取消管理

-- 群名片（群备注）
bot.set_group_card(group_id, user_id, "警告：违规发言")

-- 群名
bot.set_group_name(group_id, "新群名")

-- 专属头衔（-1 = 永久）
bot.set_group_special_title(group_id, user_id, "VIP", -1)

-- 退群（is_dismiss 可选，仅群主可解散）
bot.set_group_leave(group_id)
bot.set_group_leave(group_id, true)            -- 退群并解散

-- 好友赞（次数默认 1）
bot.send_like(user_id)
bot.send_like(user_id, 10)
```

| **所有群管理 API 需要相应权限（群主/管理员），否则返回 error。** |

## 完整示例

```lua
return {
    on_group_message = function(event)
        local msg = event.raw_message

        if msg == "你好" then
            bot.send_group_msg(event.group_id, {
                bot.at(event.user_id),
                bot.text(" 你好呀~"),
            })
        end

        if msg == "发图" then
            bot.send_group_msg(event.group_id, {
                bot.image("https://picsum.photos/400/300"),
            })
        end

        if msg == "天气" then
            bot.send_group_msg(event.group_id, "今天天气不错！")
        end
    end,
}
```
