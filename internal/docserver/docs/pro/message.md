# 消息段完整规范

消息可以是一个**字符串**或**消息段数组**。内部实现：字符串直接发送，数组序列化为 OneBot 标准消息段格式。

```go
func (c *Client) SendGroupMsg(groupID int64, message interface{}) (int64, error)
```

`interface{}` 接受 `string` 或 `[]map[string]interface{}`。

## Lua API

### 消息发送

```lua
-- 字符串
bot.send_group_msg(id, "text")

-- 消息段数组
bot.send_group_msg(id, { seg1, seg2, ... })
```

### 消息段构造器

| 函数 | 返回类型 | OneBot 输出 |
|------|----------|-------------|
| `bot.text(t)` | `{type="text", data={text=t}}` | 纯文本 |
| `bot.image(f)` | `{type="image", data={file=f}}` | 图片 |
| `bot.record(f)` | `{type="record", data={file=f}}` | 语音 |
| `bot.video(f)` | `{type="video", data={file=f}}` | 视频 |
| `bot.at(qq)` | `{type="at", data={qq=qq}}` | @某人 |
| `bot.segment(typ, data)` | `{type=typ, data=data}` | 任意类型 |

### 数据传递路径

```
Lua table → luaValueToMsg() → []map[string]interface{} → json.Marshal → HTTP POST
```

`luaTable.ForEach` 遍历数组，对每个元素用 `luaValueToSimple` 转换基本类型（string/number/bool），嵌套 data 表同样处理。

## OneBot 消息段格式

```json
{
    "message": [
        { "type": "text",  "data": { "text": "Hello" } },
        { "type": "image", "data": { "file": "https://..." } },
        { "type": "at",    "data": { "qq": "123456" } }
    ]
}
```

## 示例

```lua
-- 图文混合
bot.send_group_msg(gid, {
    bot.at(uid),
    bot.text(" 查看："),
    bot.image("https://picsum.photos/400/300"),
})

-- 多图
bot.send_group_msg(gid, {
    bot.image("1.jpg"),
    bot.image("2.jpg"),
})

-- 自定义段
bot.send_group_msg(gid, {
    bot.segment("face", { id = "178" }),
    bot.text(" 好的"),
})
```
