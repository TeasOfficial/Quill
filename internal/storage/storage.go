package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type DB struct {
	dir string
}

func New(dir string) (*DB, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	return &DB{dir: dir}, nil
}

func (db *DB) Set(key string, value lua.LValue) error {
	if strings.Contains(key, "..") {
		return fmt.Errorf("key contains '..' not allowed")
	}
	path := filepath.Join(db.dir, key+".json")

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data := luaToGo(value)
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, bytes, 0644)
}

func (db *DB) Get(L *lua.LState, key string) (lua.LValue, error) {
	if strings.Contains(key, "..") {
		return lua.LNil, fmt.Errorf("key contains '..' not allowed")
	}
	path := filepath.Join(db.dir, key+".json")

	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return lua.LNil, nil
		}
		return lua.LNil, err
	}

	var data interface{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return lua.LNil, fmt.Errorf("unmarshal: %w", err)
	}
	return goToLua(L, data), nil
}

func (db *DB) Del(key string) error {
	if strings.Contains(key, "..") {
		return fmt.Errorf("key contains '..' not allowed")
	}
	path := filepath.Join(db.dir, key+".json")
	return os.Remove(path)
}

func luaToGo(v lua.LValue) interface{} {
	switch val := v.(type) {
	case *lua.LTable:
		arr := make(map[string]interface{})
		maxIdx := 0
		val.ForEach(func(k, v lua.LValue) {
			if k.Type() == lua.LTNumber {
				idx := int(k.(lua.LNumber))
				if idx > maxIdx {
					maxIdx = idx
				}
			}
			arr[k.String()] = luaToGo(v)
		})

		if maxIdx > 0 && maxIdx == val.Len() {
			list := make([]interface{}, maxIdx)
			for i := 1; i <= maxIdx; i++ {
				list[i-1] = luaToGo(val.RawGetInt(i))
			}
			return list
		}
		return arr
	case lua.LString:
		return string(val)
	case lua.LNumber:
		return float64(val)
	case lua.LBool:
		return bool(val)
	default:
		return nil
	}
}

func goToLua(L *lua.LState, v interface{}) lua.LValue {
	switch val := v.(type) {
	case map[string]interface{}:
		tbl := L.NewTable()
		for k, v := range val {
			tbl.RawSetString(k, goToLua(L, v))
		}
		return tbl
	case []interface{}:
		tbl := L.NewTable()
		for i, v := range val {
			tbl.RawSetInt(i+1, goToLua(L, v))
		}
		return tbl
	case float64:
		return lua.LNumber(val)
	case string:
		return lua.LString(val)
	case bool:
		return lua.LBool(val)
	default:
		return lua.LNil
	}
}
