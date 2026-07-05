package module

import (
	"encoding/json"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

func RegisterLuaAPI(L *lua.LState, host *Host) {
	L.SetGlobal("module", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(1)
		mod, ok := host.modules[name]
		if !ok {
			L.Push(lua.LNil)
			return 1
		}

		tbl := L.NewTable()

		for fnName := range mod.exports {
			fnName := fnName
			L.SetField(tbl, fnName, L.NewFunction(func(L *lua.LState) int {
				nargs := L.GetTop()
				args := make([]interface{}, nargs)
				for i := 1; i <= nargs; i++ {
					args[i-1] = luaValueToGo(L.Get(i))
				}

				result, err := mod.Call(fnName, mustJSON(args))
				if err != nil {
					L.Push(lua.LNil)
					L.Push(lua.LString(err.Error()))
					return 2
				}

				L.Push(lua.LString(result))
				return 1
			}))
		}

		L.Push(tbl)
		return 1
	}))
}

func luaValueToGo(v lua.LValue) interface{} {
	switch val := v.(type) {
	case lua.LString:
		return string(val)
	case lua.LNumber:
		return float64(val)
	case lua.LBool:
		return bool(val)
	case *lua.LTable:
		return luaTableToMap(val)
	default:
		return v.String()
	}
}

func luaTableToMap(tbl *lua.LTable) map[string]interface{} {
	result := make(map[string]interface{})
	tbl.ForEach(func(k, v lua.LValue) {
		result[k.String()] = luaValueToGo(v)
	})
	return result
}

func mustJSON(v interface{}) string {
	if args, ok := v.([]interface{}); ok {
		if len(args) == 1 {
			return toJSONValue(args[0])
		}
		var parts []string
		for _, a := range args {
			parts = append(parts, toJSONValue(a))
		}
		return fmt.Sprint(parts)
	}
	return toJSONValue(v)
}

func toJSONValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case map[string]interface{}:
		b, _ := json.Marshal(val)
		return string(b)
	case float64:
		return fmt.Sprint(val)
	default:
		return fmt.Sprint(val)
	}
}
