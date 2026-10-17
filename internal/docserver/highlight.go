package docserver

import (
	"html/template"
	"strings"
	"unicode/utf8"
)

// 一个极简的、零依赖的语法高亮器。
// 只覆盖文档里实际用到的语言（lua/go/json/env/shell），
// 未知语言一律按纯文本处理（仅做 HTML 转义）。

type tokenKind int

const (
	tokPlain tokenKind = iota
	tokComment
	tokString
	tokKeyword
	tokNumber
	tokFunc
	tokType
)

func (k tokenKind) class() string {
	switch k {
	case tokComment:
		return "com"
	case tokString:
		return "str"
	case tokKeyword:
		return "kw"
	case tokNumber:
		return "num"
	case tokFunc:
		return "fn"
	case tokType:
		return "ty"
	default:
		return ""
	}
}

type stringSpec struct {
	open  string
	close string
	esc   bool
}

type langSpec struct {
	lineComments  []string
	blockComments [][2]string
	strings       []stringSpec
	keywords      map[string]bool
	iniKeys       bool
	jsonKeys      bool
}

func keywordSet(words string) map[string]bool {
	m := make(map[string]bool)
	for _, w := range strings.Fields(words) {
		m[w] = true
	}
	return m
}

var (
	goKeywords = keywordSet(`break case chan const continue default defer else
		fallthrough for func go goto if import interface map package range return
		select struct switch type var true false nil iota`)

	luaKeywords = keywordSet(`and break do else elseif end false for function goto
		if in local nil not or repeat return then true until while`)

	shKeywords = keywordSet(`if then else elif fi for while do done case esac
		function return local export in set echo exit`)

	jsonKeywords = keywordSet(`true false null`)
)

func langSpecFor(lang string) langSpec {
	l := strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(l, " \t,"); i >= 0 {
		l = l[:i]
	}
	l = strings.TrimPrefix(l, "language-")

	switch l {
	case "go", "golang":
		return langSpec{
			lineComments:  []string{"//"},
			blockComments: [][2]string{{"/*", "*/"}},
			strings: []stringSpec{
				{`"`, `"`, true},
				{`'`, `'`, true},
				{"`", "`", false},
			},
			keywords: goKeywords,
		}
	case "lua":
		return langSpec{
			lineComments:  []string{"--"},
			blockComments: [][2]string{{"--[[", "]]"}, {"--[==[", "]==]"}},
			strings: []stringSpec{
				{`"`, `"`, true},
				{`'`, `'`, true},
				{"[[", "]]", false},
			},
			keywords: luaKeywords,
		}
	case "sh", "bash", "shell", "zsh", "console", "powershell", "ps1", "ps":
		return langSpec{
			lineComments: []string{"#"},
			strings: []stringSpec{
				{`"`, `"`, true},
				{`'`, `'`, false},
			},
			keywords: shKeywords,
		}
	case "json":
		return langSpec{
			strings:  []stringSpec{{`"`, `"`, true}},
			keywords: jsonKeywords,
			jsonKeys: true,
		}
	case "env", "ini", "conf", "toml", "properties", "dotenv":
		return langSpec{
			lineComments: []string{"#", ";"},
			strings: []stringSpec{
				{`"`, `"`, true},
				{`'`, `'`, false},
			},
			iniKeys: true,
		}
	}
	return langSpec{}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// scanToken 从 code[i] 开始识别一个 token，返回其类型与结束位置（不含）。
// 任何异常输入都会退化为单字节 plain，保证不会死循环。
func scanToken(code string, i int, spec langSpec) (tokenKind, int) {
	rest := code[i:]

	for _, bc := range spec.blockComments {
		if strings.HasPrefix(rest, bc[0]) {
			body := rest[len(bc[0]):]
			if j := strings.Index(body, bc[1]); j >= 0 {
				return tokComment, i + len(bc[0]) + j + len(bc[1])
			}
			return tokComment, len(code)
		}
	}

	for _, lc := range spec.lineComments {
		if strings.HasPrefix(rest, lc) {
			end := len(code)
			if j := strings.IndexByte(rest, '\n'); j >= 0 {
				end = i + j
			}
			return tokComment, end
		}
	}

	for _, s := range spec.strings {
		if !strings.HasPrefix(rest, s.open) {
			continue
		}
		end := len(code)
		j := len(s.open)
		for j < len(rest) {
			if s.esc && rest[j] == '\\' {
				j += 2
				continue
			}
			if strings.HasPrefix(rest[j:], s.close) {
				end = i + j + len(s.close)
				break
			}
			j++
		}
		kind := tokString
		if spec.jsonKeys && s.open == `"` {
			k := end
			for k < len(code) && (code[k] == ' ' || code[k] == '\t') {
				k++
			}
			if k < len(code) && code[k] == ':' {
				kind = tokType
			}
		}
		return kind, end
	}

	c := code[i]
	if c >= '0' && c <= '9' {
		j := i + 1
		hex := false
		if c == '0' && j < len(code) && (code[j] == 'x' || code[j] == 'X') {
			hex = true
			j++
		}
		for j < len(code) {
			d := code[j]
			if (d >= '0' && d <= '9') || d == '_' || d == '.' {
				j++
				continue
			}
			if hex && ((d >= 'a' && d <= 'f') || (d >= 'A' && d <= 'F')) {
				j++
				continue
			}
			break
		}
		return tokNumber, j
	}

	if isIdentStart(c) {
		j := i + 1
		for j < len(code) && isIdentPart(code[j]) {
			j++
		}
		word := code[i:j]
		if spec.keywords[word] {
			return tokKeyword, j
		}
		k := j
		for k < len(code) && (code[k] == ' ' || code[k] == '\t') {
			k++
		}
		if spec.iniKeys && k < len(code) && code[k] == '=' {
			return tokType, j
		}
		if k < len(code) && code[k] == '(' {
			return tokFunc, j
		}
		return tokPlain, j
	}

	if c >= 0x80 {
		_, size := utf8.DecodeRuneInString(code[i:])
		if size <= 0 {
			size = 1
		}
		return tokPlain, i + size
	}
	return tokPlain, i + 1
}

// highlightHTML 把代码转成带 <span class="tok-*"> 的 HTML。
// 所有文本都经过 HTML 转义，返回的 template.HTML 是安全的。
func highlightHTML(code, lang string) template.HTML {
	spec := langSpecFor(lang)
	var b strings.Builder
	b.Grow(len(code) + len(code)/4)

	for i := 0; i < len(code); {
		kind, end := scanToken(code, i, spec)
		if end <= i {
			end = i + 1
		}
		if end > len(code) {
			end = len(code)
		}
		text := template.HTMLEscapeString(code[i:end])
		if kind == tokPlain {
			b.WriteString(text)
		} else {
			b.WriteString(`<span class="tok-`)
			b.WriteString(kind.class())
			b.WriteString(`">`)
			b.WriteString(text)
			b.WriteString(`</span>`)
		}
		i = end
	}
	return template.HTML(b.String())
}
