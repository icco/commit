package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
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
	if !slices.Contains(messages, got.Message) {
		t.Errorf("message %q not in messages list", got.Message)
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
