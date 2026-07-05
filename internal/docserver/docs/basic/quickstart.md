# 快速开始
## 五分钟写出第一个插件
### 1. 配置

编辑 `Quill.exe` 旁边的 `.env` 文件：
```env
ONEBOT_BASE_URL=http://localhost:3007
ONEBOT_WS_URL=http://localhost:12006
ONEBOT_TOKEN=your_token_here
ONEBOT_WS_MODE=forward
```

### 2. 创建插件

在 `plugins/` 目录下新建 `hello.lua`：
```lua
return {
    on_group_message = function(event)
        if event.raw_message == "你好" then
            bot.send_group_msg(event.group_id, "你也好呀~")
        end
    end,
}
```

### 3. 运行

双击 `Quill.exe`。在 QQ 群里发"你好"，机器人会回复"你也好呀~"。
## 插件可以做什么
能响应的消息：
```lua
return {
    -- 群消息
    on_group_message = function(event)
        if event.raw_message == "ping" then
            bot.send_group_msg(event.group_id, "pong")
        end
    end,
    -- 私聊消息
    on_private_message = function(event)
        bot.send_private_msg(event.user_id, "收到了")
    end,
    -- 新人入群
    on_group_increase = function(event)
        bot.send_group_msg(event.group_id, "欢迎新人~")
    end,
}
```

## 可用数据

`event` 是一个表，常用字段：

| 字段 | 含义 |
|------|------|
| `event.group_id` | 群号 |
| `event.user_id` | 发送者 QQ |
| `event.raw_message` | 消息原文 |
| `event.sender.nickname` | 发送者昵称 |

## 发消息
```lua
bot.send_group_msg(群号, "内容")
bot.send_private_msg(QQ号, "内容")
```

## 下一步
- [消息发送](message) — 文字、图片、语音、回复、合并转发
- [插件开发](plugin) — 事件类型、util 工具集、WASM 模块调用
- [数据存储](storage) — bot.db 持久化 + bot.file 文件 I/O
