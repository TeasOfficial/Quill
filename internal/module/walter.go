package module

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"quill/internal/docserver"
	"quill/internal/logc"

	"github.com/fsnotify/fsnotify"
)

type Builder struct {
	host       *Host
	modulesDir string
	watcher    *fsnotify.Watcher
	pending    map[string]bool
	mu         sync.Mutex
}

type ModuleConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Homepage    string `json:"homepage"`
	License     string `json:"license"`
}

func NewBuilder(host *Host, modulesDir string) *Builder {
	return &Builder{
		host:       host,
		modulesDir: modulesDir,
		pending:    make(map[string]bool),
	}
}

func (b *Builder) Start() error {
	var err error
	b.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}

	err = filepath.Walk(b.modulesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			b.watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk modules dir: %w", err)
	}

	go b.watchLoop()
	go b.debounceLoop()

	return nil
}

func (b *Builder) watchLoop() {
	for {
		select {
		case event, ok := <-b.watcher.Events:
			if !ok {
				return
			}
			if !strings.HasSuffix(event.Name, ".go") {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				b.schedule(dirName(event.Name))
			}
		case err, ok := <-b.watcher.Errors:
			if !ok {
				return
			}
			logc.Fmt("!", logc.Red, "\u6587\u4ef6\u76d1\u542c\u9519\u8bef: %v", err)
		}
	}
}

func (b *Builder) schedule(name string) {
	b.mu.Lock()
	b.pending[name] = true
	b.mu.Unlock()
}

func (b *Builder) debounceLoop() {
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		b.mu.Lock()
		modules := make([]string, 0, len(b.pending))
		for name := range b.pending {
			modules = append(modules, name)
			delete(b.pending, name)
		}
		b.mu.Unlock()

		for _, name := range modules {
			b.buildAndReload(name)
		}
	}
}

func (b *Builder) buildAndReload(name string) {
	modDir := filepath.Join(b.modulesDir, name)
	mainGo := filepath.Join(modDir, "main.go")
	wasmFile := filepath.Join(modDir, name+".wasm")

	cfg := b.loadConfig(modDir)

	if _, err := os.Stat(mainGo); err == nil {
		logc.Fmt("\u2699", logc.Gray, "\u7f16\u8bd1\u6a21\u5757 %s...", name)
		if err := b.compileModule(name, modDir, mainGo, wasmFile); err != nil {
			logc.Fmt("\u2715", logc.Red, "\u7f16\u8bd1\u5931\u8d25 %s: %v", name, err)
			return
		}
	} else if _, err := os.Stat(wasmFile); err == nil {
		logc.Fmt("\u25a1", logc.Gray, "\u52a0\u8f7d\u9884\u7f16\u8bd1\u6a21\u5757 %s...", name)
	} else {
		logc.Fmt("!", logc.Gray, "\u8df3\u8fc7 %s (\u65e0\u6e90\u7801/\u65e0wasm)", name)
		return
	}

	if _, err := b.host.Load(name, wasmFile); err != nil {
		logc.Fmt("\u2715", logc.Red, "\u52a0\u8f7d\u5931\u8d25 %s: %v", name, err)
		return
	}

	logc.Fmt("\u2713", logc.Cyan, "\u6a21\u5757\u5c31\u7eea: %s v%s @%s \u2014 %s", name, cfg.Version, cfg.Author, cfg.Description)

	b.registerDocs(name, modDir)
}

func (b *Builder) registerDocs(name, modDir string) {
	entries, err := os.ReadDir(modDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(modDir, e.Name()))
		if err != nil {
			continue
		}
		pageName := strings.TrimSuffix(e.Name(), ".md")
		path := "module/" + name
		if pageName != "README" {
			path += "/" + pageName
		}
		docserver.RegisterPage(path+".md", string(content))
	}
}

func (b *Builder) compileModule(name, modDir, mainGo, wasmFile string) error {
	modRoot := findModuleRoot(b.modulesDir)
	cmd := exec.Command("go", "build", "-o", wasmFile, "."+string(filepath.Separator)+mainGo)
	if modRoot != "" {
		cmd.Dir = modRoot
	}
	cmd.Env = append(os.Environ(),
		"GOOS=wasip1",
		"GOARCH=wasm",
		"GOPROXY=https://goproxy.cn,direct",
	)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()
	return cmd.Run()
}

func (b *Builder) BuildAll() {
	entries, err := os.ReadDir(b.modulesDir)
	if err != nil {
		logc.Fmt("!", logc.Yellow, "\u8bfb\u53d6\u6a21\u5757\u76ee\u5f55\u5931\u8d25: %v", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		b.buildAndReload(entry.Name())
	}
}

func (b *Builder) loadConfig(dir string) *ModuleConfig {
	data, err := os.ReadFile(filepath.Join(dir, "module.json"))
	if err != nil {
		return nil
	}
	var cfg ModuleConfig
	json.Unmarshal(data, &cfg)
	return &cfg
}

func (b *Builder) Close() {
	if b.watcher != nil {
		b.watcher.Close()
	}
}

func findModuleRoot(start string) string {
	abs, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			return abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

func dirName(path string) string {
	dir := filepath.Dir(path)
	return filepath.Base(dir)
}
