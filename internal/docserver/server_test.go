package docserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
)

func request(t *testing.T, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newHandler().ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestIndexPageRenders(t *testing.T) {
	rec := request(t, http.MethodGet, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Quill Wiki", "<main", "文档首页", `<div class="basic-only">`} {
		if !strings.Contains(body, want) {
			t.Errorf("index page missing %q", want)
		}
	}
	if strings.Contains(body, "raw HTML omitted") {
		t.Error("index page still drops raw HTML blocks")
	}
}

func TestAllDocsReachable(t *testing.T) {
	s, err := getSite()
	if err != nil {
		t.Fatalf("build site: %v", err)
	}
	if len(s.pages) < 10 {
		t.Fatalf("only %d pages indexed", len(s.pages))
	}
	for key := range s.pages {
		target := "/" + key
		if key == "index" {
			target = "/"
		}
		rec := request(t, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", target, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("GET %s content-type = %q", target, ct)
		}
	}
}

func TestTOCAndChineseHeadingIDs(t *testing.T) {
	body := request(t, http.MethodGet, "/basic/plugin").Body.String()
	if !strings.Contains(body, `id="插件写法"`) {
		t.Error("missing Chinese heading id")
	}
	if !strings.Contains(body, `class="toc"`) {
		t.Error("missing table of contents")
	}
	if !strings.Contains(body, "toc-l2") || !strings.Contains(body, "toc-l3") {
		t.Error("toc should contain level 2 and 3 entries")
	}
}

func TestProPageMode(t *testing.T) {
	body := request(t, http.MethodGet, "/pro/overview").Body.String()
	if !strings.Contains(body, `data-page-mode="pro"`) {
		t.Error("pro page should expose data-page-mode=pro")
	}
}

func TestNotFound(t *testing.T) {
	rec := request(t, http.MethodGet, "/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "页面不存在") {
		t.Error("404 page should render the themed layout")
	}
	if !strings.Contains(rec.Body.String(), "文档首页") {
		t.Error("404 page should keep the navigation")
	}
}

func TestTrailingSlashRedirect(t *testing.T) {
	rec := request(t, http.MethodGet, "/basic/plugin/")
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/basic/plugin" {
		t.Errorf("location = %q", loc)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := request(t, http.MethodPost, "/")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET, HEAD" {
		t.Errorf("allow = %q", allow)
	}
}

func TestSearchIndex(t *testing.T) {
	rec := request(t, http.MethodGet, "/wiki-index.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var items []searchItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if len(items) < 10 {
		t.Fatalf("only %d search items", len(items))
	}

	var found bool
	for _, it := range items {
		if it.Path != "basic/plugin" {
			continue
		}
		found = true
		if it.Title == "" {
			t.Error("search item missing title")
		}
		if len(it.Headings) == 0 {
			t.Error("search item missing headings")
		}
		if strings.Contains(it.Text, "<h2") || strings.Contains(it.Text, "</p>") {
			t.Error("search text should not contain HTML tags")
		}
	}
	if !found {
		t.Error("basic/plugin missing from search index")
	}
}

func TestAssets(t *testing.T) {
	cases := []struct{ path, ctype string }{
		{"/assets/wiki.css", "text/css"},
		{"/assets/wiki.js", "application/javascript"},
	}
	for _, c := range cases {
		rec := request(t, http.MethodGet, c.path)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", c.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, c.ctype) {
			t.Errorf("GET %s content-type = %q, want %q", c.path, ct, c.ctype)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("GET %s returned empty body", c.path)
		}
	}
	if rec := request(t, http.MethodGet, "/assets/nope.css"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown asset status = %d, want 404", rec.Code)
	}
}

func TestHighlightLanguages(t *testing.T) {
	lua := string(highlightHTML("local x = 1 -- note\nprint(\"hi\")", "lua"))
	for _, want := range []string{"tok-kw", "tok-num", "tok-com", "tok-str"} {
		if !strings.Contains(lua, want) {
			t.Errorf("lua highlight missing %s", want)
		}
	}

	goSrc := string(highlightHTML("func main() { s := `raw` }", "go"))
	if !strings.Contains(goSrc, "tok-kw") || !strings.Contains(goSrc, "tok-str") {
		t.Error("go highlight failed")
	}

	jsonSrc := string(highlightHTML(`{"a": 1, "b": true}`, "json"))
	if !strings.Contains(jsonSrc, "tok-ty") || !strings.Contains(jsonSrc, "tok-kw") {
		t.Error("json highlight failed")
	}

	plain := string(highlightHTML("some text", ""))
	if strings.Contains(plain, "<span") {
		t.Error("unknown language should stay plain")
	}
}

func TestHighlightEscapesHTML(t *testing.T) {
	got := string(highlightHTML("if a < b then end", "lua"))
	if !strings.Contains(got, "&lt;") {
		t.Error("HTML should be escaped")
	}
	if strings.Contains(got, "<b") {
		t.Error("raw tag leaked into highlighted output")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World!":       "hello-world",
		"bot.send_group_msg": "bot-send_group_msg",
		"插件开发":               "插件开发",
		"":                   "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHeadingIDsAreUnique(t *testing.T) {
	ids := newHeadingIDs()
	a := string(ids.Generate([]byte("Same"), ast.KindHeading))
	b := string(ids.Generate([]byte("Same"), ast.KindHeading))
	if a != "same" || b != "same-2" {
		t.Errorf("got %q and %q, want same and same-2", a, b)
	}
}

func TestPrevNextOrder(t *testing.T) {
	s, err := getSite()
	if err != nil {
		t.Fatalf("build site: %v", err)
	}
	prev, next := neighbors(s, "basic/plugin")
	if prev == nil || prev.Path != "basic/quickstart" {
		t.Errorf("prev = %+v", prev)
	}
	if next == nil || next.Path != "basic/message" {
		t.Errorf("next = %+v", next)
	}
}

func TestAssetVersioning(t *testing.T) {
	v := assetVersion()
	if len(v) != 10 {
		t.Fatalf("asset version %q should be 10 hex chars", v)
	}
	body := request(t, http.MethodGet, "/").Body.String()
	if !strings.Contains(body, "/assets/wiki.css?v=") {
		t.Error("css link should be versioned")
	}
	if !strings.Contains(body, "/assets/wiki.js?v=") {
		t.Error("js link should be versioned")
	}
	rec := request(t, http.MethodGet, "/assets/wiki.css?v="+v)
	if rec.Code != http.StatusOK {
		t.Fatalf("versioned asset status = %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("versioned asset should be immutable, got %q", cc)
	}
}
