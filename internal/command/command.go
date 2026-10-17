package command

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

type CmdFunc func(args []string)

type System struct {
	commands map[string]CmdFunc
	mu       sync.RWMutex
	done     chan struct{}
}

func New() *System {
	return &System{
		commands: make(map[string]CmdFunc),
		done:     make(chan struct{}),
	}
}

func (s *System) Register(name string, fn CmdFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands[name] = fn
}

func (s *System) Run() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println()
	fmt.Println("输入指令 (help 查看列表):")
	go func() {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			parts := strings.Fields(line)
			if len(parts) == 0 {
				continue
			}
			cmdName := parts[0]
			args := parts[1:]

			switch cmdName {
			case "help":
				s.printHelp()
			case "exit", "quit":
				fmt.Println("Bye.")
				os.Exit(0)
			default:
				s.mu.RLock()
				fn, ok := s.commands[cmdName]
				s.mu.RUnlock()
				if !ok {
					fmt.Printf("未知指令: %s (输入 help 查看列表)\n", cmdName)
					continue
				}
				fn(args)
			}
		}
	}()
}

func (s *System) printHelp() {
	fmt.Println()
	fmt.Println("可用指令:")
	fmt.Println("  help        - 显示此帮助")
	fmt.Println("  exit/quit   - 退出程序")
	s.mu.RLock()
	defer s.mu.RUnlock()
	for name := range s.commands {
		fmt.Printf("  %s\n", name)
	}
	fmt.Println()
}
