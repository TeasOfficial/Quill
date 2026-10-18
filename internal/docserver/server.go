package docserver

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"quill/internal/logc"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	ghhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

//go:embed docs/**
var DocsFS embed.FS

//go:embed static/*
var StaticFS embed.FS

var (
	assetVersionOnce sync.Once
	assetVersionVal  string
)

// assetVersion 由静态资源的哈希决定：升级 exe 后 URL 自动变化，
// 浏览器不会继续使用旧的 CSS/JS。
func assetVersion() string {
	assetVersionOnce.Do(func() {
		h := sha256.New()
		for _, name := range []string{"wiki.css", "wiki.js"} {
			if b, err := StaticFS.ReadFile("static/" + name); err == nil {
				_, _ = h.Write(b)
			}
		}
		assetVersionVal = fmt.Sprintf("%x", h.Sum(nil))[:10]
	})
	return assetVersionVal
}

type tocEntry struct {
	Level int
	Text  string
	ID    string
}

type docPage struct {
	Path    string
	Section string
	Title   string
	Mode    string
	HTML    template.HTML
	TOC     []tocEntry
	Plain   string
}

type navLink struct {
	Title string
	Href  string
	Path  string
}

type navSection struct {
	Title string
	Mode  string
	Pages []navLink
}

type site struct {
	pages    map[string]*docPage
	sections []navSection
	index    []byte
}

var (
	siteOnce sync.Once
	siteVal  *site
	siteErr  error
)

// sectionDefs 同时决定导航分组与页内排序；
// 未列出的文档会按文件名排序追加到对应分组末尾。
var sectionDefs = []struct {
	Key   string
	Title string
	Order []string
}{
	{"basic", "普通版", []string{"quickstart", "plugin", "message", "storage", "sql", "bot_api", "util_api"}},
	{"pro", "专业版", []string{"overview", "onebot", "message", "plugin", "storage"}},
}

func getSite() (*site, error) {
	siteOnce.Do(func() {
		siteVal, siteErr = buildSite()
	})
	return siteVal, siteErr
}

func buildSite() (*site, error) {
	md := newMarkdown()
	s := &site{pages: make(map[string]*docPage)}

	homeSrc, err := DocsFS.ReadFile("docs/index.md")
	if err != nil {
		return nil, fmt.Errorf("读取首页: %w", err)
	}
	home, err := renderDoc(md, "index", "", homeSrc)
	if err != nil {
		return nil, fmt.Errorf("渲染首页: %w", err)
	}
	home.Mode = "basic"
	s.pages["index"] = home

	for _, def := range sectionDefs {
		names, err := listDocs(def.Key)
		if err != nil {
			return nil, err
		}
		ns := navSection{Title: def.Title, Mode: def.Key}
		for _, name := range orderDocs(names, def.Order) {
			key := def.Key + "/" + name
			src, err := DocsFS.ReadFile("docs/" + key + ".md")
			if err != nil {
				return nil, fmt.Errorf("读取 %s: %w", key, err)
			}
			p, err := renderDoc(md, key, def.Key, src)
			if err != nil {
				return nil, fmt.Errorf("渲染 %s: %w", key, err)
			}
			p.Mode = def.Key
			s.pages[key] = p
			ns.Pages = append(ns.Pages, navLink{Title: p.Title, Href: "/" + key, Path: key})
		}
		if len(ns.Pages) > 0 {
			s.sections = append(s.sections, ns)
		}
	}

	s.index = buildSearchIndex(s.pages)
	logc.Fmt("\u2637", logc.Gray, "Wiki: \u5df2\u7d22\u5f15 %d \u4e2a\u9875\u9762", len(s.pages))
	return s, nil
}

func newMarkdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(extension.Table),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(
			ghhtml.WithUnsafe(),
			renderer.WithNodeRenderers(util.Prioritized(&codeBlockRenderer{}, 100)),
		),
	)
}

func listDocs(section string) ([]string, error) {
	entries, err := DocsFS.ReadDir("docs/" + section)
	if err != nil {
		return nil, fmt.Errorf("读取文档目录 %s: %w", section, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".md"))
	}
	return names, nil
}

func orderDocs(names, preferred []string) []string {
	pos := make(map[string]int, len(preferred))
	for i, n := range preferred {
		pos[n] = i
	}
	sort.SliceStable(names, func(i, j int) bool {
		pi, oki := pos[names[i]]
		pj, okj := pos[names[j]]
		switch {
		case oki && okj:
			return pi < pj
		case oki:
			return true
		case okj:
			return false
		default:
			return names[i] < names[j]
		}
	})
	return names
}

// headingIDs 生成稳定、可读、对中文友好的标题锚点，
// 替代 goldmark 默认实现（后者会把非 ASCII 字符全部丢弃）。
type headingIDs struct {
	used map[string]int
	seq  int
}

func newHeadingIDs() *headingIDs {
	return &headingIDs{used: make(map[string]int)}
}

func (h *headingIDs) Generate(value []byte, _ ast.NodeKind) []byte {
	base := slugify(string(value))
	if base == "" {
		h.seq++
		base = fmt.Sprintf("section-%d", h.seq)
	}
	id := base
	if n := h.used[base]; n > 0 {
		h.used[base] = n + 1
		id = fmt.Sprintf("%s-%d", base, n+1)
	} else {
		h.used[base] = 1
	}
	return []byte(id)
}

func (h *headingIDs) Put(value []byte) {
	h.used[string(value)]++
}

func slugify(s string) string {
	var b strings.Builder
	dash := false
	separate := func() {
		if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	for _, r := range strings.TrimSpace(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'),
			r >= 0x4E00 && r <= 0x9FFF, // CJK
			r >= 0x3040 && r <= 0x30FF: // 日文假名
			b.WriteRune(r)
			dash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			dash = false
		case r == '_':
			b.WriteRune(r)
			dash = false
		default:
			separate()
		}
	}
	return strings.Trim(b.String(), "-")
}

func renderDoc(md goldmark.Markdown, key, section string, src []byte) (*docPage, error) {
	ctx := parser.NewContext(parser.WithIDs(newHeadingIDs()))
	node := md.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, src, node); err != nil {
		return nil, err
	}

	title := ""
	var toc []tocEntry
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		h, ok := n.(*ast.Heading)
		if !ok {
			return ast.WalkContinue, nil
		}
		txt := cleanHeadingText(string(h.Text(src)))
		switch h.Level {
		case 1:
			if title == "" {
				title = txt
			}
		case 2, 3:
			id := ""
			if v, ok := h.AttributeString("id"); ok {
				switch t := v.(type) {
				case []byte:
					id = string(t)
				case string:
					id = t
				}
			}
			if id != "" && txt != "" {
				toc = append(toc, tocEntry{Level: h.Level, Text: txt, ID: id})
			}
		}
		return ast.WalkContinue, nil
	})

	if title == "" {
		title = strings.TrimSuffix(path.Base(key), ".md")
	}

	out := buf.String()
	return &docPage{
		Path:    key,
		Section: section,
		Title:   title,
		HTML:    template.HTML(out),
		TOC:     toc,
		Plain:   plainText(out),
	}, nil
}

func cleanHeadingText(s string) string {
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "*", "")
	return strings.Join(strings.Fields(s), " ")
}

var tagRE = regexp.MustCompile(`<[^>]*>`)

func plainText(h string) string {
	s := tagRE.ReplaceAllString(h, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

type searchItem struct {
	Path     string   `json:"p"`
	Section  string   `json:"s"`
	Title    string   `json:"t"`
	Headings []string `json:"h"`
	Text     string   `json:"x"`
}

func buildSearchIndex(pages map[string]*docPage) []byte {
	keys := make([]string, 0, len(pages))
	for k := range pages {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	items := make([]searchItem, 0, len(keys))
	for _, k := range keys {
		p := pages[k]
		heads := make([]string, 0, len(p.TOC))
		for _, t := range p.TOC {
			heads = append(heads, t.Text)
		}
		items = append(items, searchItem{
			Path:     p.Path,
			Section:  p.Section,
			Title:    p.Title,
			Headings: heads,
			Text:     truncateRunes(p.Plain, 20000),
		})
	}
	b, err := json.Marshal(items)
	if err != nil {
		return []byte("[]")
	}
	return b
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func sectionTitle(key string) string {
	switch key {
	case "basic":
		return "普通版"
	case "pro":
		return "专业版"
	default:
		return "文档"
	}
}

func neighbors(s *site, key string) (prev, next *navLink) {
	var flat []navLink
	for _, sec := range s.sections {
		flat = append(flat, sec.Pages...)
	}
	idx := -1
	for i := range flat {
		if flat[i].Path == key {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, nil
	}
	if idx > 0 {
		prev = &flat[idx-1]
	}
	if idx < len(flat)-1 {
		next = &flat[idx+1]
	}
	return prev, next
}

type pageData struct {
	Title        string
	Path         string
	SectionTitle string
	Mode         string
	Content      template.HTML
	TOC          []tocEntry
	Prev         *navLink
	Next         *navLink
	Sections     []navSection
	Description  string
	Assets       string
}

var pageTemplate = template.Must(template.New("page").Parse(pageTemplateHTML))

func Serve(addr string) error {
	if _, err := getSite(); err != nil {
		return fmt.Errorf("构建 Wiki: %w", err)
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logc.Fmt("\u2637", logc.Gray, "Wiki: http://localhost%s", addr)
	return srv.ListenAndServe()
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/assets/", handleAsset)
	mux.HandleFunc("/wiki-index.json", handleSearchIndex)
	mux.HandleFunc("/", handlePage)
	return mux
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s, err := getSite()
	if err != nil {
		http.Error(w, "Wiki 初始化失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if p := r.URL.Path; p != "/" && strings.HasSuffix(p, "/") {
		target := strings.TrimRight(p, "/")
		if target == "" {
			target = "/"
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	key := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	switch key {
	case "", "index.html", "index.htm":
		key = "index"
	}

	page, ok := s.pages[key]
	if !ok {
		data := &pageData{
			Title:        "页面不存在",
			Path:         key,
			SectionTitle: "错误",
			Mode:         "basic",
			Sections:     s.sections,
			Content:      template.HTML(notFoundContent(key)),
			Description:  "页面不存在",
		}
		renderHTML(w, data, http.StatusNotFound)
		return
	}

	prev, next := neighbors(s, key)
	data := &pageData{
		Title:        page.Title,
		Path:         key,
		SectionTitle: sectionTitle(page.Section),
		Mode:         page.Mode,
		Content:      page.HTML,
		TOC:          page.TOC,
		Prev:         prev,
		Next:         next,
		Sections:     s.sections,
		Description:  truncateRunes(page.Plain, 120),
	}
	if data.Mode == "" {
		data.Mode = "basic"
	}
	renderHTML(w, data, http.StatusOK)
}

func notFoundContent(key string) string {
	return `<h1>页面不存在</h1>
<p>没有找到文档 <code>` + template.HTMLEscapeString(key) + `</code>。</p>
<p><a href="/">返回文档首页</a></p>`
}

func renderHTML(w http.ResponseWriter, data *pageData, status int) {
	data.Assets = assetVersion()
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, data); err != nil {
		http.Error(w, "渲染失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func handleSearchIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s, err := getSite()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(s.index)
}

func handleAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/")
	var ctype string
	switch name {
	case "wiki.css":
		ctype = "text/css; charset=utf-8"
	case "wiki.js":
		ctype = "application/javascript; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	data, err := StaticFS.ReadFile("static/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ctype)
	if r.URL.RawQuery != "" {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(data)
}
