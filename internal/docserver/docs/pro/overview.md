# 架构总览

## 组件树
```
Quill.exe
├─ OneBot v11 (HTTP + WebSocket)
│   ├─ HTTP API → http://localhost:3007
│   └─ WS Event → ws://localhost:12006
├─ gopher-lua 插件引擎
│   └─ plugins/*.lua
├─ WASM 运行时 (wazero)
│   ├─ http.wasm           # HTTP 请求
│   ├─ download.wasm       # 多线程下载
│   ├─ extract.wasm        # 解压 (zip/tar/gz/bz2)
│   └─ fsnotify 热重载
└─ Wiki 文档服务 (:3081)
   └─ 内嵌 docs/ (goldmark 渲染)
```

## 目录结构

```
release/
├── Quill.exe              # 单文件可执行
├── .env                   # 启动配置
├── plugins/               # Lua 插件
└── modules/               # WASM 模块（编译产物）
    └── http/
        ├── http.wasm
        └── module.json
```

## 启动流程

1. `main.go` 加载 `.env` 配置
2. 初始化 OneBot HTTP 客户端 + WebSocket 接收端
3. 初始化 WASM 运行时，编译并加载 `modules/` 下的所有 `.wasm`
4. 启动 fsnotify 监听 `modules/**/*.go`，检测变更自动重编译热载
5. 加载 `plugins/*.lua`
6. 连接 OneBot WebSocket，开始接收事件
7. 启动 Wiki 服务 `:3081`

## 依赖

| 库 | 用途 |
|----|------|
| `gorilla/websocket` | OneBot WebSocket |
| `yuin/gopher-lua` | Lua 5.1 虚拟机 |
| `tetratelabs/wazero` | WASM 运行时 |
| `fsnotify` | 文件监听 |
| `yuin/goldmark` | Markdown 渲染 |
| `joho/godotenv` | .env 加载 |
