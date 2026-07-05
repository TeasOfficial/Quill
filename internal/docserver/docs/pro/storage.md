# 数据持久化 — 实现细节

## 架构

```
Lua: bot.db.set("key", value)
  ↓ lua table → Go map
Go:  json.Marshal → data/key.json
```

## 存储规则

- key 作为文件路径：`data/{key}.json`
- 子目录自动创建，如 `data/game/player_1.json`
- key 安全检查：拒绝包含 `..` 的 key（防路径遍历）
- encoding/json 序列化，UTF-8

## Go 侧接口

```go
// Storage 绑定到 Lua bot 表
type Storage struct {
    dir string  // 默认 "data"
}

func (s *Storage) Set(key string, value lua.LValue) error
func (s *Storage) Get(key string) (lua.LValue, error)
func (s *Storage) Del(key string) error
```

## Lua 数据转换

```
Lua table   →  luaTable.ForEach →  map[string]interface{}  →  json.Marshal
Lua number  →  float64 (JSON number)
Lua string  →  string
Lua bool    →  bool
Lua nil     →  JSON null

读取时反向：json.Unmarshal → L.NewTable → lua table
```

## 目录结构

```
data/
├── namespace1/
│   ├── key1.json
│   └── key2.json
└── namespace2/
    └── key.json
```

由 `bot.db.set("namespace1/key1", data)` 自动创建。

## 性能

- 每次读写是完整 JSON 文件 I/O
- 适合小数据（< 1MB），不适合高频写入
- 高并发场景用 WASM 模块 + SQLite 替代
