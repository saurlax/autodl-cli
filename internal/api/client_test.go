package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFailureHandling(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
		status           int
	}{
		{"platform error", `{"code":"Denied","msg":"secret-token","request_id":"req-1"}`, "req-1", 200},
		{"invalid envelope", `{}`, "API error", 200},
		{"invalid JSON", `<html>bad gateway</html>`, "invalid JSON", 200},
		{"HTTP failure", `secret-token`, "HTTP 401", 401},
		{"oversized", strings.Repeat("x", (8<<20)+1), "exceeds", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status); io.WriteString(w, tc.body) }))
			defer server.Close()
			_, err := New(server.URL, "secret-token", time.Second).Call(context.Background(), "POST", "/", map[string]any{})
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCancellationAndRedirect(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("redirect followed with credentials") }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	client := New(server.URL, "token", time.Second)
	if _, err := client.Call(context.Background(), "POST", "/", nil); err == nil || !strings.Contains(err.Error(), "307") {
		t.Fatalf("redirect result: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Call(ctx, "POST", "/", nil); err != context.Canceled {
		t.Fatalf("cancellation result: %v", err)
	}
}
