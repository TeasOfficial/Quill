# 解压模块

支持 zip / tar / gz / bz2 格式解压。

## Lua 调用

```lua
local ex = module("extract")

local r = ex.extract('{"file":"downloads/tools.zip","dest":"downloads/tools"}')
-- → {"ok":true,"dest":"downloads/tools","count":42}
```

## 支持格式

| 格式 | 扩展名 |
|------|--------|
| ZIP | `.zip` |
| TAR | `.tar` |
| TAR.GZ | `.tar.gz` `.tgz` |
| TAR.BZ2 | `.tar.bz2` `.tbz2` |
| GZ | `.gz` |
| BZ2 | `.bz2` |

自动防路径穿越（zip slip attack）。
