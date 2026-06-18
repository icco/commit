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
		{"substitutes name", "Only you, {name}, can merge.", "Roshan", "Only you, Roshan, can merge."},
		{"blank falls back to default", "Only you, {name}, can merge.", "", "Only you, Tristan, can merge."},
		{"whitespace falls back to default", "{name} merges.", "   ", "Tristan merges."},
		{"trims surrounding whitespace", "{name} merges.", "  Steph  ", "Steph merges."},
		{"overlong falls back to default", "{name} merges.", strings.Repeat("x", 65), "Tristan merges."},
		{"no placeholder is untouched", "Slap yourself.", "Roshan", "Slap yourself."},
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
