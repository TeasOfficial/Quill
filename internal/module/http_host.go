package module

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"quill/internal/logc"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

var _ = wazero.NewRuntime  // suppress unused import if needed

type httpReq struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"`
	Timeout int               `json:"timeout,omitempty"`
}

type httpResp struct {
	OK      bool              `json:"ok"`
	Status  int               `json:"status,omitempty"`
	Body    string            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Error   string            `json:"error,omitempty"`
}

func registerHostModule(rt wazero.Runtime) {
	hb := rt.NewHostModuleBuilder("quill")
	registerHTTP(hb)
	registerDownload(hb)
	registerExtract(hb)
	if _, err := hb.Instantiate(ctx); err != nil {
		logc.Fmt("!", logc.Red, "\u6ce8\u518c quill \u4e3b\u673a\u6a21\u5757\u5931\u8d25: %v", err)
	}
}

func registerHTTP(hb wazero.HostModuleBuilder) {
	hb.NewFunctionBuilder().WithFunc(safeHost(func(ctx context.Context, mod api.Module, reqPtr uint32, reqLen uint32) uint64 {
		return hostHTTPRequest(ctx, mod, reqPtr, reqLen)
	})).Export("http_request")
}

func registerDownload(hb wazero.HostModuleBuilder) {
	hb.NewFunctionBuilder().WithFunc(safeHost(func(ctx context.Context, mod api.Module, reqPtr uint32, reqLen uint32) uint64 {
		return hostDownload(ctx, mod, reqPtr, reqLen)
	})).Export("http_download")
}

func registerExtract(hb wazero.HostModuleBuilder) {
	hb.NewFunctionBuilder().WithFunc(safeHost(func(ctx context.Context, mod api.Module, reqPtr uint32, reqLen uint32) uint64 {
		return hostExtract(ctx, mod, reqPtr, reqLen)
	})).Export("archive_extract")
}

func safeHost(fn func(context.Context, api.Module, uint32, uint32) uint64) func(context.Context, api.Module, uint32, uint32) uint64 {
	return func(ctx context.Context, mod api.Module, a uint32, b uint32) (r uint64) {
		defer func() {
			if rec := recover(); rec != nil {
				r = packFail(mod, fmt.Sprintf("host panic: %v", rec))
			}
		}()
		return fn(ctx, mod, a, b)
	}
}

func hostHTTPRequest(ctx context.Context, mod api.Module, reqPtr uint32, reqLen uint32) uint64 {
	mem := mod.Memory()
	if mem == nil {
		return packErr(mod, "no exported memory")
	}

	reqBytes, ok := mem.Read(reqPtr, reqLen)
	if !ok {
		return packErr(mod, "failed to read request from wasm memory")
	}

	var req httpReq
	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return packErr(mod, "invalid request json: "+err.Error())
	}

	if req.Method == "" {
		req.Method = "GET"
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	resp := doHTTP(req)
	return packResp(mod, resp)
}

func doHTTP(req httpReq) httpResp {
	client := &http.Client{
		Timeout: time.Duration(req.Timeout) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	var bodyReader io.Reader
	if req.Body != "" {
		bodyReader = strings.NewReader(req.Body)
	}

	httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
	if err != nil {
		return httpResp{OK: false, Error: err.Error()}
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "Quill/1.0")
	}

	res, err := client.Do(httpReq)
	if err != nil {
		return httpResp{OK: false, Error: err.Error()}
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(res.Body, 10*1024*1024))

	headers := make(map[string]string)
	for k := range res.Header {
		headers[k] = res.Header.Get(k)
	}

	return httpResp{
		OK:      true,
		Status:  res.StatusCode,
		Body:    string(body),
		Headers: headers,
	}
}

func packResp(mod api.Module, resp httpResp) uint64 {
	respJSON, _ := json.Marshal(resp)
	return writeWasm(mod, respJSON)
}

func packErr(mod api.Module, msg string) uint64 {
	resp := httpResp{OK: false, Error: msg}
	respJSON, _ := json.Marshal(resp)
	return writeWasm(mod, respJSON)
}

func packOK(mod api.Module, data map[string]interface{}) uint64 {
	data["ok"] = true
	b, _ := json.Marshal(data)
	return writeWasm(mod, b)
}

func packFail(mod api.Module, errMsg string) uint64 {
	b, _ := json.Marshal(map[string]interface{}{"ok": false, "error": errMsg})
	return writeWasm(mod, b)
}

func writeWasm(mod api.Module, data []byte) uint64 {
	mallocFn := mod.ExportedFunction("malloc")
	if mallocFn == nil {
		return 0
	}
	results, err := mallocFn.Call(ctx, uint64(len(data)+1))
	if err != nil || len(results) == 0 {
		return 0
	}
	ptr := uint32(results[0])
	mod.Memory().Write(ptr, append(data, 0))
	return uint64(ptr)<<32 | uint64(uint32(len(data)))
}
