package module

import (
	"archive/zip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func TestMarkDoneKeepsKnownTotal(t *testing.T) {
	task := &dlTask{TotalSize: 1000, Downloaded: 600}
	task.markDone()
	if !task.Done {
		t.Fatal("task should be done")
	}
	if task.TotalSize != 1000 {
		t.Fatalf("TotalSize = %d, want 1000", task.TotalSize)
	}
}

func TestMarkDoneFillsUnknownTotal(t *testing.T) {
	task := &dlTask{TotalSize: 0, Downloaded: 42}
	task.markDone()
	if task.TotalSize != 42 {
		t.Fatalf("TotalSize = %d, want 42", task.TotalSize)
	}
}

func TestTLSVerificationIsOnByDefault(t *testing.T) {
	SetInsecureTLS(false)
	defer SetInsecureTLS(false)

	client := newHTTPClient(time.Second)
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport %T", client.Transport)
	}
	if tr.TLSClientConfig == nil || tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification should be enabled by default")
	}

	SetInsecureTLS(true)
	client = newHTTPClient(time.Second)
	tr = client.Transport.(*http.Transport)
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("InsecureSkipVerify should be true after SetInsecureTLS(true)")
	}
}

func TestHTTPRequestHonorsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte("late"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	raw := doHTTPRequest(ctx, http.MethodGet, srv.URL, nil, "", 5*time.Second)
	elapsed := time.Since(start)
	if elapsed > 300*time.Millisecond {
		t.Fatalf("request was not canceled, took %v", elapsed)
	}

	var resp httpResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("invalid response json: %v", err)
	}
	if resp.OK {
		t.Fatalf("expected failure response, got %#v", resp)
	}
}

func TestDefaultHTTPTimeoutIsSane(t *testing.T) {
	if defaultHTTPTimeout > 2*time.Minute {
		t.Fatalf("defaultHTTPTimeout = %v, want <= 2m", defaultHTTPTimeout)
	}
}

func createTestZip(t *testing.T, path, name, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractAsyncAndPoll(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "a.zip")
	dest := filepath.Join(dir, "out")
	createTestZip(t, zipPath, "hello.txt", "hi")

	L := lua.NewState()
	defer L.Close()
	utilTbl := L.NewTable()
	RegisterModuleUtil(L, utilTbl)
	L.SetGlobal("util", utilTbl)

	L.Push(utilTbl.RawGetString("extract_async"))
	L.Push(lua.LString(zipPath))
	L.Push(lua.LString(dest))
	if err := L.PCall(2, 1, nil); err != nil {
		t.Fatalf("extract_async: %v", err)
	}
	id := L.Get(-1).String()
	L.Pop(1)

	deadline := time.Now().Add(3 * time.Second)
	for {
		L.Push(utilTbl.RawGetString("extract_poll"))
		L.Push(lua.LString(id))
		if err := L.PCall(1, 1, nil); err != nil {
			t.Fatalf("extract_poll: %v", err)
		}
		st, ok := L.Get(-1).(*lua.LTable)
		L.Pop(1)
		if !ok {
			t.Fatal("extract_poll should return a table")
		}
		if st.RawGetString("done") == lua.LTrue {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("extract task did not finish in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

	b, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if string(b) != "hi" {
		t.Fatalf("content = %q", string(b))
	}
}
