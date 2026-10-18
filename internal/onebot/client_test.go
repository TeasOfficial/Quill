package onebot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// bot.* 的 HTTP 调用必须能被插件执行的超时预算取消。
func TestClientContextCancelsRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		_, _ = w.Write([]byte(`{"status":"ok","retcode":0,"data":{}}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	c.SetContext(ctx)

	start := time.Now()
	if _, err := c.GetStatus(); err == nil {
		t.Fatal("expected an error when the context expires")
	}
	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Fatalf("request was not canceled, took %v", elapsed)
	}

	c.ClearContext()
}
