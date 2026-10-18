# 数据库（SQLite / MySQL）

Quill 内置 SQL 支持，插件通过 `bot.sql` 直接读写数据库，无需额外安装。

## 启用

在 `.env` 中配置：

```env
# SQLite：默认写入 data/bot.db
ONEBOT_SQL_DRIVER=sqlite

# 或 MySQL（需要自己提供 DSN）
# ONEBOT_SQL_DRIVER=mysql
# ONEBOT_SQL_DSN=user:pass@tcp(127.0.0.1:3306)/quill?parseTime=true
```

不配置 `ONEBOT_SQL_DRIVER` 时 `bot.sql` 不存在，插件应先判断：

```lua
if not bot.sql then
    return
end
```

## bot.sql.exec

执行 INSERT / UPDATE / DELETE，返回受影响行数与自增 ID。

```lua
local affected, last_id = bot.sql.exec(
    "INSERT INTO points (qq, score) VALUES (?, ?)",
    { event.user_id, 10 }
)
```

## bot.sql.query

查询多行，返回数组表（每行一个 table）。

```lua
local rows = bot.sql.query("SELECT qq, score FROM points ORDER BY score DESC LIMIT ?", {10})
for _, row in ipairs(rows) do
    util.Log("INFO", row.qq .. " => " .. row.score)
end
```

## bot.sql.query_row

只取第一行，没有结果返回 `nil`。

```lua
local row = bot.sql.query_row("SELECT score FROM points WHERE qq = ?", {event.user_id})
local score = row and row.score or 0
```

## NULL 的处理

Lua 数组里的 `nil` 会截断长度，不能直接写 `{1, nil, 3}`。
用 `bot.sql.NULL` 表示 SQL NULL：

```lua
bot.sql.exec("INSERT INTO t (a, b, c) VALUES (?, ?, ?)", {1, bot.sql.NULL, 3})
```

查询结果中的 NULL 会变成 Lua 的 `nil`。

## 事务

`bot.sql.tx(fn)` 传入 tx 对象；正常结束提交，抛错则回滚。

```lua
local ok, err = bot.sql.tx(function(tx)
    tx.exec("UPDATE accounts SET balance = balance - ? WHERE id = ?", {100, 1})
    tx.exec("UPDATE accounts SET balance = balance + ? WHERE id = ?", {100, 2})
end)
if not ok then
    util.Log("ERROR", "转账失败: " .. err)
end
```

## 建表示例

```lua
bot.sql.exec([[
    CREATE TABLE IF NOT EXISTS points (
        qq    INTEGER PRIMARY KEY,
        score INTEGER NOT NULL DEFAULT 0
    )
]])
```

## 注意

- SQL 在事件预算内执行（最长 5 秒），慢查询会被中断
- 参数一律用 `?` 占位，不要拼接字符串，避免 SQL 注入
- SQLite 默认开启 WAL 与 `busy_timeout(5000)`，并使用单连接
- MySQL 需要自行保证网络可达与账号权限
