package module

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
