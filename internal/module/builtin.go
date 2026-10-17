package module

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

const defaultHTTPTimeout = 15 * time.Minute

type dlTask struct {
	ID         string
	URL        string
	Path       string
	Name       string
	TotalSize  int64
	Downloaded int64
	StartTime  time.Time
	Done       bool
	Error      string
	mu         sync.Mutex
}

var (
	dlTasks   = make(map[string]*dlTask)
	dlTasksMu sync.RWMutex
	dlSeq     int64
)

func newDlTask(url, path string) *dlTask {
	dlSeq++
	dlTasksMu.Lock()
	defer dlTasksMu.Unlock()
	t := &dlTask{
		ID:        fmt.Sprintf("%d", dlSeq),
		URL:       url,
		Path:      path,
		Name:      filepath.Base(path),
		StartTime: time.Now(),
	}
	dlTasks[t.ID] = t
	return t
}

func (t *dlTask) setError(err string) {
	t.mu.Lock()
	t.Done = true
	t.Error = err
	t.mu.Unlock()
}

func (t *dlTask) markDone() {
	t.mu.Lock()
	t.Done = true
	t.TotalSize = t.Downloaded
	t.mu.Unlock()
}

type progressReader struct {
	reader io.Reader
	task   *dlTask
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if pr.task != nil && n > 0 {
		pr.task.mu.Lock()
		pr.task.Downloaded += int64(n)
		pr.task.mu.Unlock()
	}
	return n, err
}

func RegisterModuleUtil(L *lua.LState, utilTbl *lua.LTable) {
	utilTbl.RawSetString("http_get", L.NewFunction(builtinHTTPGet))
	utilTbl.RawSetString("http_post", L.NewFunction(builtinHTTPPost))
	utilTbl.RawSetString("http_request", L.NewFunction(builtinHTTPRequest))
	utilTbl.RawSetString("download_file", L.NewFunction(builtinDownloadFile))
	utilTbl.RawSetString("download_poll", L.NewFunction(builtinDownloadPoll))
	utilTbl.RawSetString("download_list", L.NewFunction(builtinDownloadList))
	utilTbl.RawSetString("extract_archive", L.NewFunction(builtinExtractArchive))
}

func builtinHTTPGet(L *lua.LState) int {
	url := L.CheckString(1)
	resp := doHTTPRequest("GET", url, nil, "", defaultHTTPTimeout)
	L.Push(lua.LString(resp))
	return 1
}

func builtinHTTPPost(L *lua.LState) int {
	url := L.CheckString(1)
	resp := doHTTPRequest("POST", url, nil, "", defaultHTTPTimeout)
	L.Push(lua.LString(resp))
	return 1
}

func builtinHTTPRequest(L *lua.LState) int {
	raw := L.CheckString(1)
	var req struct {
		Method  string            `json:"method"`
		URL     string            `json:"url"`
		Headers map[string]string `json:"headers,omitempty"`
		Body    string            `json:"body,omitempty"`
		Timeout int               `json:"timeout,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString("invalid request: " + err.Error()))
		return 2
	}
	if req.Method == "" {
		req.Method = "GET"
	}
	t := time.Duration(req.Timeout) * time.Second
	if t <= 0 {
		t = defaultHTTPTimeout
	}
	resp := doHTTPRequest(req.Method, req.URL, req.Headers, req.Body, t)
	L.Push(lua.LString(resp))
	return 1
}

func builtinDownloadFile(L *lua.LState) int {
	url := L.CheckString(1)
	path := L.CheckString(2)
	threads := 4
	if L.GetTop() >= 3 {
		threads = L.CheckInt(3)
	}
	if threads <= 0 {
		threads = 4
	}

	os.MkdirAll(filepath.Dir(path), 0755)

	task := newDlTask(url, path)

	go func() {
		size, err := downloadFile(url, path, threads, task)
		if err != nil {
			task.setError(err.Error())
		} else {
			task.TotalSize = size
			task.markDone()
		}
	}()

	L.Push(lua.LString(task.ID))
	return 1
}

func builtinDownloadPoll(L *lua.LState) int {
	id := L.CheckString(1)

	dlTasksMu.RLock()
	t, ok := dlTasks[id]
	dlTasksMu.RUnlock()

	if !ok {
		L.Push(lua.LNil)
		return 1
	}

	tbl := taskToLuaTable(L, t)
	L.Push(tbl)
	return 1
}

func taskToLuaTable(L *lua.LState, t *dlTask) *lua.LTable {
	t.mu.Lock()
	done := t.Done
	total := t.TotalSize
	downloaded := t.Downloaded
	errStr := t.Error
	elapsed := time.Since(t.StartTime).Seconds()
	t.mu.Unlock()

	tbl := L.NewTable()
	tbl.RawSetString("id", lua.LString(t.ID))
	tbl.RawSetString("name", lua.LString(t.Name))
	tbl.RawSetString("done", lua.LBool(done))
	tbl.RawSetString("total", lua.LNumber(total))
	tbl.RawSetString("downloaded", lua.LNumber(downloaded))
	tbl.RawSetString("elapsed", lua.LNumber(elapsed))
	if errStr != "" {
		tbl.RawSetString("error", lua.LString(errStr))
	}
	var pct float64
	if total > 0 {
		pct = float64(downloaded) / float64(total) * 100
	}
	tbl.RawSetString("percent", lua.LNumber(pct))
	return tbl
}

func builtinExtractArchive(L *lua.LState) int {
	file := L.CheckString(1)
	dest := L.CheckString(2)

	os.MkdirAll(dest, 0755)

	count, err := extractArchive(file, dest)
	if err != nil {
		L.Push(lua.LNil)
		L.Push(lua.LString(err.Error()))
		return 2
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"ok":    true,
		"dest":  dest,
		"count": float64(count),
	})
	L.Push(lua.LString(string(resp)))
	return 1
}

func builtinDownloadList(L *lua.LState) int {
	dlTasksMu.RLock()
	defer dlTasksMu.RUnlock()

	now := time.Now()
	tbl := L.NewTable()
	idx := 1
	for _, t := range dlTasks {
		t.mu.Lock()
		done := t.Done
		total := t.TotalSize
		downloaded := t.Downloaded
		err := t.Error
		t.mu.Unlock()

		if done && now.Sub(t.StartTime) > 60*time.Second {
			continue
		}

		item := L.NewTable()
		item.RawSetString("id", lua.LString(t.ID))
		item.RawSetString("name", lua.LString(t.Name))
		item.RawSetString("url", lua.LString(t.URL))
		item.RawSetString("total", lua.LNumber(total))
		item.RawSetString("downloaded", lua.LNumber(downloaded))
		item.RawSetString("done", lua.LBool(done))
		if err != "" {
			item.RawSetString("error", lua.LString(err))
		}

		var pct float64
		if total > 0 {
			pct = float64(downloaded) / float64(total) * 100
		}
		item.RawSetString("percent", lua.LNumber(pct))

		elapsed := time.Since(t.StartTime).Seconds()
		item.RawSetString("elapsed", lua.LNumber(elapsed))

		tbl.RawSetInt(idx, item)
		idx++
	}
	L.Push(tbl)
	return 1
}

type httpRequest struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
}

type httpResponse struct {
	OK      bool              `json:"ok"`
	Status  int               `json:"status,omitempty"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func doHTTPRequest(method, url string, headers map[string]string, body string, timeout time.Duration) string {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return marshalResp(httpResponse{OK: false, Error: err.Error()})
	}

	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Quill/1.0")
	}

	res, err := client.Do(req)
	if err != nil {
		return marshalResp(httpResponse{OK: false, Error: err.Error()})
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(res.Body, 10*1024*1024))

	respHeaders := make(map[string]string)
	for k := range res.Header {
		respHeaders[k] = res.Header.Get(k)
	}

	return marshalResp(httpResponse{
		OK:      true,
		Status:  res.StatusCode,
		Body:    string(respBody),
		Headers: respHeaders,
	})
}

func marshalResp(r httpResponse) string {
	b, _ := json.Marshal(r)
	return string(b)
}

func downloadFile(url, path string, threads int, task *dlTask) (int64, error) {
	client := &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	headReq, _ := http.NewRequest("HEAD", url, nil)
	headReq.Header.Set("User-Agent", "Quill/1.0")
	headResp, err := client.Do(headReq)
	if err != nil {
		return streamDownload(url, path, task)
	}
	headResp.Body.Close()

	if task != nil {
		task.mu.Lock()
		task.TotalSize = headResp.ContentLength
		task.mu.Unlock()
	}

	contentLength := headResp.ContentLength
	acceptRanges := headResp.Header.Get("Accept-Ranges") == "bytes"

	if contentLength <= 0 || !acceptRanges || threads <= 1 || contentLength < 5*1024*1024 {
		return streamDownload(url, path, task)
	}

	return multiDownload(url, path, contentLength, threads)
}

func streamDownload(url, path string, task *dlTask) (int64, error) {
	client := &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if task != nil && resp.ContentLength > 0 {
		task.mu.Lock()
		task.TotalSize = resp.ContentLength
		task.mu.Unlock()
	}

	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	var reader io.Reader = resp.Body
	if task != nil {
		reader = &progressReader{reader: resp.Body, task: task}
	}

	n, err := io.Copy(f, reader)
	return n, err
}

func multiDownload(url, path string, total int64, threads int) (int64, error) {
	client := &http.Client{
		Timeout: defaultHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	chunkSize := total / int64(threads)
	var wg sync.WaitGroup
	errs := make([]error, threads)
	chunks := make([]string, threads)

	downloadChunk := func(i int) {
		defer wg.Done()
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == threads-1 {
			end = total - 1
		}

		tmpPath := path + fmt.Sprintf(".part%d", i)
		chunks[i] = tmpPath

		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
		req.Header.Set("User-Agent", "Quill/1.0")

		resp, err := client.Do(req)
		if err != nil {
			errs[i] = err
			return
		}
		defer resp.Body.Close()

		f, err := os.Create(tmpPath)
		if err != nil {
			errs[i] = err
			return
		}
		defer f.Close()

		_, errs[i] = io.Copy(f, resp.Body)
	}

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go downloadChunk(i)
	}
	wg.Wait()

	for _, e := range errs {
		if e != nil {
			cleanupChunks(chunks)
			return 0, fmt.Errorf("multi-download: %w", e)
		}
	}

	out, err := os.Create(path)
	if err != nil {
		cleanupChunks(chunks)
		return 0, fmt.Errorf("assemble: %w", err)
	}
	defer out.Close()

	var written int64
	for _, chunk := range chunks {
		f, err := os.Open(chunk)
		if err != nil {
			cleanupChunks(chunks)
			return 0, fmt.Errorf("read chunk: %w", err)
		}
		n, _ := io.Copy(out, f)
		written += n
		f.Close()
	}
	cleanupChunks(chunks)

	return written, nil
}

func cleanupChunks(chunks []string) {
	for _, c := range chunks {
		if c != "" {
			os.Remove(c)
		}
	}
}

func extractArchive(file, dest string) (int, error) {
	lower := strings.ToLower(file)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(file, dest)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(file, dest)
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"):
		return extractTarBz2(file, dest)
	case strings.HasSuffix(lower, ".tar"):
		return extractTar(file, dest)
	case strings.HasSuffix(lower, ".gz"):
		return extractGz(file, dest)
	case strings.HasSuffix(lower, ".bz2"):
		return extractBz2(file, dest)
	default:
		return 0, fmt.Errorf("unsupported archive format: %s", filepath.Ext(file))
	}
}

func extractZip(file, dest string) (int, error) {
	r, err := zip.OpenReader(file)
	if err != nil {
		return 0, err
	}
	defer r.Close()

	count := 0
	for _, f := range r.File {
		if err := writeZipEntry(f, dest); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func writeZipEntry(f *zip.File, dest string) error {
	target := filepath.Join(dest, f.Name)
	if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(filepath.Separator)) {
		return fmt.Errorf("zip entry escapes dest: %s", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0755)
	}
	os.MkdirAll(filepath.Dir(target), 0755)
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func extractTar(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return writeTarEntries(tar.NewReader(f), dest)
}

func extractTarGz(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gzr.Close()
	return writeTarEntries(tar.NewReader(gzr), dest)
}

func extractTarBz2(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return writeTarEntries(tar.NewReader(bzip2.NewReader(f)), dest)
}

func writeTarEntries(tr *tar.Reader, dest string) (int, error) {
	count := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		target := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(filepath.Separator)) {
			return count, fmt.Errorf("tar entry escapes dest: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.Create(target)
			if err != nil {
				return count, err
			}
			io.Copy(out, tr)
			out.Close()
		}
		count++
	}
	return count, nil
}

func extractGz(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return 0, err
	}
	defer gzr.Close()
	name := strings.TrimSuffix(filepath.Base(file), ".gz")
	if name == filepath.Base(file) {
		name += ".out"
	}
	out, err := os.Create(filepath.Join(dest, name))
	if err != nil {
		return 0, err
	}
	defer out.Close()
	io.Copy(out, gzr)
	return 1, nil
}

func extractBz2(file, dest string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	name := strings.TrimSuffix(filepath.Base(file), ".bz2")
	if name == filepath.Base(file) {
		name += ".out"
	}
	out, err := os.Create(filepath.Join(dest, name))
	if err != nil {
		return 0, err
	}
	defer out.Close()
	_, err = io.Copy(out, bzip2.NewReader(f))
	if err != nil {
		return 0, err
	}
	return 1, nil
}
