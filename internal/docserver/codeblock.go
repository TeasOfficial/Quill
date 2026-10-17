package docserver

import (
	"html/template"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// codeBlockRenderer 覆盖 goldmark 默认的代码块渲染：
// 输出统一的外壳（语言标签 + 复制按钮由前端补齐）并做语法高亮。
type codeBlockRenderer struct{}

func (r *codeBlockRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.renderFenced)
	reg.Register(ast.KindCodeBlock, r.renderIndented)
}

func (r *codeBlockRenderer) renderFenced(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		lang := ""
		if n, ok := node.(*ast.FencedCodeBlock); ok {
			if l := n.Language(source); l != nil {
				lang = string(l)
			}
		}
		writeCodeBlock(w, lang, blockText(node, source))
	}
	return ast.WalkSkipChildren, nil
}

func (r *codeBlockRenderer) renderIndented(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		writeCodeBlock(w, "", blockText(node, source))
	}
	return ast.WalkSkipChildren, nil
}

func blockText(node ast.Node, source []byte) string {
	lines := node.Lines()
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(source))
	}
	return b.String()
}

func writeCodeBlock(w util.BufWriter, lang, code string) {
	display := strings.TrimSpace(lang)
	if i := strings.IndexAny(display, " \t,"); i >= 0 {
		display = display[:i]
	}
	if display == "" {
		display = "text"
	}
	_, _ = w.WriteString(`<pre class="code" data-lang="`)
	_, _ = w.WriteString(template.HTMLEscapeString(display))
	_, _ = w.WriteString(`"><code>`)
	_, _ = w.WriteString(string(highlightHTML(code, display)))
	_, _ = w.WriteString("</code></pre>\n")
}
