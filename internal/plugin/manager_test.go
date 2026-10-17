package plugin

import (
	"context"
	"os"
	"path/filepath"
	"quill/internal/onebot"
	"quill/internal/storage"
	"strings"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager(nil, nil, false)
	t.Cleanup(m.Close)
	return m
}

// 没有 return 的插件文件不能导致进程崩溃。
func TestPluginWithoutReturnIsSkipped(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plain.lua"), []byte("-- 只有注释和赋值\nLocalVar = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "good.lua"), []byte(`
return { PluginInfo = { name = "good", version = "1.0.0" } }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestManager(t)
	if err := m.LoadPlugins(dir); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}
	if n := countRegistry(m); n != 1 {
		t.Fatalf("registry has %d plugins, want 1", n)
	}
}

// 插件源码里的 ! 不应被改写（旧的 preprocessLua 会破坏字符串）。
func TestPluginSourceNotRewritten(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "p.lua"), []byte(`
return {
    PluginInfo = { name = "p", version = "1.0.0" },
    marker = "a!b && c || d != e",
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestManager(t)
	if err := m.LoadPlugins(dir); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	var got lua.LValue
	m.registry.ForEach(func(_, v lua.LValue) {
		if tbl, ok := v.(*lua.LTable); ok {
			got = tbl.RawGetString("marker")
		}
	})
	want := "a!b && c || d != e"
	if stringOf(got) != want {
		t.Fatalf("marker = %q, want %q", stringOf(got), want)
	}
}

// include() 应返回被加载文件的结果表。
func TestIncludeReturnsModuleTable(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "admin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "admin", "util.lua"), []byte(`
return { value = 42 }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "admin", "init.lua"), []byte(`
local u = include("util")
return { PluginInfo = { name = "admin", version = "1.0.0" }, got = u.value }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestManager(t)
	if err := m.LoadPlugins(dir); err != nil {
		t.Fatalf("LoadPlugins: %v", err)
	}

	var got lua.LValue
	m.registry.ForEach(func(_, v lua.LValue) {
		if tbl, ok := v.(*lua.LTable); ok {
			got = tbl.RawGetString("got")
		}
	})
	if n, ok := got.(lua.LNumber); !ok || int(n) != 42 {
		t.Fatalf("include result = %v, want 42", got)
	}
}

func countRegistry(m *Manager) int {
	n := 0
	m.registry.ForEach(func(_, _ lua.LValue) { n++ })
	return n
}

// 新建文件必须允许写入 data/（旧实现因 EvalSymlinks 对新路径失败而报错）。
func TestSafeDataPathAllowsNewFile(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("data", 0o755); err != nil {
		t.Fatal(err)
	}

	p, err := safeDataPath("new_file.txt")
	if err != nil {
		t.Fatalf("safeDataPath(new_file.txt) error: %v", err)
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "/data/new_file.txt") {
		t.Fatalf("unexpected path %q", p)
	}

	// 多级新目录也应允许（父目录尚未创建）。
	if _, err := safeDataPath("sub/dir/file.txt"); err != nil {
		t.Fatalf("nested new path error: %v", err)
	}
}

func TestSafeDataPathRejectsEscapes(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("data", 0o755); err != nil {
		t.Fatal(err)
	}

	bad := []string{
		"../secret.txt",
		"a/../../secret.txt",
		filepath.Join("..", "..", "windows", "system32"),
	}
	for _, p := range bad {
		if _, err := safeDataPath(p); err == nil {
			t.Errorf("safeDataPath(%q) should fail", p)
		}
	}

	// 绝对路径也应拒绝
	if _, err := safeDataPath(`C:\Windows\win.ini`); err == nil {
		t.Error("absolute path should be rejected")
	}

	// 文件名里含 .. 不应误杀
	if _, err := safeDataPath("a..b.txt"); err != nil {
		t.Errorf("safeDataPath(a..b.txt) should be allowed: %v", err)
	}
}

// 消息段类型错误应返回 error，而不是 panic。
func TestLuaValueToMsgNoPanic(t *testing.T) {
	L := lua.NewState()
	defer L.Close()

	bad := L.NewTable()
	seg := L.NewTable()
	seg.RawSetString("type", lua.LNumber(1))
	bad.RawSetInt(1, seg)
	if _, err := luaValueToMsg(bad); err == nil {
		t.Fatal("expected error for numeric segment type")
	}

	// 数组里的字符串按 text 段处理
	mixed := L.NewTable()
	mixed.RawSetInt(1, lua.LString("hello"))
	mixed.RawSetInt(2, L.NewTable())
	if _, err := luaValueToMsg(mixed); err == nil {
		t.Fatal("expected error for segment without type")
	}

	mixed.RawSetInt(2, func() *lua.LTable {
		t := L.NewTable()
		t.RawSetString("type", lua.LString("at"))
		return t
	}())
	msg, err := luaValueToMsg(mixed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	segs, ok := msg.([]map[string]interface{})
	if !ok || len(segs) != 2 {
		t.Fatalf("segments = %#v", msg)
	}
	if segs[0]["type"] != "text" {
		t.Fatalf("first segment type = %v, want text", segs[0]["type"])
	}
}

// 定时器回调中再次注册的定时器不能被丢弃（轮询模式依赖）。
func TestTimerReschedulingSurvives(t *testing.T) {
	m := newTestManager(t)

	if err := m.L.DoString(`
		count = 0
		function tick()
			count = count + 1
			if count < 3 then
				util.After(0, tick)
			end
		end
	`); err != nil {
		t.Fatal(err)
	}

	fn := m.L.GetGlobal("tick")
	m.mu.Lock()
	m.timers = []timerEntry{{id: 1, deadline: time.Now().Add(-time.Second), fn: fn}}
	m.processTimersLocked()
	m.mu.Unlock()

	for i := 0; i < 4; i++ {
		time.Sleep(2 * time.Millisecond)
		m.mu.Lock()
		m.processTimersLocked()
		m.mu.Unlock()
	}

	if got := int(m.L.GetGlobal("count").(lua.LNumber)); got != 3 {
		t.Fatalf("count = %d, want 3", got)
	}
	if len(m.timers) != 0 {
		t.Fatalf("pending timers = %d, want 0", len(m.timers))
	}
}

// util.Sleep 必须能被打断，否则超时机制形同虚设。
func TestSleepInterruptedByContext(t *testing.T) {
	m := newTestManager(t)

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	m.L.SetContext(ctx)
	defer m.L.RemoveContext()

	fn, err := m.L.Load(strings.NewReader(`util.Sleep(5)`), "sleep_test")
	if err != nil {
		t.Fatal(err)
	}
	m.L.Push(fn)
	start := time.Now()
	err = m.L.PCall(0, 0, nil)
	if err == nil {
		t.Fatal("expected sleep to be interrupted")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("sleep was not interrupted (took %v)", elapsed)
	}
}

// bot.file.write 必须能真正创建新文件（回归：EvalSymlinks 对新路径报错）。
func TestBotFileWriteCreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	db, err := storage.New("data")
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(nil, db, false)
	defer m.Close()

	if err := m.L.DoString(`assert(bot.file.write("hello.txt", "hi!"))`); err != nil {
		t.Fatalf("bot.file.write: %v", err)
	}
	b, err := os.ReadFile(filepath.Join("data", "hello.txt"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != "hi!" {
		t.Fatalf("content = %q", string(b))
	}

	if err := m.L.DoString(`local c = bot.file.read("hello.txt"); assert(c == "hi!", c)`); err != nil {
		t.Fatalf("bot.file.read: %v", err)
	}
	if err := m.L.DoString(`assert(bot.file.exists("hello.txt"))`); err != nil {
		t.Fatalf("bot.file.exists: %v", err)
	}
}

// 后台定时器循环必须真的会触发（不依赖新事件）。
func TestTimerLoopFiresWithoutEvents(t *testing.T) {
	m := newTestManager(t)
	m.Start()

	m.mu.Lock()
	err := m.L.DoString(`fired = 0; util.After(0.05, function() fired = 1 end)`)
	m.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		v := m.L.GetGlobal("fired")
		m.mu.Unlock()
		if n, ok := v.(lua.LNumber); ok && int(n) == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timer did not fire within 2s")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// 死循环必须被 5 秒上限中断，且之后 Lua 状态仍可继续使用。
func TestDispatchTimeoutInterruptsInfiniteLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("slow test")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "loop.lua"), []byte(`
return {
    PluginInfo = { name = "loop", version = "1.0.0" },
    on_group_message = function(event)
        while true do end
    end,
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestManager(t)
	if err := m.LoadPlugins(dir); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	m.Dispatch(&onebot.Event{PostType: "message", MessageType: "group", GroupID: 1})
	elapsed := time.Since(start)

	if elapsed < pluginTimeout {
		t.Fatalf("dispatch returned in %v, expected timeout around %v", elapsed, pluginTimeout)
	}
	if elapsed > pluginTimeout+5*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
	if err := m.L.DoString(`AFTER_TIMEOUT = 1`); err != nil {
		t.Fatalf("lua state unusable after timeout: %v", err)
	}
}
