package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestMessageHandler(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	messageHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json; charset=UTF-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=UTF-8", ct)
	}

	var got messageResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// With no ?name=, every message is personalized to the default name.
	want := make([]string, len(messages))
	for i, m := range messages {
		want[i] = personalize(m, "")
	}
	if !slices.Contains(want, got.Message) {
		t.Errorf("message %q not in personalized messages list", got.Message)
	}
}

func TestPersonalize(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		arg  string
		want string
	}{
		{"substitutes name", "Only you, {{.Name}}, can merge.", "Roshan", "Only you, Roshan, can merge."},
		{"blank falls back to default", "Only you, {{.Name}}, can merge.", "", "Only you, Tristan, can merge."},
		{"whitespace falls back to default", "{{.Name}} merges.", "   ", "Tristan merges."},
		{"trims surrounding whitespace", "{{.Name}} merges.", "  Steph  ", "Steph merges."},
		{"overlong falls back to default", "{{.Name}} merges.", strings.Repeat("x", 65), "Tristan merges."},
		{"no placeholder is untouched", "Slap yourself.", "Roshan", "Slap yourself."},
		{"malformed template degrades to raw text", "Approve this {{.Name", "Roshan", "Approve this {{.Name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := personalize(tt.msg, tt.arg); got != tt.want {
				t.Errorf("personalize(%q, %q) = %q, want %q", tt.msg, tt.arg, got, tt.want)
			}
		})
	}
}

func TestMessageHandlerName(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/?name=Roshan", nil)
	rr := httptest.NewRecorder()

	messageHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var got messageResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := make([]string, len(messages))
	for i, m := range messages {
		want[i] = personalize(m, "Roshan")
	}
	if !slices.Contains(want, got.Message) {
		t.Errorf("message %q not in personalized messages list", got.Message)
	}
}

func TestNewRouterRoutesRoot(t *testing.T) {
	h := newRouter(zap.NewNop())
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestListenAddr(t *testing.T) {
	t.Setenv("ADDR", "")
	if got := listenAddr(); got != ":8080" {
		t.Errorf("default = %q, want :8080", got)
	}
	t.Setenv("ADDR", ":9999")
	if got := listenAddr(); got != ":9999" {
		t.Errorf("override = %q, want :9999", got)
	}
}

func TestRealIPFromTrustedProxy(t *testing.T) {
	var gotAddr string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		gotAddr = r.RemoteAddr
	})
	handler := realIPFromTrustedProxy(next)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if gotAddr != "203.0.113.195" {
		t.Errorf("got %q, want 203.0.113.195", gotAddr)
	}

	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.1"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	handler.ServeHTTP(w, req)
	if gotAddr != "203.0.113.1" {
		t.Errorf("got %q, want 203.0.113.1", gotAddr)
	}
}
