# 数据持久化

## 概述

Bot 提供简单的键值存储 API，数据保存为 JSON 文件。适合插件保存用户数据、游戏进度、配置等。

存储位置：`data/` 目录，按命名空间分文件。

## API

| 方法 | 说明 |
|------|------|
| `bot.db.set(key, value)` | 保存数据（自动覆盖） |
| `bot.db.get(key)` | 读取数据，不存在返回 `nil` |
| `bot.db.del(key)` | 删除数据 |

- `key` 是字符串，建议用 `命名空间/子键` 格式，如 `"game/player_123"`
- `value` 是 Lua 表（支持嵌套），自动序列化为 JSON
- 数据持久化到 `data/{key}.json`

## 示例：记账本

```lua
-- 存钱
local user = event.user_id
local db = bot.db

local account = db.get("money/" .. user) or { balance = 0 }
account.balance = account.balance + 100
db.set("money/" .. user, account)

bot.send_group_msg(event.group_id, "存了 100，当前余额 " .. account.balance)
```

## 示例：猜数字游戏

```lua
return {
    on_group_message = function(event)
        local db = bot.db
        local uid = event.user_id .. "_" .. event.group_id
        local game = db.get("guess/" .. uid)

        if event.raw_message == "/guess" then
            game = {
                target = math.random(1, 100),
                tries = 0,
                best = (game and game.best) or 999,
            }
            db.set("guess/" .. uid, game)
            bot.send_group_msg(event.group_id, {
                bot.at(event.user_id),
                bot.text(" 猜数字游戏开始！1~100，最好成绩: " .. game.best),
            })
            return
        end

        if game and game.target then
            local num = tonumber(event.raw_message)
            if not num then return end

            game.tries = game.tries + 1
            if num < game.target then
                bot.send_group_msg(event.group_id, "太小了！")
            elseif num > game.target then
                bot.send_group_msg(event.group_id, "太大了！")
            else
                if game.tries < game.best then
                    game.best = game.tries
                end
                bot.send_group_msg(event.group_id, {
                    bot.at(event.user_id),
                    bot.text(" 猜对了！用了 " .. game.tries .. " 次，最好成绩: " .. game.best),
                })
                game.target = nil
            end
            db.set("guess/" .. uid, game)
        end
    end,
}
```

## 存储结构

```
data/
├── guess/
│   ├── 123456_671812808.json    # QQ号_群号
│   └── 789012_671812808.json
├── money/
│   ├── 123456.json
│   └── 789012.json
└── config/
    └── global.json
```

## 注意事项

- 数据文件在 `data/` 目录下，不会自动清理
- key 中不能包含 `..`（防目录穿越）
- 所有数据明文存储，敏感信息请自行加密
- 插件间可以通过 key 共享数据（约定命名空间即可）

## 文件 I/O

Bot 提供受限的文件读写 API，所有操作限定在 `data/` 目录内。

| 方法 | 说明 |
|------|------|
| `bot.file.read(path)` | 读取文件，返回字符串；不存在返回 `nil, err` |
| `bot.file.write(path, content)` | 写入文件，自动创建父目录 |
| `bot.file.exists(path)` | 检查文件是否存在，返回 `true/false` |
| `bot.file.delete(path)` | 删除文件 |
| `bot.file.list(dir)` | 列出目录下的文件/目录名，返回数组；无参数时列出 `data/` |

路径相对于 `data/` 目录，禁止 `..` 穿越。

```lua
-- 下载文件后写入
local http = module("http")
local resp = http.get("https://api.example.com/data.json")
bot.file.write("cache/api_data.json", resp)

-- 读取缓存
local cached = bot.file.read("cache/api_data.json")

-- 列出文件
local files = bot.file.list("cache")
for _, f in ipairs(files) do
    print(f)
end
```
