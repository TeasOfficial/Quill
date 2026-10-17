# bot API 参考

`bot` 是全局可用的机器人 API 表，所有插件中直接使用。

---

## 消息发送

### bot.send_group_msg

发送群消息。

```lua
bot.send_group_msg(group_id, content)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 目标群号 |
| `content` | string 或 table | 纯文本或消息段数组 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | number | 消息 ID（message_id） |
| 失败 | nil, string | nil + 错误信息 |

```lua
-- 纯文本
bot.send_group_msg(123456789, "Hello")

-- 消息段数组
bot.send_group_msg(123456789, {
    bot.at(123456),
    bot.text(" 你好"),
    bot.image("https://example.com/pic.jpg"),
})
```

---

### bot.send_private_msg

发送私聊消息。

```lua
bot.send_private_msg(user_id, content)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `user_id` | number | 目标 QQ 号 |
| `content` | string 或 table | 纯文本或消息段数组 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | number | 消息 ID |
| 失败 | nil, string | nil + 错误信息 |

---

### bot.send_msg

通用发送，通过 `message_type` 区分群/私聊。

```lua
bot.send_msg(message_type, id, content)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `message_type` | string | `"group"` 或 `"private"` |
| `id` | number | 群号（group）或 QQ 号（private） |
| `content` | string 或 table | 纯文本或消息段数组 |

```lua
bot.send_msg("group", 123456789, "群消息")
bot.send_msg("private", 123456, "私聊消息")
```

---

## 消息管理

### bot.delete_msg

撤回消息。

```lua
bot.delete_msg(message_id)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

```lua
-- 先发后撤模式
local msg_id = bot.send_group_msg(group_id, "这条消息 3 秒后消失")
util.After(3, function()
    bot.delete_msg(msg_id)
end)
```

---

### bot.get_msg

查询单条消息内容。

```lua
bot.get_msg(message_id)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | JSON 格式的消息数据 |
| 失败 | nil, string | nil + 错误信息 |

---

### bot.get_forward_msg

查询合并转发消息内容。

```lua
bot.get_forward_msg(forward_id)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `forward_id` | string | 转发消息 ID |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | JSON 格式的转发数据 |
| 失败 | nil, string | nil + 错误信息 |

---

## 群管理

### bot.set_group_kick

踢出群成员。

```lua
bot.set_group_kick(group_id, user_id, reject_add_request?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `user_id` | number | 被踢者 QQ |
| `reject_add_request` | boolean | 可选，是否拒绝此人再次加群请求，默认 `false` |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

> 需要管理员权限。

---

### bot.set_group_ban

禁言 / 解禁群成员。

```lua
bot.set_group_ban(group_id, user_id, duration)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `user_id` | number | 被禁言者 QQ |
| `duration` | number | 禁言时长（秒），`0` = 解禁 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

```lua
bot.set_group_ban(gid, uid, 60)     -- 禁言 1 分钟
bot.set_group_ban(gid, uid, 3600)   -- 禁言 1 小时
bot.set_group_ban(gid, uid, 0)      -- 解禁
```

> 需要管理员权限。

---

### bot.set_group_whole_ban

全员禁言。

```lua
bot.set_group_whole_ban(group_id, enable)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `enable` | boolean | `true` = 开启，`false` = 关闭 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

> 需要群主或管理员权限。

---

### bot.set_group_admin

设 / 撤群管理员。

```lua
bot.set_group_admin(group_id, user_id, enable)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `user_id` | number | 目标 QQ |
| `enable` | boolean | `true` = 设为管理，`false` = 取消管理 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

> 需要群主权限。

---

### bot.set_group_card

修改群成员的群名片（群备注）。

```lua
bot.set_group_card(group_id, user_id, card)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `user_id` | number | 目标 QQ |
| `card` | string | 新群名片，空串 = 清除 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

> 需要管理员权限。

---

### bot.set_group_name

修改群名称。

```lua
bot.set_group_name(group_id, name)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `name` | string | 新群名 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

> 需要群主权限。

---

### bot.set_group_special_title

设置群专属头衔。

```lua
bot.set_group_special_title(group_id, user_id, title, duration)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `user_id` | number | 目标 QQ |
| `title` | string | 专属头衔（最长 6 字） |
| `duration` | number | 有效期（秒），`-1` = 永久 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

> 需要群主权限。

---

### bot.set_group_leave

退出或解散群组。

```lua
bot.set_group_leave(group_id, is_dismiss?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `is_dismiss` | boolean | 可选，是否解散群（仅群主），默认 `false` |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

> 退出后需重新邀请才能回来。解散仅群主可操作。

---

### bot.send_like

给好友点赞。

```lua
bot.send_like(user_id, times?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `user_id` | number | 好友 QQ |
| `times` | number | 可选，点赞次数，默认 `1`，上限由平台决定 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

---

## 消息段构造器

### bot.text

`{type="text", data={text=text}}`

### bot.image

`{type="image", data={file=url}}`

### bot.record

`{type="record", data={file=url}}`

### bot.video

`{type="video", data={file=url}}`

### bot.at

```lua
bot.at(user_id)
-- → {type="at", data={qq="123456"}}
```

### bot.reply

```lua
bot.reply(message_id)
-- → {type="reply", data={id="123456"}}
```

### bot.forward

```lua
bot.forward(forward_id)
-- → {type="forward", data={id="forward_id"}}
```

引用已有的合并转发消息。

### bot.node

```lua
bot.node(user_id, nickname, content)
-- → {type="node", data={user_id="...", nickname="...", content=...}}
```

合并转发中的一条消息。`content` 可以是字符串或消息段数组。

### bot.segment

```lua
bot.segment("type", data_table)
-- → {type="type", data={...}}
```

自定义消息段。

---

## 持久存储

### bot.db.set

```lua
bot.db.set(key, value)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `key` | string | 存储键，建议 `"namespace/subkey"` 格式 |
| `value` | any | 任意值（table 支持嵌套） |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | nil, string | nil + 错误信息 |

> 数据持久化到 `data/{key}.json`。

### bot.db.get

```lua
bot.db.get(key)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | any | 存储的值 |
| 不存在 | nil | 键不存在 |

### bot.db.del

```lua
bot.db.del(key)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | nil, string | nil + 错误信息 |

---

## 文件 I/O

所有路径相对于 `data/` 目录，禁止 `..` 和符号链接。

### bot.file.read

```lua
bot.file.read(path)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | string | 文件内容 |
| 失败 | nil, string | nil + 错误信息 |

### bot.file.write

```lua
bot.file.write(path, content)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `path` | string | 文件路径（自动创建父目录） |
| `content` | string | 文件内容 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

### bot.file.exists

```lua
bot.file.exists(path)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 存在 | boolean | `true` |
| 不存在 | boolean | `false` |

### bot.file.delete

```lua
bot.file.delete(path)
```

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | boolean | `true` |
| 失败 | boolean, string | `false` + 错误信息 |

### bot.file.list

```lua
bot.file.list(dir?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `dir` | string | 可选，子目录路径，默认列出 `data/` 根 |

| 返回 | 类型 | 说明 |
|------|------|------|
| 成功 | table | 文件和目录名数组 `{"a.txt", "subdir", ...}` |
| 失败 | nil, string | nil + 错误信息 |

> ⚠️ 设置 `ONEBOT_FILE_UNSAFE=true` 后，以上所有路径限制解除，可读写任意路径。

---

## 请求处理

### bot.set_friend_add_request

同意或拒绝好友请求。

```lua
bot.set_friend_add_request(flag, approve, remark?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `flag` | string | 请求标识，从 `on_friend_request` 事件的 `event.flag` 获取 |
| `approve` | boolean | `true` = 同意，`false` = 拒绝 |
| `remark` | string | 可选，同意时的好友备注 |

```lua
on_friend_request = function(event)
    bot.set_friend_add_request(event.flag, true, "来自群聊")
end,
```

### bot.set_group_add_request

同意或拒绝加群请求 / 邀请。

```lua
bot.set_group_add_request(flag, sub_type, approve, reason?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `flag` | string | 请求标识，从 `on_group_request` 事件的 `event.flag` 获取 |
| `sub_type` | string | `"add"` = 加群请求，`"invite"` = 邀请入群 |
| `approve` | boolean | `true` = 同意，`false` = 拒绝 |
| `reason` | string | 可选，拒绝时的理由 |

```lua
on_group_request = function(event)
    if event.sub_type == "add" then
        bot.set_group_add_request(event.flag, "add", true)
    end
end,
```

---

## 账号查询

### bot.get_login_info

获取当前登录账号信息。

```lua
bot.get_login_info()
```

| 返回 | 类型 | 说明 |
|------|------|------|
| string | JSON | `{"user_id":123456,"nickname":"Bot"}` |

### bot.get_friend_list

获取好友列表。

```lua
bot.get_friend_list()
```

返回 JSON 数组，每条含 `user_id`、`nickname`、`remark`。

### bot.get_stranger_info

查询陌生人信息。

```lua
bot.get_stranger_info(user_id, no_cache?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `user_id` | number | QQ 号 |
| `no_cache` | boolean | 可选，`true` = 跳过缓存直接请求 |

返回 JSON：`{"user_id":...,"nickname":"...","sex":"male","age":20}`。

---

## 群查询

### bot.get_group_list

获取机器人加入的群列表。

```lua
bot.get_group_list()
```

返回 JSON 数组，每条含 `group_id`、`group_name`。

### bot.get_group_info

获取群详细信息。

```lua
bot.get_group_info(group_id, no_cache?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `group_id` | number | 群号 |
| `no_cache` | boolean | 可选，跳过缓存 |

### bot.get_group_member_info

获取群成员详细信息。

```lua
bot.get_group_member_info(group_id, user_id, no_cache?)
```

返回 JSON：`{"group_id":...,"user_id":...,"nickname":"...","card":"...","role":"admin", ...}`。

### bot.get_group_member_list

获取群成员完整列表。

```lua
bot.get_group_member_list(group_id)
```

返回 JSON 数组。

### bot.get_group_honor_info

获取群荣誉（龙王、群聊之火等）。

```lua
bot.get_group_honor_info(group_id, type?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `type` | string | 可选，`"talkative"` `"performer"` `"emotion"` `"all"`，默认 `"all"` |

---

## 能力检查 & 资源获取

### bot.can_send_image / bot.can_send_record

检查当前是否允许发送图片/语音。

```lua
bot.can_send_image()   -- → {"yes":true}
bot.can_send_record()  -- → {"yes":false}
```

### bot.get_image

获取图片的下载链接或本地路径。

```lua
bot.get_image(file)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `file` | string | 图片文件名或 URL |

### bot.get_record

获取语音文件。

```lua
bot.get_record(file, out_format?)
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `file` | string | 语音文件名 |
| `out_format` | string | 可选，输出格式，默认 `"mp3"` |

### bot.get_cookies / bot.get_csrf_token / bot.get_credentials

获取 QQ 登录凭证（极少使用）。

```lua
bot.get_cookies("domain.com")
bot.get_csrf_token()
bot.get_credentials("domain.com")
```
