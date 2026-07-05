package docserver

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"

	"quill/internal/logc"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed docs/**
var DocsFS embed.FS

var md = goldmark.New(
	goldmark.WithExtensions(extension.Table),
)

var (
	dynamicPages   = make(map[string]string)
	dynamicPagesMu sync.RWMutex
)

func RegisterPage(path, content string) {
	dynamicPagesMu.Lock()
	dynamicPages[path] = content
	dynamicPagesMu.Unlock()
	logc.Fmt("\u2637", logc.Gray, "Wiki: \u6ce8\u518c\u6a21\u5757\u9875 /%s", path)
}

func UnregisterPage(path string) {
	dynamicPagesMu.Lock()
	delete(dynamicPages, path)
	dynamicPagesMu.Unlock()
}

type Page struct {
	Title    string
	Content  template.HTML
	Sections []NavSection
}

type NavSection struct {
	Name  string
	Title string
	Pages []NavLink
}

type NavLink struct {
	Title string
	Href  string
}

var sectionNames = map[string]string{
	"basic": "普通版",
	"pro":   "专业版",
}

func titleOf(section string) string {
	if t, ok := sectionNames[section]; ok {
		return t
	}
	return section
}

func Serve(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", servePage)
	server := &http.Server{Addr: addr, Handler: mux}
	logc.Fmt("\u2637", logc.Gray, "Wiki: http://localhost%s", addr)
	return server.ListenAndServe()
}

func servePage(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index"
	}
	path = strings.TrimPrefix(path, "/")

	sections := listSections()

	var content []byte
	var err error

	dynamicPagesMu.RLock()
	mdText, isDynamic := dynamicPages[path+".md"]
	dynamicPagesMu.RUnlock()

	if isDynamic {
		content = []byte(mdText)
	} else {
		content, err = DocsFS.ReadFile("docs/" + path + ".md")
	}

	if err != nil || (content == nil && !isDynamic) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Page not found: %s", path)
		return
	}

	var buf bytes.Buffer
	if err := md.Convert(content, &buf); err != nil {
		http.Error(w, "Render error", http.StatusInternalServerError)
		return
	}

	page := Page{
		Title:    path,
		Content:  template.HTML(buf.String()),
		Sections: sections,
	}

	tmpl := template.Must(template.New("page").Parse(tplHTML))
	tmpl.Execute(w, page)
}

func listSections() []NavSection {
	dirs, _ := DocsFS.ReadDir("docs")
	var sections []NavSection

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		title := sectionNames[d.Name()]
		if title == "" {
			title = d.Name()
		}

		files, _ := DocsFS.ReadDir("docs/" + d.Name())
		var pages []NavLink
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(f.Name(), ".md")
			content, err := DocsFS.ReadFile("docs/" + d.Name() + "/" + f.Name())
			if err != nil {
				continue
			}
			pageTitle := extractH1(string(content))
			if pageTitle == "" {
				pageTitle = name
			}
			pages = append(pages, NavLink{
				Title: pageTitle,
				Href:  "/" + d.Name() + "/" + name,
			})
		}
		sort.Slice(pages, func(i, j int) bool {
			return pages[i].Title < pages[j].Title
		})
		sections = append(sections, NavSection{
			Name:  d.Name(),
			Title: title,
			Pages: pages,
		})
	}

	dynamicPagesMu.RLock()
	if len(dynamicPages) > 0 {
		var modulePages []NavLink
		for path, content := range dynamicPages {
			pageTitle := extractH1(content)
			if pageTitle == "" {
				pageTitle = strings.TrimSuffix(path, ".md")
			}
			modulePages = append(modulePages, NavLink{
				Title: pageTitle,
				Href:  "/" + strings.TrimSuffix(path, ".md"),
			})
		}
		sort.Slice(modulePages, func(i, j int) bool {
			return modulePages[i].Title < modulePages[j].Title
		})
		sections = append(sections, NavSection{
			Name:  "\u6a21\u5757",
			Title: "\u6a21\u5757\u6587\u6863",
			Pages: modulePages,
		})
	}
	dynamicPagesMu.RUnlock()

	return sections
}

func extractH1(md string) string {
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

const tplHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}} — Quill Wiki</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; background: #0d1117; color: #c9d1d9; display: flex; min-height: 100vh; }
nav { width: 240px; background: #161b22; padding: 20px; border-right: 1px solid #30363d; flex-shrink: 0; overflow-y: auto; }
nav h2 { color: #58a6ff; margin-bottom: 16px; font-size: 16px; }
nav a { display: block; color: #8b949e; text-decoration: none; padding: 5px 8px; border-radius: 6px; font-size: 13px; }
nav a:hover { color: #c9d1d9; background: #21262d; }
nav .nav-section { color: #484f58; font-size: 11px; text-transform: uppercase; padding: 12px 8px 4px; letter-spacing: 1px; }
.mode-toggle { padding: 6px 10px; background: #21262d; border-radius: 6px; display: flex; align-items: center; gap: 8px; margin-bottom: 16px; }
.mode-toggle span { font-size: 12px; color: #8b949e; cursor: pointer; padding: 2px 8px; border-radius: 4px; }
.mode-toggle span.active { color: #fff; background: #30363d; }
main { flex: 1; padding: 40px 48px; max-width: 860px; }
main h1 { color: #f0f6fc; font-size: 28px; border-bottom: 1px solid #21262d; padding-bottom: 8px; margin-bottom: 24px; }
main h2 { color: #f0f6fc; font-size: 20px; margin: 32px 0 12px; }
main h3 { color: #f0f6fc; font-size: 16px; margin: 24px 0 8px; }
main p { line-height: 1.7; margin-bottom: 12px; }
main code { background: #161b22; padding: 2px 6px; border-radius: 4px; font-family: "SF Mono", "Fira Code", "Cascadia Code", monospace; font-size: 13px; color: #d2a8ff; }
main pre { background: #161b22; padding: 16px; border-radius: 8px; overflow-x: auto; margin: 12px 0; border: 1px solid #30363d; }
main pre code { background: none; padding: 0; color: #c9d1d9; }
main ul, main ol { margin: 8px 0 12px 24px; line-height: 1.7; }
main a { color: #58a6ff; }
main table { border-collapse: collapse; width: 100%; margin: 16px 0; }
main th, main td { border: 1px solid #30363d; padding: 8px 12px; text-align: left; font-size: 14px; }
main th { background: #161b22; }
main hr { border: none; border-top: 1px solid #21262d; margin: 32px 0; }
.pro-only { display: none; }
.basic-only { display: block; }
body.pro .pro-only { display: block; }
body.pro .basic-only { display: none; }
</style>
</head>
<body class="basic">
<nav>
  <h2>Quill Wiki</h2>
  <div class="mode-toggle">
    <span class="active" onclick="setMode('basic')">普通</span>
    <span onclick="setMode('pro')">专业</span>
  </div>
  {{range .Sections}}
  <div class="nav-section">{{.Title}}</div>
  {{range .Pages}}
  <a href="{{.Href}}">{{.Title}}</a>
  {{end}}
  {{end}}
</nav>
<main>{{.Content}}</main>
<script>
function setMode(m) {
  document.body.className = m;
  document.querySelectorAll('.mode-toggle span').forEach(s => s.classList.toggle('active', s.textContent === (m==='basic'?'普通':'专业')));
  localStorage.setItem('wiki-mode', m);
}
var saved = localStorage.getItem('wiki-mode');
if (saved) setMode(saved);
</script>
</body>
</html>`
