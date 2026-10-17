/* Quill Wiki — 前端交互 */
(function () {
  var root = document.documentElement;

  function read(key, fallback) {
    try {
      var v = localStorage.getItem(key);
      return v === null ? fallback : v;
    } catch (e) {
      return fallback;
    }
  }
  function write(key, value) {
    try { localStorage.setItem(key, value); } catch (e) {}
  }

  /* 首屏渲染前确定主题与模式，避免闪烁 */
  var theme = read('quill-theme', '');
  if (!theme) {
    theme = (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches) ? 'light' : 'dark';
  }
  root.setAttribute('data-theme', theme);

  var mode = read('quill-mode', 'basic');
  if (root.getAttribute('data-page-mode') === 'pro') mode = 'pro';
  root.setAttribute('data-mode', mode);

  function ready(fn) {
    if (document.readyState !== 'loading') fn();
    else document.addEventListener('DOMContentLoaded', fn);
  }

  ready(function () {
    var tocRefresh = function () {};

    /* ── 文档模式 ─────────────────────────────── */
    var modeBtns = document.querySelectorAll('[data-mode-btn]');

    function applyMode(next) {
      root.setAttribute('data-mode', next);
      write('quill-mode', next);
      for (var i = 0; i < modeBtns.length; i++) {
        var on = modeBtns[i].getAttribute('data-mode-btn') === next;
        modeBtns[i].setAttribute('aria-pressed', on ? 'true' : 'false');
      }
      tocRefresh();
    }
    for (var i = 0; i < modeBtns.length; i++) {
      (function (btn) {
        btn.addEventListener('click', function () {
          applyMode(btn.getAttribute('data-mode-btn'));
        });
      })(modeBtns[i]);
    }
    applyMode(root.getAttribute('data-mode') || 'basic');

    /* ── 主题切换 ─────────────────────────────── */
    var themeBtn = document.getElementById('themeBtn');
    if (themeBtn) {
      themeBtn.addEventListener('click', function () {
        var next = root.getAttribute('data-theme') === 'light' ? 'dark' : 'light';
        root.setAttribute('data-theme', next);
        write('quill-theme', next);
      });
    }

    /* ── 移动端侧栏 ───────────────────────────── */
    var menuBtn = document.getElementById('menuBtn');
    var sidebar = document.getElementById('sidebar');
    var overlay = document.getElementById('overlay');

    function setMenu(open) {
      if (!sidebar) return;
      sidebar.classList.toggle('open', open);
      if (overlay) overlay.hidden = !open;
      if (menuBtn) menuBtn.setAttribute('aria-expanded', open ? 'true' : 'false');
    }
    if (menuBtn) {
      menuBtn.addEventListener('click', function () {
        setMenu(!sidebar.classList.contains('open'));
      });
    }
    if (overlay) {
      overlay.addEventListener('click', function () { setMenu(false); });
    }

    /* ── 阅读进度 ─────────────────────────────── */
    var bar = document.getElementById('progress');
    function onScroll() {
      if (!bar) return;
      var doc = document.documentElement;
      var max = doc.scrollHeight - doc.clientHeight;
      var ratio = max > 0 ? (doc.scrollTop || document.body.scrollTop) / max : 0;
      if (ratio < 0) ratio = 0;
      if (ratio > 1) ratio = 1;
      bar.style.transform = 'scaleX(' + ratio + ')';
    }
    document.addEventListener('scroll', onScroll, { passive: true });
    onScroll();

    /* ── 标题锚点 ─────────────────────────────── */
    var heads = document.querySelectorAll('.markdown h2[id], .markdown h3[id]');
    for (var h = 0; h < heads.length; h++) {
      var a = document.createElement('a');
      a.className = 'anchor';
      a.href = '#' + heads[h].id;
      a.textContent = '#';
      a.setAttribute('aria-hidden', 'true');
      a.setAttribute('tabindex', '-1');
      heads[h].appendChild(a);
    }

    /* ── 代码复制 ─────────────────────────────── */
    function copyText(text, done) {
      function legacy() {
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.setAttribute('readonly', '');
        ta.style.position = 'fixed';
        ta.style.opacity = '0';
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); done(); } catch (e) {}
        document.body.removeChild(ta);
      }
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, legacy);
      } else {
        legacy();
      }
    }

    var pres = document.querySelectorAll('pre.code');
    for (var p = 0; p < pres.length; p++) {
      (function (pre) {
        var btn = document.createElement('button');
        btn.type = 'button';
        btn.className = 'copy-btn';
        btn.textContent = '复制';
        btn.setAttribute('aria-label', '复制代码');
        btn.addEventListener('click', function () {
          var code = pre.querySelector('code');
          copyText(code ? code.innerText : '', function () {
            btn.textContent = '已复制';
            setTimeout(function () { btn.textContent = '复制'; }, 1200);
          });
        });
        pre.appendChild(btn);
      })(pres[p]);
    }

    /* ── 页内目录滚动高亮 ─────────────────────── */
    var tocLinks = document.querySelectorAll('.toc a');
    var tocTargets = {};

    function tocIdOf(link) {
      var href = link.getAttribute('href') || '';
      if (href.charAt(0) !== '#') return '';
      var id = href.slice(1);
      try { id = decodeURIComponent(id); } catch (e) {}
      return id;
    }

    /* 隐藏被“文档模式”遮挡的小节（否则是死链接） */
    tocRefresh = function () {
      for (var i = 0; i < tocLinks.length; i++) {
        var id = tocIdOf(tocLinks[i]);
        if (!id) continue;
        var el = document.getElementById(id);
        tocLinks[i].style.display = (el && el.offsetParent === null) ? 'none' : '';
      }
    };
    tocRefresh();

    if (tocLinks.length && 'IntersectionObserver' in window) {
      for (var t = 0; t < tocLinks.length; t++) {
        var tid = tocIdOf(tocLinks[t]);
        if (tid) tocTargets[tid] = tocLinks[t];
      }
      var targets = [];
      for (var key in tocTargets) {
        if (!Object.prototype.hasOwnProperty.call(tocTargets, key)) continue;
        var el = document.getElementById(key);
        if (el) targets.push(el);
      }
      var setActive = function (activeId) {
        for (var u = 0; u < tocLinks.length; u++) tocLinks[u].classList.remove('active');
        if (tocTargets[activeId]) tocTargets[activeId].classList.add('active');
      };
      var io = new IntersectionObserver(function (entries) {
        for (var e = 0; e < entries.length; e++) {
          if (entries[e].isIntersecting) setActive(entries[e].target.id);
        }
      }, { rootMargin: '-64px 0px -72% 0px', threshold: 0 });
      for (var s = 0; s < targets.length; s++) io.observe(targets[s]);
    }

    /* ── 搜索 ─────────────────────────────────── */
    var input = document.getElementById('searchInput');
    var results = document.getElementById('searchResults');
    var indexPromise = null;
    var activeIdx = -1;
    var timer = null;

    function loadIndex() {
      if (!indexPromise) {
        indexPromise = fetch('/wiki-index.json')
          .then(function (r) { return r.json(); })
          .catch(function () { return []; });
      }
      return indexPromise;
    }

    function esc(s) {
      return String(s).replace(/[&<>"']/g, function (c) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
      });
    }

    function highlight(text, terms) {
      var out = esc(text);
      if (!terms.length) return out;
      var pattern = terms.map(function (t) {
        return t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      }).filter(Boolean).join('|');
      if (!pattern) return out;
      try {
        return out.replace(new RegExp('(' + pattern + ')', 'gi'), '<mark>$1</mark>');
      } catch (e) {
        return out;
      }
    }

    function items() {
      return results ? results.querySelectorAll('.search-item') : [];
    }

    function setActive(n) {
      var list = items();
      if (!list.length) return;
      activeIdx = (n + list.length) % list.length;
      for (var i = 0; i < list.length; i++) list[i].classList.toggle('active', i === activeIdx);
      list[activeIdx].scrollIntoView({ block: 'nearest' });
    }

    function hideResults() {
      if (results) results.hidden = true;
      activeIdx = -1;
    }

    function render(hits, terms) {
      if (!results) return;
      if (!hits.length) {
        results.innerHTML = '<div class="search-empty">没有匹配的文档</div>';
        results.hidden = false;
        return;
      }
      var html = '';
      for (var i = 0; i < hits.length; i++) {
        var it = hits[i];
        var section = it.s === 'pro' ? '专业版' : (it.s === 'basic' ? '普通版' : '文档');
        var text = it.x || '';
        var lower = text.toLowerCase();
        var pos = terms.length ? lower.indexOf(terms[0]) : -1;
        var snippet = '';
        if (pos >= 0) {
          var start = Math.max(0, pos - 24);
          if (start > 0) {
            var sp = text.indexOf(' ', start);
            if (sp >= 0 && sp < pos) start = sp + 1;
          }
          snippet = (start > 0 ? '…' : '') + text.slice(start, pos + 72) + '…';
        } else if (it.h && it.h.length) {
          snippet = it.h.slice(0, 3).join(' · ');
        }
        html += '<a class="search-item" href="/' + it.p + '">' +
          '<div class="si-title">' + highlight(it.t, terms) + '</div>' +
          '<div class="si-meta">' + section + '</div>' +
          (snippet ? '<div class="si-snippet">' + highlight(snippet, terms) + '</div>' : '') +
          '</a>';
      }
      results.innerHTML = html;
      results.hidden = false;
      activeIdx = -1;
    }

    function search(query) {
      var terms = query.toLowerCase().split(/\s+/).filter(Boolean);
      if (!terms.length) {
        hideResults();
        return;
      }
      loadIndex().then(function (data) {
        var scored = [];
        for (var i = 0; i < data.length; i++) {
          var it = data[i];
          var title = (it.t || '').toLowerCase();
          var heads = (it.h || []).join(' ').toLowerCase();
          var body = (it.x || '').toLowerCase();
          var score = 0;
          for (var j = 0; j < terms.length; j++) {
            if (title.indexOf(terms[j]) >= 0) score += 12;
            if (heads.indexOf(terms[j]) >= 0) score += 5;
            if (body.indexOf(terms[j]) >= 0) score += 2;
          }
          if (score > 0) scored.push({ it: it, score: score });
        }
        scored.sort(function (a, b) { return b.score - a.score; });
        var top = scored.slice(0, 10).map(function (s) { return s.it; });
        render(top, terms);
      });
    }

    if (input && results) {
      input.addEventListener('input', function () {
        clearTimeout(timer);
        var q = input.value.trim();
        timer = setTimeout(function () { search(q); }, 120);
      });
      input.addEventListener('focus', function () {
        if (input.value.trim()) search(input.value.trim());
      });
      input.addEventListener('keydown', function (e) {
        if (e.key === 'ArrowDown') { e.preventDefault(); setActive(activeIdx + 1); }
        else if (e.key === 'ArrowUp') { e.preventDefault(); setActive(activeIdx - 1); }
        else if (e.key === 'Enter') {
          var list = items();
          if (!list.length) return;
          e.preventDefault();
          (list[activeIdx >= 0 ? activeIdx : 0]).click();
        } else if (e.key === 'Escape') {
          hideResults();
          input.blur();
        }
      });
      input.addEventListener('blur', function () {
        setTimeout(hideResults, 150);
      });
      document.addEventListener('click', function (e) {
        if (!results.contains(e.target) && e.target !== input) hideResults();
      });
      var searchBtn = document.getElementById('searchBtn');
      if (searchBtn) {
        searchBtn.addEventListener('click', function () {
          if (sidebar && !sidebar.classList.contains('open')) setMenu(true);
          input.focus();
          input.select();
        });
      }
    }

    /* ── 快捷键 ───────────────────────────────── */
    document.addEventListener('keydown', function (e) {
      var tag = (e.target && e.target.tagName || '').toLowerCase();
      if (tag === 'input' || tag === 'textarea' || e.metaKey || e.ctrlKey || e.altKey) return;
      if (e.key === '/') {
        e.preventDefault();
        if (input) {
          if (sidebar && window.innerWidth <= 900) setMenu(true);
          input.focus();
        }
      }
    });
  });
})();
