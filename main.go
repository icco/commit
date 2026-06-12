// Package main serves a simple HTTP endpoint that returns a random message.
package main

import (
	"crypto/rand"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/icco/gutil/logging"
	"github.com/icco/gutil/render"
	"go.uber.org/zap"
)

const service = "commit"

type messageResponse struct {
	Message string `json:"message"`
}

func messageHandler(w http.ResponseWriter, r *http.Request) {
	log := logging.FromContext(r.Context())
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(messages))))
	if err != nil {
		log.Errorw("pick message", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	render.JSON(log, w, http.StatusOK, messageResponse{
		Message: messages[idx.Int64()],
	})
}

// realIPFromTrustedProxy rewrites r.RemoteAddr from X-Forwarded-For only when
// the direct peer is in private IP space — i.e. our reverse proxy on the
// docker bridge. Avoids the spoofing footgun in chi's deprecated RealIP.
func realIPFromTrustedProxy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		peer := net.ParseIP(host)
		if peer != nil && (peer.IsPrivate() || peer.IsLoopback()) {
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				if first := strings.TrimSpace(strings.SplitN(xff, ",", 2)[0]); first != "" {
					r.RemoteAddr = first
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func newRouter(log *zap.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(realIPFromTrustedProxy)
	r.Use(logging.Middleware(log))
	r.Get("/", messageHandler)
	return r
}

func listenAddr() string {
	if addr := os.Getenv("ADDR"); addr != "" {
		return addr
	}
	return ":8080"
}

func main() {
	log := logging.Must(logging.NewLogger(service))
	defer logging.Sync(log)

	addr := listenAddr()
	log.Infow("listening", zap.String("addr", addr))
	srv := &http.Server{
		Addr:              addr,
		Handler:           newRouter(log.Desugar()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalw("server stopped", zap.Error(err))
	}
}
