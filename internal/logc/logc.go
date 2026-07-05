package logc

import (
	"fmt"
	"os"
	"strings"
)

var Enabled = isTerminal()

func isTerminal() bool {
	term := os.Getenv("TERM")
	if term == "dumb" || term == "" {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("ConEmuANSI") != "ON" && os.Getenv("ANSICON") == "" && strings.Contains(os.Getenv("TERM_PROGRAM"), "") {
		_, ok := os.LookupEnv("WT_SESSION")
		if !ok && os.Getenv("TERMINAL_EMULATOR") == "" {
			return false
		}
	}
	return true
}

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	magenta = "\033[35m"
	cyan   = "\033[36m"
	white  = "\033[37m"
	gray   = "\033[90m"
)

func Bold(s string) string    { return styled(bold, s) }
func Dim(s string) string     { return styled(dim, s) }
func Red(s string) string     { return styled(red, s) }
func Green(s string) string   { return styled(green, s) }
func Yellow(s string) string  { return styled(yellow, s) }
func Blue(s string) string    { return styled(blue, s) }
func Magenta(s string) string { return styled(magenta, s) }
func Cyan(s string) string    { return styled(cyan, s) }
func White(s string) string   { return styled(white, s) }
func Gray(s string) string    { return styled(gray, s) }

func Tag(tag, msg string) string {
	return fmt.Sprintf("%s%s%s %s", dim, tag, reset, msg)
}

func styled(code, s string) string {
	if !Enabled {
		return s
	}
	return code + s + reset
}

func Fmt(tag string, color func(string) string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if len(msg) > 500 {
		msg = msg[:497] + "..."
	}
	if Enabled {
		fmt.Printf("%s %s\n", color(tag), msg)
	} else {
		fmt.Printf("[%s] %s\n", tag, msg)
	}
}
