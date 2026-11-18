package materials

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchRetriesTransient(t *testing.T) {
	tries := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tries++
		if tries < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	cfg := FetchConfig{Retries: 3, Backoff: time.Millisecond}
	dst := filepath.Join(t.TempDir(), "a.bin")
	if err := cfg.download(context.Background(), srv.URL, dst); err != nil {
		t.Fatalf("应重试后成功: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "payload" {
		t.Fatalf("内容不符: %q err=%v", got, err)
	}
	if tries != 3 {
		t.Fatalf("期望 3 次尝试，实际 %d", tries)
	}
}

func TestFetchDoesNotRetry4xx(t *testing.T) {
	tries := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tries++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := FetchConfig{Retries: 5, Backoff: time.Millisecond}
	dst := filepath.Join(t.TempDir(), "a.bin")
	if err := cfg.download(context.Background(), srv.URL, dst); err == nil {
		t.Fatal("404 应直接失败")
	}
	if tries != 1 {
		t.Fatalf("404 不应重试，实际 %d 次", tries)
	}
}

func TestSourceErrorsFormat(t *testing.T) {
	var e sourceErrors
	e.add("https://a", os.ErrNotExist)
	e.add("https://b", os.ErrPermission)
	got := e.String()
	for _, want := range []string{"https://a", "https://b", "not exist", "permission"} {
		if !contains(got, want) {
			t.Fatalf("错误信息缺少 %q: %s", want, got)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
