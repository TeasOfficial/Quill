package database

import (
	"path/filepath"
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func runLua(t *testing.T, L *lua.LState, code string) {
	t.Helper()
	fn, err := L.Load(strings.NewReader(code), "test")
	if err != nil {
		t.Fatalf("load lua: %v", err)
	}
	L.Push(fn)
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		t.Fatalf("run lua: %v", err)
	}
	L.SetTop(0)
}

func newSQLiteLua(t *testing.T) *lua.LState {
	t.Helper()
	db, err := Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	L := lua.NewState()
	t.Cleanup(L.Close)
	bot := L.NewTable()
	Attach(L, bot, db)
	L.SetGlobal("bot", bot)
	return L
}

func TestSQLiteCRUD(t *testing.T) {
	L := newSQLiteLua(t)

	runLua(t, L, `bot.sql.exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, score INTEGER, note TEXT)")`)
	runLua(t, L, `local n, id = bot.sql.exec("INSERT INTO users (name, score, note) VALUES (?, ?, ?)", {"alice", 10, bot.sql.NULL})
		assert(n == 1, "affected=" .. tostring(n))
		assert(id == 1, "id=" .. tostring(id))`)

	runLua(t, L, `local rows = bot.sql.query("SELECT id, name, score, note FROM users")
		assert(#rows == 1, "rows=" .. tostring(#rows))
		assert(rows[1].name == "alice", tostring(rows[1].name))
		assert(rows[1].score == 10)
		assert(rows[1].note == nil)`)

	runLua(t, L, `local row = bot.sql.query_row("SELECT name FROM users WHERE id = ?", {1})
		assert(row and row.name == "alice")`)

	runLua(t, L, `local row = bot.sql.query_row("SELECT * FROM users WHERE id = ?", {999})
		assert(row == nil)`)

	runLua(t, L, `bot.sql.exec("UPDATE users SET score = score + ? WHERE name = ?", {5, "alice"})
		local row = bot.sql.query_row("SELECT score FROM users WHERE name = ?", {"alice"})
		assert(row.score == 15)`)

	runLua(t, L, `local n = bot.sql.exec("DELETE FROM users WHERE name = ?", {"alice"})
		assert(n == 1)
		assert(#(bot.sql.query("SELECT * FROM users")) == 0)`)
}

func TestSQLiteErrorsAreReturned(t *testing.T) {
	L := newSQLiteLua(t)

	runLua(t, L, `local rows, err = bot.sql.query("SELECT * FROM missing_table")
		assert(rows == nil)
		assert(type(err) == "string" and #err > 0)`)

	runLua(t, L, `local n, err = bot.sql.exec("INSERT INTO missing_table VALUES (1)")
		assert(n == nil)
		assert(type(err) == "string")`)
}

func TestSQLiteTransaction(t *testing.T) {
	L := newSQLiteLua(t)
	runLua(t, L, `bot.sql.exec("CREATE TABLE t (v INTEGER)")`)

	// 成功提交
	runLua(t, L, `local ok = bot.sql.tx(function(tx)
		tx.exec("INSERT INTO t (v) VALUES (?)", {1})
		tx.exec("INSERT INTO t (v) VALUES (?)", {2})
	end)
	assert(ok == true)
	assert(#(bot.sql.query("SELECT * FROM t")) == 2)`)

	// 出错回滚
	runLua(t, L, `local ok, err = bot.sql.tx(function(tx)
		tx.exec("INSERT INTO t (v) VALUES (?)", {3})
		error("boom")
	end)
	assert(ok == nil)
	assert(type(err) == "string" and err:find("boom"))
	assert(#(bot.sql.query("SELECT * FROM t")) == 2)`)
}

func TestSQLiteParamTypes(t *testing.T) {
	L := newSQLiteLua(t)
	runLua(t, L, `bot.sql.exec("CREATE TABLE p (i INTEGER, f REAL, s TEXT, b INTEGER)")`)
	runLua(t, L, `bot.sql.exec("INSERT INTO p (i, f, s, b) VALUES (?, ?, ?, ?)", {42, 1.5, "x", true})
		local row = bot.sql.query_row("SELECT * FROM p")
		assert(row.i == 42)
		assert(row.f == 1.5)
		assert(row.s == "x")
		assert(row.b == 1)`)
}

func TestOpenRejectsUnknownDriver(t *testing.T) {
	if _, err := Open("postgres", ""); err == nil {
		t.Fatal("unknown driver should fail")
	}
}

func TestMySQLRequiresDSN(t *testing.T) {
	if _, err := Open("mysql", ""); err == nil {
		t.Fatal("mysql without DSN should fail")
	}
}
