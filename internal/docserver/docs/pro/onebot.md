# OneBot v11 协议实现

## HTTP API

```
POST {base_url}/{action}
Content-Type: application/json
Authorization: Bearer {token}
```

已实现接口：

| action | Go 方法 | 参数 |
|--------|---------|------|
| `send_group_msg` | `SendGroupMsg(id, msg)` | group_id, message |
| `send_private_msg` | `SendPrivateMsg(id, msg)` | user_id, message |

## WebSocket 事件接收

### forward 模式

Bot 作为 WS 客户端连接 OneBot：

```go
dialer := websocket.Dialer{HandshakeTimeout: 5s}
conn, _ := dialer.Dial(wsURL, headers)
// 循环读取 JSON 事件
```

`wsURL` 由 `ONEBOT_WS_URL` 指定，`Authentication` 头传 token。

### reverse 模式

Bot 作为 WS 服务端，OneBot 连接过来。同时支持 HTTP POST 事件上报：

```go
mux.HandleFunc(path, func(w, r) {
    if r.Header("Upgrade") == "websocket" {
        conn, _ := upgrader.Upgrade(w, r, nil)
        r.Listen()   // WS 循环
    }
    if r.Method == "POST" {
        json.NewDecoder(r.Body).Decode(&event)
        r.OnEvent(&event)  // HTTP 事件
    }
})
```

## 事件类型

```go
type Event struct {
    Time        int64
    PostType    string   // message, notice, request, meta_event
    MessageType string   // group, private
    GroupID     int64
    UserID      int64
    RawMessage  string
    Sender      Sender
}
```

## 配置

| 环境变量 | 说明 |
|----------|------|
| `ONEBOT_BASE_URL` | HTTP API 地址 |
| `ONEBOT_WS_URL` | WS 地址（默认同 BASE_URL） |
| `ONEBOT_TOKEN` | 认证令牌 |
| `ONEBOT_WS_MODE` | `forward` / `reverse` |
| `ONEBOT_WS_PATH` | WS 路径 |
| `ONEBOT_WS_LISTEN` | reverse 模式监听地址 |
