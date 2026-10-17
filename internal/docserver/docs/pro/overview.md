# 架构总览

## 组件树
```
Quill.exe
├─ OneBot v11 (HTTP + WebSocket)
│   ├─ HTTP API → http://localhost:3007
│   └─ WS Event → ws://localhost:12006
├─ gopher-lua 插件引擎
│   ├─ plugins/*.lua
│   └─ util 内置工具集 (HTTP/下载/解压/加密/编码等)
└─ Wiki 文档服务 (:3081)
   └─ 内嵌 docs/ (goldmark 渲染)
```

## 目录结构

```
release/
├── Quill.exe              # 单文件可执行
├── .env                   # 启动配置
└── plugins/               # Lua 插件
```

## 启动流程

1. `main.go` 加载 `.env` 配置
2. 初始化 OneBot HTTP 客户端 + WebSocket 接收端
3. 加载 `plugins/*.lua`
4. 连接 OneBot WebSocket，开始接收事件
5. 启动 Wiki 服务 `:3081`

## 依赖

| 库 | 用途 |
|----|------|
| `gorilla/websocket` | OneBot WebSocket |
| `yuin/gopher-lua` | Lua 5.1 虚拟机 |
| `yuin/goldmark` | Markdown 渲染 |
| `joho/godotenv` | .env 加载 |
