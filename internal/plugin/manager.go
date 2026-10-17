package plugin

import (
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
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"quill/internal/logc"
	"quill/internal/module"
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
	logCh      chan logEntry
	logDone    chan struct{}
	memFS      map[string][]byte
	memFSMu    sync.RWMutex
}

type logEntry struct {
	level string
	msg   string
	time  time.Time
}

type timerEntry struct {
	id       int64
	deadline time.Time
	fn       lua.LValue
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
		logCh:      make(chan logEntry, 256),
		logDone:    make(chan struct{}),
		memFS:      make(map[string][]byte),
	}
	go m.logWriter()
	m.registerBotAPI()
	return m
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	close(m.logCh)
	<-m.logDone
	m.L.Close()
}

func (m *Manager) logWriter() {
	defer close(m.logDone)
	var buf strings.Builder
	for entry := range m.logCh {
		buf.Reset()
		buf.WriteString(entry.time.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&buf, " [%-5s] %s", entry.level, entry.msg)
		fmt.Println(buf.String())
	}
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

	L.SetField(bot, "send_like", L.NewFunction(func(L *lua.LState) int {
		userID := L.CheckInt64(1)
		times := 1
		if L.GetTop() >= 2 {
			times = L.CheckInt(2)
		}
		if err := m.client.SendLike(userID, times); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_kick", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		uid := L.CheckInt64(2)
		reject := false
		if L.GetTop() >= 3 {
			reject = L.CheckBool(3)
		}
		if err := m.client.SetGroupKick(gid, uid, reject); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_ban", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		uid := L.CheckInt64(2)
		duration := int64(L.CheckNumber(3))
		if err := m.client.SetGroupBan(gid, uid, duration); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_whole_ban", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		enable := L.CheckBool(2)
		if err := m.client.SetGroupWholeBan(gid, enable); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_admin", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		uid := L.CheckInt64(2)
		enable := L.CheckBool(3)
		if err := m.client.SetGroupAdmin(gid, uid, enable); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_card", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		uid := L.CheckInt64(2)
		card := L.CheckString(3)
		if err := m.client.SetGroupCard(gid, uid, card); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_name", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		name := L.CheckString(2)
		if err := m.client.SetGroupName(gid, name); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_special_title", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		uid := L.CheckInt64(2)
		title := L.CheckString(3)
		duration := int64(L.CheckNumber(4))
		if err := m.client.SetGroupSpecialTitle(gid, uid, title, duration); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_leave", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		isDismiss := false
		if L.GetTop() >= 2 {
			isDismiss = L.CheckBool(2)
		}
		if err := m.client.SetGroupLeave(gid, isDismiss); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "get_status", L.NewFunction(func(L *lua.LState) int {
		data, err := m.client.GetStatus()
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))

	L.SetField(bot, "get_version", L.NewFunction(func(L *lua.LState) int {
		data, err := m.client.GetVersionInfo()
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))

	L.SetField(bot, "set_friend_add_request", L.NewFunction(func(L *lua.LState) int {
		flag := L.CheckString(1)
		approve := L.CheckBool(2)
		remark := ""
		if L.GetTop() >= 3 {
			remark = L.CheckString(3)
		}
		if err := m.client.SetFriendAddRequest(flag, approve, remark); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	L.SetField(bot, "set_group_add_request", L.NewFunction(func(L *lua.LState) int {
		flag := L.CheckString(1)
		subType := L.CheckString(2)
		approve := L.CheckBool(3)
		reason := ""
		if L.GetTop() >= 4 {
			reason = L.CheckString(4)
		}
		if err := m.client.SetGroupAddRequest(flag, subType, approve, reason); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	queryAPI := func(name string, fn func() (json.RawMessage, error)) {
		L.SetField(bot, name, L.NewFunction(func(L *lua.LState) int {
			data, err := fn()
			if err != nil {
				L.Push(lua.LNil)
				L.Push(lua.LString(err.Error()))
				return 2
			}
			L.Push(lua.LString(string(data)))
			return 1
		}))
	}
	queryAPI("get_login_info", func() (json.RawMessage, error) { return m.client.GetLoginInfo() })
	queryAPI("get_friend_list", func() (json.RawMessage, error) { return m.client.GetFriendList() })
	queryAPI("get_group_list", func() (json.RawMessage, error) { return m.client.GetGroupList() })
	queryAPI("get_csrf_token", func() (json.RawMessage, error) { return m.client.GetCSRFToken() })
	queryAPI("can_send_image", func() (json.RawMessage, error) { return m.client.CanSendImage() })
	queryAPI("can_send_record", func() (json.RawMessage, error) { return m.client.CanSendRecord() })

	L.SetField(bot, "get_stranger_info", L.NewFunction(func(L *lua.LState) int {
		uid := L.CheckInt64(1)
		noCache := false
		if L.GetTop() >= 2 {
			noCache = L.CheckBool(2)
		}
		data, err := m.client.GetStrangerInfo(uid, noCache)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_group_info", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		noCache := false
		if L.GetTop() >= 2 {
			noCache = L.CheckBool(2)
		}
		data, err := m.client.GetGroupInfo(gid, noCache)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_group_member_info", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		uid := L.CheckInt64(2)
		noCache := false
		if L.GetTop() >= 3 {
			noCache = L.CheckBool(3)
		}
		data, err := m.client.GetGroupMemberInfo(gid, uid, noCache)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_group_member_list", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		data, err := m.client.GetGroupMemberList(gid)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_group_honor_info", L.NewFunction(func(L *lua.LState) int {
		gid := L.CheckInt64(1)
		htype := "all"
		if L.GetTop() >= 2 {
			htype = L.CheckString(2)
		}
		data, err := m.client.GetGroupHonorInfo(gid, htype)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_cookies", L.NewFunction(func(L *lua.LState) int {
		domain := L.CheckString(1)
		data, err := m.client.GetCookies(domain)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_credentials", L.NewFunction(func(L *lua.LState) int {
		domain := L.CheckString(1)
		data, err := m.client.GetCredentials(domain)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_record", L.NewFunction(func(L *lua.LState) int {
		file := L.CheckString(1)
		outFmt := "mp3"
		if L.GetTop() >= 2 {
			outFmt = L.CheckString(2)
		}
		data, err := m.client.GetRecord(file, outFmt)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(string(data)))
		return 1
	}))
	L.SetField(bot, "get_image", L.NewFunction(func(L *lua.LState) int {
		file := L.CheckString(1)
		data, err := m.client.GetImage(file)
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
	L.SetField(util, "TableKeys", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		result := L.NewTable()
		i := 1
		tbl.ForEach(func(k, _ lua.LValue) {
			result.RawSetInt(i, lua.LString(k.String()))
			i++
		})
		L.Push(result)
		return 1
	}))
	L.SetField(util, "TableLen", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		count := 0
		tbl.ForEach(func(_, _ lua.LValue) {
			count++
		})
		L.Push(lua.LNumber(count))
		return 1
	}))
	L.SetField(util, "TableGet", L.NewFunction(func(L *lua.LState) int {
		tbl := L.CheckTable(1)
		path := L.CheckString(2)
		L.Push(digTable(tbl, path))
		return 1
	}))
	L.SetField(util, "DNSResolveALL", L.NewFunction(func(L *lua.LState) int {
		domain := L.CheckString(1)
		addrs, err := net.LookupHost(domain)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		var v4, v6 []string
		for _, addr := range addrs {
			if strings.Contains(addr, ":") {
				v6 = append(v6, addr)
			} else {
				v4 = append(v4, addr)
			}
		}
		tbl := L.NewTable()
		if len(v4) > 0 {
			tbl.RawSetInt(0, lua.LString(v4[0]))
		}
		if len(v6) > 0 {
			tbl.RawSetInt(1, lua.LString(v6[0]))
		}
		L.Push(tbl)
		return 1
	}))
	L.SetField(util, "DNSResolve", L.NewFunction(func(L *lua.LState) int {
		domain := L.CheckString(1)
		addrs, err := net.LookupHost(domain)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LString(addrs[0]))
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
		fn := L.Get(2)
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
	L.SetField(util, "Log", L.NewFunction(func(L *lua.LState) int {
		level := L.CheckString(1)
		msg := L.CheckString(2)
		select {
		case m.logCh <- logEntry{level, msg, time.Now()}:
		default:
		}
		return 0
	}))

	memFS := L.NewTable()
	L.SetField(memFS, "Write", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		data := L.CheckString(2)
		m.memFSMu.Lock()
		m.memFS[name] = []byte(data)
		m.memFSMu.Unlock()
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(memFS, "Read", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		m.memFSMu.RLock()
		data, ok := m.memFS[name]
		m.memFSMu.RUnlock()
		if !ok {
			L.Push(lua.LNil)
		} else {
			L.Push(lua.LString(string(data)))
		}
		return 1
	}))
	L.SetField(memFS, "Delete", L.NewFunction(func(L *lua.LState) int {
		m.memFSMu.Lock()
		delete(m.memFS, L.CheckString(1))
		m.memFSMu.Unlock()
		return 0
	}))
	L.SetField(memFS, "Exists", L.NewFunction(func(L *lua.LState) int {
		m.memFSMu.RLock()
		_, ok := m.memFS[L.CheckString(1)]
		m.memFSMu.RUnlock()
		L.Push(lua.LBool(ok))
		return 1
	}))
	L.SetField(memFS, "List", L.NewFunction(func(L *lua.LState) int {
		m.memFSMu.RLock()
		tbl := L.NewTable()
		i := 1
		for k := range m.memFS {
			tbl.RawSetInt(i, lua.LString(k))
			i++
		}
		m.memFSMu.RUnlock()
		L.Push(tbl)
		return 1
	}))
	L.SetField(memFS, "Clear", L.NewFunction(func(L *lua.LState) int {
		m.memFSMu.Lock()
		m.memFS = make(map[string][]byte)
		m.memFSMu.Unlock()
		return 0
	}))
	L.SetField(memFS, "Dump", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		path := L.CheckString(2)
		m.memFSMu.RLock()
		data, ok := m.memFS[name]
		m.memFSMu.RUnlock()
		if !ok {
			L.Push(lua.LNil)
			L.Push(lua.LString("not found in MemFS"))
			return 2
		}
		safePath, err := safeDataPath(path)
		if err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		os.MkdirAll(filepath.Dir(safePath), 0755)
		if err := os.WriteFile(safePath, data, 0644); err != nil {
			L.Push(lua.LBool(false))
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))
	L.SetField(memFS, "Load", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(1)
		safePath, err := safeDataPath(path)
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
		m.memFSMu.Lock()
		m.memFS[filepath.Base(path)] = data
		m.memFSMu.Unlock()
		L.Push(lua.LString(filepath.Base(path)))
		return 1
	}))
	L.SetField(util, "MemFS", memFS)

	module.RegisterModuleUtil(L, util)

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

func digTable(t *lua.LTable, path string) lua.LValue {
	parts := strings.Split(path, ".")
	for _, p := range parts {
		if t == nil {
			return lua.LNil
		}
		v := t.RawGetString(p)
		next, ok := v.(*lua.LTable)
		if !ok {
			return v
		}
		t = next
	}
	return t
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
	case *lua.LTable:
		return luaTableToSimpleValue(val)
	default:
		return v.String()
	}
}

func luaTableToSimpleValue(tbl *lua.LTable) interface{} {
	maxIdx := tbl.Len()
	if maxIdx > 0 {
		arr := make([]interface{}, 0, maxIdx)
		tbl.ForEach(func(k, v lua.LValue) {
			if kn, ok := k.(lua.LNumber); ok {
				idx := int(kn)
				if idx >= 1 && idx <= maxIdx {
					for len(arr) < idx {
						arr = append(arr, nil)
					}
					arr[idx-1] = luaValueToSimple(v)
				}
			}
		})
		return arr
	}
	m := make(map[string]interface{})
	tbl.ForEach(func(k, v lua.LValue) {
		m[k.String()] = luaValueToSimple(v)
	})
	return m
}

func (m *Manager) LoadPlugins(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doLoadPlugins(dir)
}

func (m *Manager) UnloadPlugins() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.doUnloadPlugins()
}

func (m *Manager) doUnloadPlugins() {
	m.registry = m.L.NewTable()
	m.L.SetGlobal("__plugin_registry", m.registry)
	logc.Fmt("\u2715", logc.Gray, "\u5df2\u5378\u8f7d\u5168\u90e8\u63d2\u4ef6")
}

func (m *Manager) ReloadPlugins(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.doUnloadPlugins()
	return m.doLoadPlugins(dir)
}

// underInitDir reports whether dirPath is at or below a directory that contains
// an init.lua. Files inside a directory plugin are loaded through include() and
// must not be registered as standalone plugins.
func underInitDir(dirPath, root string, initDirs map[string]bool) bool {
	for d := dirPath; ; d = filepath.Dir(d) {
		if initDirs[d] {
			return true
		}
		if d == root || len(d) <= len(root) {
			return false
		}
	}
}

func (m *Manager) doLoadPlugins(dir string) error {
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

		source, err := os.ReadFile(targetPath)
		if err != nil {
			L.RaiseError("include %s: %v", target, err)
			return 0
		}
		source = preprocessLua(source)
		if err := L.DoString(string(source)); err != nil {
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

		if info.Name() != "init.lua" && underInitDir(filepath.Dir(path), dir, initDirs) {
			return nil // loaded by init.lua via include()
		}

		return m.doPluginFile(dir, path)
	})

	return nil
}

func (m *Manager) doPluginFile(baseDir, path string) error {
	m.L.SetGlobal("__plugin_dir", lua.LString(filepath.Dir(path)))

	source, err := os.ReadFile(path)
	if err != nil {
		logc.Fmt("!", logc.Yellow, "插件读取失败 %s: %v", path, err)
		return nil
	}
	source = preprocessLua(source)

	if err := m.L.DoString(string(source)); err != nil {
		logc.Fmt("!", logc.Yellow, "插件加载失败 %s: %v", path, err)
		return nil
	}

	rel, _ := filepath.Rel(baseDir, path)
	fileName := strings.TrimSuffix(rel, ".lua")
	fileName = strings.ReplaceAll(fileName, string(filepath.Separator), "/")

	pluginTable := m.L.Get(-1)

	tbl, ok := pluginTable.(*lua.LTable)
	if !ok {
		logc.Fmt("!", logc.Gray, "跳过 %s (未返回 table)", fileName)
		m.L.Pop(1)
		return nil
	}

	info := tbl.RawGetString("PluginInfo")
	if info == lua.LNil {
		logc.Fmt("!", logc.Red, "加载失败 %s: 缺少 PluginInfo", fileName)
		m.L.Pop(1)
		return nil
	}

	infoTbl, ok := info.(*lua.LTable)
	if !ok {
		logc.Fmt("!", logc.Red, "加载失败 %s: PluginInfo 不是 table", fileName)
		m.L.Pop(1)
		return nil
	}

	pName := stringOf(infoTbl.RawGetString("name"))
	pVer := stringOf(infoTbl.RawGetString("version"))
	if pName == "" || pVer == "" {
		logc.Fmt("!", logc.Red, "加载失败 %s: PluginInfo 缺少 name 或 version", fileName)
		m.L.Pop(1)
		return nil
	}

	pAuthor := stringOf(infoTbl.RawGetString("author"))
	pUpdate := stringOf(infoTbl.RawGetString("update_url"))
	_ = pUpdate

	displayName := pName
	if pVer != "" {
		displayName += " v" + pVer
	}
	extra := ""
	if pAuthor != "" {
		extra += " @" + pAuthor
	}
	if extra == "" {
		extra = fileName
	} else {
		extra += " (" + fileName + ")"
	}

	m.registry.RawSetString(displayName, tbl)
	logc.Fmt("\u25C6", logc.Magenta, "已加载插件: %s %s", displayName, extra)
	m.L.Pop(1)
	return nil
}

func preprocessLua(src []byte) []byte {
	s := string(src)
	s = strings.ReplaceAll(s, "!=", "~=")
	s = strings.ReplaceAll(s, "||", " or ")
	s = strings.ReplaceAll(s, "&&", " and ")
	s = strings.ReplaceAll(s, "!", "not ")
	return []byte(s)
}

func stringOf(v lua.LValue) string {
	if v == lua.LNil {
		return ""
	}
	if s, ok := v.(lua.LString); ok {
		return string(s)
	}
	return v.String()
}

func (m *Manager) Dispatch(event *onebot.Event) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.processTimers(m.L)
	eventTbl := eventToTable(m.L, event)

	handlerKey, fallbackKey := routeEvent(event)
	L := m.L

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

	const maxEntries = 32
	var parts []string
	count := 0
	seen := make(map[string]bool)

	tbl.ForEach(func(k, v lua.LValue) {
		if count >= maxEntries {
			return
		}
		key := k.String()
		if seen[key] {
			return
		}
		seen[key] = true
		count++

		var valStr string
		if sub, ok := v.(*lua.LTable); ok {
			if sub == tbl {
				valStr = "<self>"
			} else {
				valStr = "\n" + printLuaTable(sub, depth+1)
			}
		} else {
			s := fmt.Sprint(v)
			if len(s) > 200 {
				s = s[:200] + "..."
			}
			valStr = s
		}

		if n, ok := k.(lua.LNumber); ok && float64(n) == float64(int64(n)) {
			parts = append(parts, fmt.Sprintf("%s[%d] = %s", childIndent, int64(n), valStr))
		} else {
			parts = append(parts, fmt.Sprintf("%s[%q] = %s", childIndent, key, valStr))
		}
	})

	result := indent + "{\n" + strings.Join(parts, ",\n") + "\n" + indent + "}"
	if len(result) > 8192 {
		result = result[:8192] + "\n... (truncated)"
	}
	if count == maxEntries {
		result += "\n... (max entries)"
	}
	return result
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
