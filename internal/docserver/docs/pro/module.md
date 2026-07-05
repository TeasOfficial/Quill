# WASM 模块开发

## 概述

WASM 模块用 Go 编写，编译为 `.wasm` 后由 bot 动态加载。

## 模块文件

```
modules/my_module/
├── module.json
├── main.go
└── my_module.wasm    (自动生成)
```

## module.json

```json
{
    "name": "http",
    "description": "HTTP 请求模块",
    "version": "1.0.0",
    "author": "你的名字"
}
```

## main.go

```go
package main

import "unsafe"

func main() {}

//go:wasmexport malloc
func malloc(size uint32) *byte {
    buf := make([]byte, size)
    return &buf[0]
}

//go:wasmexport my_func
func my_func(ptr uint32, len uint32) uint64 {
    input := readString(ptr, len)
    result := process(input)
    return writeString(result)
}

func readString(ptr, len uint32) string {
    p := (*byte)(unsafe.Pointer(uintptr(ptr)))
    return string(unsafe.Slice(p, len))
}

func writeString(s string) uint64 {
    b := []byte(s)
    p := malloc(uint32(len(b)))
    offset := uint32(uintptr(unsafe.Pointer(p)))
    copy(unsafe.Slice(p, uint32(len(b))), b)
    return uint64(offset)<<32 | uint64(len(b))
}
```

## 函数签名

```
export fn(inputPtr uint32, inputLen uint32) uint64
    返回: (resultPtr << 32) | resultLen
```

参数和返回值都通过 WASM 线性内存传递。

## 必须导出的函数

| 函数 | 说明 |
|------|------|
| `malloc(size) *byte` | 内存分配，Host 调用 |
| 业务函数 | 任意数量，每个导出一个功能 |

## 自动编译

修改 `main.go` 后：

1. fsnotify 检测变更（1.5s 去抖）
2. 执行 `GOOS=wasip1 GOARCH=wasm go build -o name.wasm`  
3. 卸载旧模块，加载新 `.wasm`

编译约 2-4 秒，不影响其他模块和插件。

## 运行原理

```go
// Host 加载模块
wasmMod := runtime.Instantiate(ctx, wasmBytes)
malloc := wasmMod.ExportedFunction("malloc")
fn := wasmMod.ExportedFunction("get")

// 调用: 在模块内存中写入参数，调用函数，读取结果
argPtr := malloc(len(data))
mem.Write(argPtr, data)
result := fn.Call(argPtr, len(data))
resultBytes := mem.Read(resultPtr, resultLen)
```

## Lua 调用

```lua
local mod = module("http")  -- 按 module.json 中的 name
if mod then
    local result = mod.get("https://api.example.com")
end
```

Host 通过 gopher-lua 闭包绑定：`module("http")` 返回一个 Lua table，每个导出函数对应一个闭包，闭包内部调用 `mod.Call(funcName, arg)`。

## Host API 回调

WASM 模块可以通过 `//go:wasmimport quill <function>` 调用 Go 宿主提供的功能：

| 回调函数 | 用途 |
|----------|------|
| `http_request(ptr, len) uint64` | HTTP 请求（GET/POST/自定义） |
| `http_download(ptr, len) uint64` | 多线程文件下载 |
| `archive_extract(ptr, len) uint64` | 解压 (zip/tar/gz/bz2) |

输入输出均为 JSON，通过 WASM 线性内存传递。

```go
//go:wasmimport quill http_download
func hostDownload(reqPtr uint32, reqLen uint32) uint64

//go:wasmexport dl
func dl(reqPtr uint32, reqLen uint32) uint64 {
    return hostDownload(reqPtr, reqLen)
}
```

## 模块文档

在模块目录下放置 `README.md`，启动时自动注册到 Wiki 侧边栏"模块文档"分组。其他 `.md` 文件以子路径注册。
