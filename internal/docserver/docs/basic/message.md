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
