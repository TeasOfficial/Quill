package plugin

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"quill/internal/logc"
	"quill/internal/onebot"
	"quill/internal/storage"

	lua "github.com/yuin/gopher-lua"
)

type Manager struct {
	mu         sync.Mutex
	L          *lua.LState
	client     *onebot.Client
	registry   *lua.LTable
	db         *storage.DB
	fileUnsafe bool
	timers     []timerEntry
	timerSeq   int64
}

type timerEntry struct {
	id       int64
	deadline time.Time
	fn       *lua.LTable
}

func NewManager(client *onebot.Client, db *storage.DB, fileUnsafe bool) *Manager {
	L := lua.NewState()
	registry := L.NewTable()
	L.SetGlobal("__plugin_registry", registry)

	m := &Manager{
		L:          L,
		client:     client,
		registry:   registry,
		db:         db,
		fileUnsafe: fileUnsafe,
	}
	m.registerBotAPI()
	return m
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.L.Close()
}

func (m *Manager) registerBotAPI() {
	L := m.L
	bot := L.NewTable()

	L.SetField(bot, "send_group_msg", L.NewFunction(func(L *lua.LState) int {
		groupID := L.CheckInt64(1)
		msg := luaValueToMsg(L.Get(2))
		msgID, err := m.client.SendGroupMsg(groupID, msg)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LNumber(msgID))
		return 1
	}))

	L.SetField(bot, "send_private_msg", L.NewFunction(func(L *lua.LState) int {
		userID := L.CheckInt64(1)
		msg := luaValueToMsg(L.Get(2))
		msgID, err := m.client.SendPrivateMsg(userID, msg)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LNumber(msgID))
		return 1
	}))

	L.SetField(bot, "delete_msg", L.NewFunction(func(L *lua.LState) int {
		msgID := L.CheckInt64(1)
		if err := m.client.DeleteMsg(msgID); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "send_msg", L.NewFunction(func(L *lua.LState) int {
		msgType := L.CheckString(1)
		var userID, groupID int64
		if msgType == "private" {
			userID = L.CheckInt64(2)
		} else {
			groupID = L.CheckInt64(2)
		}
		msg := luaValueToMsg(L.Get(3))
		msgID, err := m.client.SendMsg(msgType, userID, groupID, msg)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LNumber(msgID))
		return 1
	}))

	L.SetField(bot, "get_msg", L.NewFunction(func(L *lua.LState) int {
		msgID := L.CheckInt64(1)
		data, err := m.client.GetMsg(msgID)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))

	L.SetField(bot, "get_forward_msg", L.NewFunction(func(L *lua.LState) int {
		fwdID := L.CheckString(1)
		data, err := m.client.GetForwardMsg(fwdID)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))

	L.SetField(bot, "image", L.NewFunction(func(L *lua.LState) int {
		file := L.CheckString(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("image"))
		data := L.NewTable()
		data.RawSetString("file", lua.LString(file))
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "record", L.NewFunction(func(L *lua.LState) int {
		file := L.CheckString(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("record"))
		data := L.NewTable()
		data.RawSetString("file", lua.LString(file))
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "video", L.NewFunction(func(L *lua.LState) int {
		file := L.CheckString(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("video"))
		data := L.NewTable()
		data.RawSetString("file", lua.LString(file))
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "segment", L.NewFunction(func(L *lua.LState) int {
		typ := L.CheckString(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString(typ))
		if L.GetTop() >= 2 {
			seg.RawSetString("data", L.Get(2))
		}
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "text", L.NewFunction(func(L *lua.LState) int {
		text := L.CheckString(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("text"))
		data := L.NewTable()
		data.RawSetString("text", lua.LString(text))
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "at", L.NewFunction(func(L *lua.LState) int {
		qq := L.CheckInt64(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("at"))
		data := L.NewTable()
		data.RawSetString("qq", lua.LString(fmt.Sprint(qq)))
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "reply", L.NewFunction(func(L *lua.LState) int {
		msgID := L.CheckInt64(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("reply"))
		data := L.NewTable()
		data.RawSetString("id", lua.LString(fmt.Sprint(msgID)))
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "forward", L.NewFunction(func(L *lua.LState) int {
		fwdID := L.CheckString(1)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("forward"))
		data := L.NewTable()
		data.RawSetString("id", lua.LString(fwdID))
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	L.SetField(bot, "node", L.NewFunction(func(L *lua.LState) int {
		userID := L.CheckInt64(1)
		nickname := L.CheckString(2)
		content := L.Get(3)
		seg := L.NewTable()
		seg.RawSetString("type", lua.LString("node"))
		data := L.NewTable()
		data.RawSetString("user_id", lua.LString(fmt.Sprint(userID)))
		data.RawSetString("nickname", lua.LString(nickname))
		if tbl, ok := content.(*lua.LTable); ok {
			data.RawSetString("content", tbl)
		} else {
			data.RawSetString("content", content)
		}
		seg.RawSetString("data", data)
		L.Push(seg)
		return 1
	}))

	dbTable := L.NewTable()
	L.SetField(dbTable, "set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		val := L.Get(2)
		if err := m.db.Set(key, val); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(dbTable, "get", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		val, err := m.db.Get(L, key)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(val)
		return 1
	}))
	L.SetField(dbTable, "del", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		if err := m.db.Del(key); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(bot, "db", dbTable)

	fileTable := L.NewTable()
	L.SetField(fileTable, "read", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		safePath, err := m.safeFilePath(path)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		data, err := os.ReadFile(safePath)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(fileTable, "write", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		content := L.CheckString(2)
		safePath, err := m.safeFilePath(path)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		os.MkdirAll(filepath.Dir(safePath), 0755)
		if err := os.WriteFile(safePath, []byte(content), 0644); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(fileTable, "exists", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		safePath, err := m.safeFilePath(path)
		if err != nil {
			L.Push(lua.LFalse)
			return 1
		}
		_, err = os.Stat(safePath)
		L.Push(lua.LBool(err == nil))
		return 1
	}))
	L.SetField(fileTable, "delete", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		safePath, err := m.safeFilePath(path)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		if err := os.Remove(safePath); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(fileTable, "list", L.NewFunction(func(L *lua.LState) int {
		dir := ""
		if L.GetTop() >= 1 {
			dir = L.CheckString(1)
		}
		safeDir, err := m.safeFilePath(dir)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		entries, err := os.ReadDir(safeDir)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		tbl := L.NewTable()
		for i, e := range entries {
			tbl.RawSetInt(i+1, lua.LString(e.Name()))
		}
		L.Push(tbl)
		return 1
	}))
	L.SetField(bot, "file", fileTable)

	util := L.NewTable()
	L.SetField(util, "TableToJSON", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		goVal := luaValueToSimple(tbl)
		b, err := json.Marshal(goVal)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(b)))
		return 1
	}))
	L.SetField(util, "JSONToTable", L.NewFunction(func(L *lua.LState) int {
		raw := L.CheckString(1)
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(jsonToLua(L, v))
		return 1
	}))
	L.SetField(util, "PrintTable", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		L.Push(lua.LString(printLuaTable(tbl, 0)))
		return 1
	}))
	L.SetField(util, "CRC32", L.NewFunction(func(L *lua.LState) int {
		data := L.CheckString(1)
		crc := crc32.ChecksumIEEE([]byte(data))
		L.Push(lua.LString(fmt.Sprintf("%08X", crc)))
		return 1
	}))
	L.SetField(util, "Base64Encode", L.NewFunction(func(L *lua.LState) int {
		data := L.CheckString(1)
		L.Push(lua.LString(base64.StdEncoding.EncodeToString([]byte(data))))
		return 1
	}))
	L.SetField(util, "Base64Decode", L.NewFunction(func(L *lua.LState) int {
		data := L.CheckString(1)
		dec, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(dec)))
		return 1
	}))
	L.SetField(util, "MD5", L.NewFunction(func(L *lua.LState) int {
		h := md5.Sum([]byte(L.CheckString(1)))
		L.Push(lua.LString(fmt.Sprintf("%x", h)))
		return 1
	}))
	L.SetField(util, "SHA1", L.NewFunction(func(L *lua.LState) int {
		h := sha1.Sum([]byte(L.CheckString(1)))
		L.Push(lua.LString(fmt.Sprintf("%x", h)))
		return 1
	}))
	L.SetField(util, "SHA256", L.NewFunction(func(L *lua.LState) int {
		h := sha256.Sum256([]byte(L.CheckString(1)))
		L.Push(lua.LString(fmt.Sprintf("%x", h)))
		return 1
	}))
	L.SetField(util, "SHA512", L.NewFunction(func(L *lua.LState) int {
		h := sha512.Sum512([]byte(L.CheckString(1)))
		L.Push(lua.LString(fmt.Sprintf("%x", h)))
		return 1
	}))
	L.SetField(util, "IsUTF8", L.NewFunction(func(L *lua.LState) int {
		data := L.CheckString(1)
		L.Push(lua.LBool(utf8.Valid([]byte(data))))
		return 1
	}))
	L.SetField(util, "IsJSON", L.NewFunction(func(L *lua.LState) int {
		raw := L.CheckString(1)
		L.Push(lua.LBool(json.Valid([]byte(raw))))
		return 1
	}))
	L.SetField(util, "IsBase64", L.NewFunction(func(L *lua.LState) int {
		data := L.CheckString(1)
		_, err := base64.StdEncoding.DecodeString(data)
		L.Push(lua.LBool(err == nil))
		return 1
	}))
	L.SetField(util, "IsSafePath", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		_, err := safeDataPath(path)
		L.Push(lua.LBool(err == nil))
		return 1
	}))
	L.SetField(util, "IsValid", L.NewFunction(func(L *lua.LState) int {
		v := L.Get(1)
		L.Push(lua.LBool(!lua.LVIsFalse(v) && v.Type() != lua.LTNil))
		return 1
	}))
	L.SetField(util, "UUID", L.NewFunction(func(L *lua.LState) int {
		u := make([]byte, 16)
		rand.Read(u)
		u[6] = (u[6] & 0x0f) | 0x40
		u[8] = (u[8] & 0x3f) | 0x80
		L.Push(lua.LString(fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])))
		return 1
	}))
	L.SetField(util, "URLEncode", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(url.QueryEscape(L.CheckString(1))))
		return 1
	}))
	L.SetField(util, "URLDecode", L.NewFunction(func(L *lua.LState) int {
		dec, err := url.QueryUnescape(L.CheckString(1))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(dec))
		return 1
	}))
	L.SetField(util, "AESEncrypt", L.NewFunction(func(L *lua.LState) int {
		plain := L.CheckString(1)
		key := L.CheckString(2)
		result, err := aesEncrypt(plain, key)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(result))
		return 1
	}))
	L.SetField(util, "AESDecrypt", L.NewFunction(func(L *lua.LState) int {
		ciphertext := L.CheckString(1)
		key := L.CheckString(2)
		result, err := aesDecrypt(ciphertext, key)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(result))
		return 1
	}))
	L.SetField(util, "GetEnv", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(1)
		if strings.HasPrefix(key, "ONEBOT_") {
			L.Push(lua.LNil)
			return 1
		}
		v := os.Getenv(key)
		if v == "" && L.GetTop() >= 2 {
			v = L.CheckString(2)
		}
		if v == "" {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(v))
		}
		return 1
	}))
	L.SetField(util, "Sleep", L.NewFunction(func(L *lua.LState) int {
		sec := L.CheckNumber(1)
		time.Sleep(time.Duration(float64(sec) * float64(time.Second)))
		return 0
	}))
	L.SetField(util, "After", L.NewFunction(func(L *lua.LState) int {
		sec := L.CheckNumber(1)
		fn := L.CheckTable(2)
		m.timerSeq++
		m.timers = append(m.timers, timerEntry{
			id:       m.timerSeq,
			deadline: time.Now().Add(time.Duration(float64(sec) * float64(time.Second))),
			fn:       fn,
		})
		L.Push(lua.LNumber(m.timerSeq))
		return 1
	}))
	L.SetField(util, "CancelTimer", L.NewFunction(func(L *lua.LState) int {
		id := int64(L.CheckNumber(1))
		for i, t := range m.timers {
			if t.id == id {
				m.timers = append(m.timers[:i], m.timers[i+1:]...)
				break
			}
		}
		return 0
	}))
	L.SetGlobal("util", util)

	L.SetGlobal("bot", bot)
}

func safeDataPath(rel string) (string, error) {
	return resolveSafe(rel, "data")
}

func (m *Manager) safeFilePath(rel string) (string, error) {
	if m.fileUnsafe {
		return resolveSafe(rel, "")
	}
	return resolveSafe(rel, "data")
}

func resolveSafe(rel, root string) (string, error) {
	clean := filepath.Clean(rel)
	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path not allowed: %s", rel)
	}
	base := root
	if base == "" {
		base = "."
	}
	abs, err := filepath.Abs(filepath.Join(base, clean))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	if root != "" {
		dataRoot, _ := filepath.Abs(root)
		if !strings.HasPrefix(abs, dataRoot+string(filepath.Separator)) && abs != dataRoot {
			return "", fmt.Errorf("path outside data dir: %s", rel)
		}
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}
	if root != "" {
		dataRoot, _ := filepath.Abs(root)
		if !strings.HasPrefix(real, dataRoot+string(filepath.Separator)) && real != dataRoot {
			return "", fmt.Errorf("symlink escapes data dir: %s", rel)
		}
	}
	return abs, nil
}

func luaValueToMsg(v lua.LValue) interface{} {
	switch val := v.(type) {
	case lua.LString:
		return string(val)
	case *lua.LTable:
		var segments []map[string]interface{}
		val.ForEach(func(_, seg lua.LValue) {
			tbl, ok := seg.(*lua.LTable)
			if !ok {
				return
			}
			segMap := make(map[string]interface{})
			typ := tbl.RawGetString("type")
			if typ != lua.LNil {
				segMap["type"] = string(typ.(lua.LString))
			}
			dataVal := tbl.RawGetString("data")
			if dataVal != lua.LNil {
				if dataTbl, ok := dataVal.(*lua.LTable); ok {
					dataMap := make(map[string]interface{})
					dataTbl.ForEach(func(k, dv lua.LValue) {
						dataMap[string(k.(lua.LString))] = luaValueToSimple(dv)
					})
					segMap["data"] = dataMap
				}
			}
			segments = append(segments, segMap)
		})
		return segments
	}
	return ""
}

func luaValueToSimple(v lua.LValue) interface{} {
	switch val := v.(type) {
	case lua.LString:
		return string(val)
	case lua.LNumber:
		return float64(val)
	case lua.LBool:
		return bool(val)
	default:
		return v.String()
	}
}

func (m *Manager) LoadPlugins(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	dir, _ = filepath.Abs(dir)

	initDirs := make(map[string]bool)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == "init.lua" {
			initDirs[filepath.Dir(path)] = true
		}
		return nil
	})

	m.L.SetGlobal("include", m.L.NewFunction(func(L *lua.LState) int {
		currentDir := string(L.GetGlobal("__plugin_dir").(lua.LString))
		target := L.CheckString(1)
		targetPath := filepath.Join(currentDir, target)
		if !strings.HasSuffix(targetPath, ".lua") {
			targetPath += ".lua"
		}
		targetPath, _ = filepath.Abs(targetPath)

		if !strings.HasPrefix(targetPath, currentDir+string(filepath.Separator)) && targetPath != currentDir {
			L.RaiseError("include %s: not allowed outside plugin directory", target)
			return 0
		}

		prevDir := L.GetGlobal("__plugin_dir")
		L.SetGlobal("__plugin_dir", lua.LString(filepath.Dir(targetPath)))

		if err := L.DoFile(targetPath); err != nil {
			L.RaiseError("include %s: %v", target, err)
			return 0
		}

		L.SetGlobal("__plugin_dir", prevDir)
		return 1
	}))

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".lua") {
			return nil
		}

		parentDir := filepath.Dir(path)
		if initDirs[parentDir] && info.Name() != "init.lua" {
			return nil // loaded by init.lua via include()
		}

		return m.doPluginFile(dir, path)
	})

	return nil
}

func (m *Manager) doPluginFile(baseDir, path string) error {
	m.L.SetGlobal("__plugin_dir", lua.LString(filepath.Dir(path)))

	if err := m.L.DoFile(path); err != nil {
		logc.Fmt("!", logc.Yellow, "插件加载失败 %s: %v", path, err)
		return nil
	}

	rel, _ := filepath.Rel(baseDir, path)
	pluginName := strings.TrimSuffix(rel, ".lua")
	pluginName = strings.ReplaceAll(pluginName, string(filepath.Separator), "/")

	pluginTable := m.L.Get(-1)

	if tbl, ok := pluginTable.(*lua.LTable); ok {
		m.registry.RawSetString(pluginName, tbl)
		logc.Fmt("◆", logc.Magenta, "已加载插件: %s", pluginName)
	} else {
		logc.Fmt("!", logc.Gray, "跳过 %s (未返回 table)", pluginName)
	}
	m.L.Pop(1)

	return nil
}

func (m *Manager) Dispatch(event *onebot.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	m.L.SetContext(ctx)
	defer m.L.RemoveContext()

	L := m.L
	m.processTimers(L)
	eventTbl := eventToTable(L, event)

	handlerKey, fallbackKey := routeEvent(event)

	m.registry.ForEach(func(name, plugin lua.LValue) {
		tbl, ok := plugin.(*lua.LTable)
		if !ok {
			return
		}
		handler := tbl.RawGetString(handlerKey)
		if handler == lua.LNil && fallbackKey != "" {
			handler = tbl.RawGetString(fallbackKey)
		}
		if handler == lua.LNil {
			return
		}

		L.Push(handler)
		L.Push(eventTbl)
		if err := L.PCall(1, 0, nil); err != nil {
			logc.Fmt("!", logc.Red, "[%s] %s 错误: %v", name, handlerKey, err)
		}
	})
}

func (m *Manager) processTimers(L *lua.LState) {
	if len(m.timers) == 0 {
		return
	}
	now := time.Now()
	remaining := make([]timerEntry, 0, len(m.timers))
	for _, t := range m.timers {
		if !now.Before(t.deadline) {
			L.Push(t.fn)
			L.PCall(0, 0, nil)
		} else {
			remaining = append(remaining, t)
		}
	}
	m.timers = remaining
}

func routeEvent(e *onebot.Event) (key, fallback string) {
	switch e.PostType {
	case "message":
		switch e.MessageType {
		case "group":
			return "on_group_message", ""
		case "private":
			return "on_private_message", ""
		default:
			return "on_message", ""
		}
	case "notice":
		switch e.NoticeType {
		case "group_upload":
			return "on_group_upload", "on_notice"
		case "group_admin":
			return "on_group_admin", "on_notice"
		case "group_increase":
			return "on_group_increase", "on_notice"
		case "group_decrease":
			return "on_group_decrease", "on_notice"
		case "group_ban":
			return "on_group_ban", "on_notice"
		case "friend_add":
			return "on_friend_add", "on_notice"
		case "group_recall":
			return "on_group_recall", "on_notice"
		case "friend_recall":
			return "on_friend_recall", "on_notice"
		case "notify":
			return "on_notify", "on_notice"
		case "group_card":
			return "on_group_card", "on_notice"
		case "offline_file":
			return "on_offline_file", "on_notice"
		case "client_status":
			return "on_client_status", "on_notice"
		case "essence":
			return "on_essence", "on_notice"
		default:
			return "on_notice", ""
		}
	case "request":
		switch e.RequestType {
		case "friend":
			return "on_friend_request", "on_request"
		case "group":
			return "on_group_request", "on_request"
		default:
			return "on_request", ""
		}
	case "meta_event":
		return "on_meta_event", ""
	default:
		return "on_" + e.PostType, ""
	}
}

func eventToTable(L *lua.LState, e *onebot.Event) *lua.LTable {
	tbl := L.NewTable()
	tbl.RawSetString("time", lua.LNumber(e.Time))
	tbl.RawSetString("self_id", lua.LNumber(e.SelfID))
	tbl.RawSetString("post_type", lua.LString(e.PostType))
	tbl.RawSetString("message_type", lua.LString(e.MessageType))
	tbl.RawSetString("notice_type", lua.LString(e.NoticeType))
	tbl.RawSetString("request_type", lua.LString(e.RequestType))
	tbl.RawSetString("message_id", lua.LNumber(e.MessageID))
	tbl.RawSetString("sub_type", lua.LString(e.SubType))
	tbl.RawSetString("group_id", lua.LNumber(e.GroupID))
	tbl.RawSetString("user_id", lua.LNumber(e.UserID))
	tbl.RawSetString("operator_id", lua.LNumber(e.OperatorID))
	tbl.RawSetString("target_id", lua.LNumber(e.TargetID))
	tbl.RawSetString("raw_message", lua.LString(e.RawMessage))
	tbl.RawSetString("comment", lua.LString(e.Comment))
	tbl.RawSetString("flag", lua.LString(e.Flag))

	if e.File != nil {
		fileTbl := L.NewTable()
		fileTbl.RawSetString("id", lua.LString(e.File.ID))
		fileTbl.RawSetString("name", lua.LString(e.File.Name))
		fileTbl.RawSetString("size", lua.LNumber(e.File.Size))
		fileTbl.RawSetString("busid", lua.LNumber(e.File.BusID))
		tbl.RawSetString("file", fileTbl)
	}

	sender := L.NewTable()
	sender.RawSetString("user_id", lua.LNumber(e.Sender.UserID))
	sender.RawSetString("nickname", lua.LString(e.Sender.Nickname))
	sender.RawSetString("card", lua.LString(e.Sender.Card))
	sender.RawSetString("role", lua.LString(e.Sender.Role))
	tbl.RawSetString("sender", sender)

	return tbl
}

func luaToGo(v lua.LValue) interface{} {
	switch val := v.(type) {
	case lua.LString:
		return string(val)
	case lua.LNumber:
		return float64(val)
	case lua.LBool:
		return bool(val)
	case *lua.LTable:
		if val.Len() > 0 {
			arr := make([]interface{}, 0)
			val.ForEach(func(_, elem lua.LValue) {
				arr = append(arr, luaToGo(elem))
			})
			return arr
		}
		obj := make(map[string]interface{})
		val.ForEach(func(k, elem lua.LValue) {
			obj[k.String()] = luaToGo(elem)
		})
		return obj
	default:
		return v.String()
	}
}

func jsonToLua(L *lua.LState, v interface{}) lua.LValue {
	switch val := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(val)
	case float64:
		return lua.LNumber(val)
	case string:
		return lua.LString(val)
	case []interface{}:
		tbl := L.NewTable()
		for i, item := range val {
			tbl.RawSetInt(i+1, jsonToLua(L, item))
		}
		return tbl
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, item := range val {
			tbl.RawSetString(k, jsonToLua(L, item))
		}
		return tbl
	default:
		return lua.LString(fmt.Sprint(val))
	}
}

func printLuaTable(tbl *lua.LTable, depth int) string {
	if depth > 8 {
		return "{...}"
	}
	indent := strings.Repeat("  ", depth)
	childIndent := strings.Repeat("  ", depth+1)

	var parts []string
	seen := make(map[string]bool)

	tbl.ForEach(func(k, v lua.LValue) {
		key := k.String()
		if seen[key] {
			return
		}
		seen[key] = true

		var valStr string
		if sub, ok := v.(*lua.LTable); ok {
			valStr = "\n" + printLuaTable(sub, depth+1)
		} else {
			valStr = fmt.Sprint(v)
		}

		if n, ok := k.(lua.LNumber); ok && float64(n) == float64(int64(n)) {
			parts = append(parts, fmt.Sprintf("%s[%d] = %s", childIndent, int64(n), valStr))
		} else {
			parts = append(parts, fmt.Sprintf("%s[%q] = %s", childIndent, key, valStr))
		}
	})

	if len(parts) == 0 {
		return indent + "{}"
	}
	return indent + "{\n" + strings.Join(parts, ",\n") + "\n" + indent + "}"
}

func aesEncrypt(plain, key string) (string, error) {
	block, err := aes.NewCipher(aesKey(key))
	if err != nil {
		return "", err
	}
	plainBytes := []byte(plain)
	plainBytes = pkcs7Pad(plainBytes, aes.BlockSize)
	ciphertext := make([]byte, aes.BlockSize+len(plainBytes))
	iv := ciphertext[:aes.BlockSize]
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plainBytes)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func aesDecrypt(cipherB64, key string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return "", err
	}
	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	block, err := aes.NewCipher(aesKey(key))
	if err != nil {
		return "", err
	}
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	plainBytes := pkcs7Unpad(ciphertext)
	if plainBytes == nil {
		return "", fmt.Errorf("invalid padding")
	}
	return string(plainBytes), nil
}

func aesKey(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	for i := 0; i < pad; i++ {
		data = append(data, byte(pad))
	}
	return data
}

func pkcs7Unpad(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	pad := int(data[len(data)-1])
	if pad > len(data) || pad > aes.BlockSize {
		return nil
	}
	return data[:len(data)-pad]
}
