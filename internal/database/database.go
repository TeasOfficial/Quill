// Package database 提供 SQLite / MySQL 的薄封装，供 Lua 通过 bot.sql 使用。
// SQLite 使用纯 Go 驱动（modernc.org/sqlite），不需要 cgo。
package database

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	lua "github.com/yuin/gopher-lua"
	_ "modernc.org/sqlite"
)

// DB 是 database/sql 的薄封装，记录驱动与 DSN 便于日志排查。
type DB struct {
	*sql.DB
	Driver string
	DSN    string
}

// Open 按驱动名打开数据库。
//   - "sqlite"（默认 DSN: data/bot.db）
//   - "mysql"（必须提供 DSN）
func Open(driver, dsn string) (*DB, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "sqlite", "sqlite3":
		return openSQLite(dsn)
	case "mysql", "mariadb":
		return openMySQL(dsn)
	default:
		return nil, fmt.Errorf("不支持的 SQL 驱动 %q（可用 sqlite / mysql）", driver)
	}
}

func openSQLite(dsn string) (*DB, error) {
	if dsn == "" {
		dsn = filepath.Join("data", "bot.db")
	}

	memory := strings.Contains(dsn, ":memory:")

	if !memory {
		path := dsn
		if i := strings.IndexByte(path, '?'); i >= 0 {
			path = path[:i]
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("创建 SQLite 目录: %w", err)
		}
	}

	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	pragmas := "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	if !memory {
		pragmas += "&_pragma=journal_mode(WAL)"
	}
	dsn += sep + pragmas

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite: %w", err)
	}
	// SQLite 写并发有限，单连接最稳；bot 本身也是串行处理。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	return ping(&DB{DB: db, Driver: "sqlite", DSN: dsn})
}

func openMySQL(dsn string) (*DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("MySQL 需要 ONEBOT_SQL_DSN，例如 user:pass@tcp(127.0.0.1:3306)/quill?parseTime=true")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)

	return ping(&DB{DB: db, Driver: "mysql", DSN: dsn})
}

func ping(db *DB) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("连接 %s 失败: %w", db.Driver, err)
	}
	return db, nil
}

// querier 同时覆盖 *sql.DB 与 *sql.Tx。
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// codec 负责 Lua 值与 SQL 参数/结果之间的转换。
// null 是 bot.sql.NULL 哨兵：Lua 数组里的 nil 会截断长度，
// 需要用哨兵显式表示 SQL NULL。
type codec struct {
	null lua.LValue
}

// Attach 把 bot.sql 注册到 bot 表。
func Attach(L *lua.LState, bot *lua.LTable, db *DB) {
	tbl := L.NewTable()
	null := L.NewTable()
	tbl.RawSetString("NULL", null)
	c := &codec{null: null}

	registerQuerier(L, tbl, db, c)
	tbl.RawSetString("driver", lua.LString(db.Driver))

	// bot.sql.tx(function(tx) ... end)：成功提交，抛错则回滚。
	tbl.RawSetString("tx", L.NewFunction(func(L *lua.LState) int {
		fn := L.CheckFunction(1)
		ctx := luaCtx(L)

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		txTbl := L.NewTable()
		txTbl.RawSetString("NULL", null)
		registerQuerier(L, txTbl, tx, c)

		if err := L.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, txTbl); err != nil {
			_ = tx.Rollback()
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}

		if err := tx.Commit(); err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(lua.LTrue)
		return 1
	}))

	bot.RawSetString("sql", tbl)
}

func registerQuerier(L *lua.LState, tbl *lua.LTable, q querier, c *codec) {
	// exec(sql, params?) -> affected, last_insert_id
	tbl.RawSetString("exec", L.NewFunction(func(L *lua.LState) int {
		query := L.CheckString(1)
		args, err := c.args(L.Get(2))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		res, err := q.ExecContext(luaCtx(L), query, args...)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		affected, _ := res.RowsAffected()
		lastID, _ := res.LastInsertId()
		L.Push(lua.LNumber(affected))
		L.Push(lua.LNumber(lastID))
		return 2
	}))

	// query(sql, params?) -> { {col=val, ...}, ... }
	tbl.RawSetString("query", L.NewFunction(func(L *lua.LState) int {
		query := L.CheckString(1)
		args, err := c.args(L.Get(2))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		rows, err := q.QueryContext(luaCtx(L), query, args...)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer rows.Close()

		result, err := rowsToLua(L, rows)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		L.Push(result)
		return 1
	}))

	// query_row(sql, params?) -> row | nil
	tbl.RawSetString("query_row", L.NewFunction(func(L *lua.LState) int {
		query := L.CheckString(1)
		args, err := c.args(L.Get(2))
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		rows, err := q.QueryContext(luaCtx(L), query, args...)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		defer rows.Close()

		result, err := rowsToLua(L, rows)
		if err != nil {
			L.Push(lua.LNil)
			L.Push(lua.LString(err.Error()))
			return 2
		}
		first := result.RawGetInt(1)
		if first == lua.LNil {
			L.Push(lua.LNil)
			return 1
		}
		L.Push(first)
		return 1
	}))
}

func rowsToLua(L *lua.LState, rows *sql.Rows) (*lua.LTable, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	out := L.NewTable()
	idx := 1
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := L.NewTable()
		for i, name := range cols {
			row.RawSetString(name, toLua(vals[i]))
		}
		out.RawSetInt(idx, row)
		idx++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func toLua(v any) lua.LValue {
	switch t := v.(type) {
	case nil:
		return lua.LNil
	case bool:
		return lua.LBool(t)
	case int64:
		return lua.LNumber(float64(t))
	case float64:
		return lua.LNumber(t)
	case string:
		return lua.LString(t)
	case []byte:
		return lua.LString(string(t))
	case time.Time:
		return lua.LString(t.Format(time.RFC3339))
	default:
		return lua.LString(fmt.Sprint(t))
	}
}

func (c *codec) args(v lua.LValue) ([]any, error) {
	if v == nil || v == lua.LNil {
		return nil, nil
	}
	if tbl, ok := v.(*lua.LTable); ok {
		n := tbl.Len()
		args := make([]any, 0, n)
		for i := 1; i <= n; i++ {
			args = append(args, c.toGo(tbl.RawGetInt(i)))
		}
		return args, nil
	}
	return []any{c.toGo(v)}, nil
}

func (c *codec) toGo(v lua.LValue) any {
	if c.null != nil && v == c.null {
		return nil
	}
	switch t := v.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(t)
	case lua.LNumber:
		f := float64(t)
		if f == math.Trunc(f) && math.Abs(f) < 1<<62 {
			return int64(f)
		}
		return f
	case lua.LString:
		return string(t)
	default:
		return v.String()
	}
}

func luaCtx(L *lua.LState) context.Context {
	if ctx := L.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}
