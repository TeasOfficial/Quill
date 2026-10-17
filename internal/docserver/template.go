package docserver

const pageTemplateHTML = `<!DOCTYPE html>
<html lang="zh-CN" data-theme="dark" data-mode="{{.Mode}}" data-page-mode="{{.Mode}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="color-scheme" content="dark light">
<title>{{.Title}} · Quill Wiki</title>
{{if .Description}}<meta name="description" content="{{.Description}}">{{end}}
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' rx='9' fill='%2358a6ff'/%3E%3Ctext x='16' y='23' font-family='ui-sans-serif,sans-serif' font-size='19' font-weight='700' text-anchor='middle' fill='%230d1117'%3EQ%3C/text%3E%3C/svg%3E">
<link rel="stylesheet" href="/assets/wiki.css?v={{.Assets}}">
<script src="/assets/wiki.js?v={{.Assets}}"></script>
</head>
<body>
<a class="skip-link" href="#content">跳到正文</a>
<div class="progress" id="progress" aria-hidden="true"></div>

<header class="topbar">
  <button class="icon-btn menu-btn" id="menuBtn" type="button" aria-label="打开导航" aria-expanded="false" aria-controls="sidebar">☰</button>
  <a class="brand" href="/"><span class="brand-mark">Q</span><span class="brand-name">Quill</span><span class="brand-sub">Wiki</span></a>
  <div class="topbar-spacer"></div>
  <button class="icon-btn" id="searchBtn" type="button" aria-label="搜索文档" title="搜索（/）">⌕</button>
  <button class="icon-btn" id="themeBtn" type="button" aria-label="切换明暗主题" title="切换明暗主题">◐</button>
</header>

<div class="layout">
  <aside class="sidebar" id="sidebar">
    <div class="search-wrap">
      <input id="searchInput" type="search" placeholder="搜索文档…" autocomplete="off" spellcheck="false" aria-label="搜索文档">
      <kbd class="search-kbd">/</kbd>
      <div class="search-results" id="searchResults" hidden></div>
    </div>

    <nav class="nav" aria-label="文档目录">
      <a class="nav-home{{if eq .Path "index"}} active{{end}}" href="/">文档首页</a>
      {{range .Sections}}
      <div class="nav-group" data-mode="{{.Mode}}">
        <div class="nav-title">{{.Title}}</div>
        {{range .Pages}}<a href="{{.Href}}"{{if eq .Path $.Path}} class="active" aria-current="page"{{end}}>{{.Title}}</a>{{end}}
      </div>
      {{end}}
    </nav>

    <div class="mode-switch" role="group" aria-label="文档模式">
      <button type="button" data-mode-btn="basic">普通</button>
      <button type="button" data-mode-btn="pro">专业</button>
    </div>
  </aside>

  <main class="content" id="content">
    <div class="crumbs">{{if .SectionTitle}}{{.SectionTitle}} <span>/</span> {{end}}{{.Title}}</div>
    <article class="markdown">{{.Content}}</article>

    {{if or .Prev .Next}}
    <nav class="pager" aria-label="上下篇">
      {{if .Prev}}<a class="pager-item pager-prev" href="{{.Prev.Href}}"><span class="pager-label">← 上一篇</span><span class="pager-title">{{.Prev.Title}}</span></a>{{end}}
      {{if .Next}}<a class="pager-item pager-next" href="{{.Next.Href}}"><span class="pager-label">下一篇 →</span><span class="pager-title">{{.Next.Title}}</span></a>{{end}}
    </nav>
    {{end}}

    <footer class="footer">Quill Wiki · 内置于 quill.exe 的文档服务</footer>
  </main>

  {{if .TOC}}
  <aside class="toc" aria-label="页内目录">
    <div class="toc-title">本页目录</div>
    <nav>{{range .TOC}}<a class="toc-l{{.Level}}" href="#{{.ID}}">{{.Text}}</a>{{end}}</nav>
  </aside>
  {{end}}
</div>

<div class="overlay" id="overlay" hidden></div>
</body>
</html>`
