package module

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"quill/internal/logc"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

var ctx = context.Background()

type Module struct {
	Name    string
	wasmMod api.Module
	exports map[string]api.Function
	mu      sync.Mutex
}

type Host struct {
	runtime wazero.Runtime
	modules map[string]*Module
	mu      sync.RWMutex
}

func NewHost() *Host {
	rt := wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	registerHostModule(rt)
	return &Host{
		runtime: rt,
		modules: make(map[string]*Module),
	}
}

func (h *Host) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.modules {
		m.wasmMod.Close(ctx)
	}
	h.runtime.Close(ctx)
}

func (h *Host) Load(name, wasmPath string) (*Module, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if old, ok := h.modules[name]; ok {
		old.wasmMod.Close(ctx)
		delete(h.modules, name)
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("read wasm: %w", err)
	}

	config := wazero.NewModuleConfig().
		WithName(name).
		WithStdout(log.Writer()).
		WithStderr(log.Writer())

	wasmMod, err := h.runtime.InstantiateWithConfig(ctx, wasmBytes, config)
	if err != nil {
		return nil, fmt.Errorf("instantiate wasm: %w", err)
	}

	mem := wasmMod.Memory()
	if mem == nil {
		wasmMod.Close(ctx)
		return nil, fmt.Errorf("module %s: no exported memory", name)
	}

	exportMap := make(map[string]api.Function)
	for fnName := range wasmMod.ExportedFunctionDefinitions() {
		if fnName == "_start" || fnName == "_initialize" || fnName == "memory" || fnName == "resume" {
			continue
		}
		fn := wasmMod.ExportedFunction(fnName)
		if fn != nil {
			exportMap[fnName] = fn
		}
	}

	mod := &Module{
		Name:    name,
		wasmMod: wasmMod,
		exports: exportMap,
	}
	h.modules[name] = mod
	logc.Fmt("\u25c6", logc.Cyan, "\u6a21\u5757\u5df2\u52a0\u8f7d: %s (%d functions)", name, len(exportMap))
	return mod, nil
}

func (m *Module) Call(funcName, argJSON string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fn, ok := m.exports[funcName]
	if !ok {
		return "", fmt.Errorf("%s.%s: function not found", m.Name, funcName)
	}

	mem := m.wasmMod.Memory()
	argBytes := []byte(argJSON)

	mallocFn := m.wasmMod.ExportedFunction("malloc")
	if mallocFn == nil {
		return "", fmt.Errorf("%s: no malloc export", m.Name)
	}

	results, err := mallocFn.Call(ctx, uint64(len(argBytes)+1))
	if err != nil {
		return "", fmt.Errorf("%s.malloc: %w", m.Name, err)
	}
	argPtr := uint32(results[0])

	mem.Write(argPtr, append(argBytes, 0))

	defer func() {
		if freeFn := m.wasmMod.ExportedFunction("free"); freeFn != nil {
			freeFn.Call(ctx, uint64(argPtr))
		}
	}()

	results, err = fn.Call(ctx, uint64(argPtr), uint64(len(argBytes)))
	if err != nil {
		return "", fmt.Errorf("%s.%s: %w", m.Name, funcName, err)
	}

	packed := results[0]
	resultPtr := uint32(packed >> 32)
	resultLen := uint32(packed & 0xFFFFFFFF)

	if resultPtr == 0 || resultLen == 0 {
		return "", nil
	}

	resultBytes, ok := mem.Read(resultPtr, resultLen)
	if !ok {
		return "", fmt.Errorf("%s.%s: failed to read result", m.Name, funcName)
	}

	return string(resultBytes), nil
}

func (h *Host) Replace(name string, mod *Module) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.modules[name]; ok {
		old.wasmMod.Close(ctx)
	}
	h.modules[name] = mod
}

func (h *Host) Has(name string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.modules[name]
	return ok
}
