# Quill 文档

Go 宿主 + Lua 插件的 OneBot v11 QQ 机器人。

## 快速开始
编辑 `.env` 配置，双击 `Quill.exe` 即可运行。
```env
ONEBOT_BASE_URL=http://localhost:3007
ONEBOT_WS_URL=http://localhost:12006
ONEBOT_TOKEN=your_token_here
ONEBOT_WS_MODE=forward
```

然后在 `plugins/` 目录下放 `.lua` 文件写插件。

## 内置能力

| 组件 | 功能 |
|------|------|
| `bot` | 消息收发、群管理、事件处理 |
| `util` | JSON / Base64 / 哈希 / AES / UUID / URL 编码 / 定时器 |
| `util.xxx()` | 内置工具集（HTTP 请求、多线程下载、解压等） |
| `bot.db` | 键值持久化存储 |
| `bot.sql` | SQLite / MySQL（配置 `ONEBOT_SQL_DRIVER` 后启用） |
| `bot.file` | 文件读写（沙箱内） |
| Wiki | :3081 内嵌文档 |

<div class="basic-only">

## 从哪里开始
如果你是第一次接触，先看 **basic: quickstart** — 5 分钟写出第一个插件。
</div>

<div class="pro-only">

## 架构概览

```
plugins/*.lua          Lua 插件（业务逻辑）
    ↓
internal/
    onebot/             OneBot v11 HTTP + WebSocket
    plugin/             gopher-lua 插件引擎 + util 工具集
    storage/            JSON 键值持久化
    database/           SQLite / MySQL 封装（bot.sql）
    config/             .env 配置加载
    docserver/          内嵌 Wiki 文档服务
    logc/               彩色终端日志
```

</div>
