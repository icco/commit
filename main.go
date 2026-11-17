// Package main serves a simple HTTP endpoint that returns a random message.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"go.icco.me/gutil/logging"
	"go.icco.me/gutil/render"
	"go.icco.me/gutil/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
)

const service = "commit"

// defaultName is the addressee used when no ?name= is supplied, keeping the
// public endpoint's messages reading as they always have.
const defaultName = "Tristan"

type messageResponse struct {
	Message string `json:"message"`
}

// personalize renders a message as a text/template with {{.Name}} bound to
// name. A blank or absurdly long name falls back to defaultName. A message that
// fails to parse or execute is returned unchanged, so a malformed template in
// the pool degrades to its raw text rather than taking down the endpoint.
func personalize(msg, name string) string {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 64 {
		name = defaultName
	}
	tmpl, err := template.New("message").Parse(msg)
	if err != nil {
		return msg
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, struct{ Name string }{Name: name}); err != nil {
		return msg
	}
	return buf.String()
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
		Message: personalize(messages[idx.Int64()], r.URL.Query().Get("name")),
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
	return otelhttp.NewHandler(r, service,
		otelhttp.WithFilter(func(req *http.Request) bool {
			return req.URL.Path != "/metrics" && req.URL.Path != "/healthz"
		}),
	)
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, log, listenAddr()); err != nil {
		log.Errorw("server error", zap.Error(err))
	}
}

func run(ctx context.Context, log *zap.SugaredLogger, addr string) error {
	shutdown, err := tracing.Init(ctx, service)
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := shutdown(shutdownCtx); err != nil {
			log.Warnw("tracing shutdown", zap.Error(err))
		}
	}()

	log.Infow("listening", zap.String("addr", addr))
	srv := &http.Server{
		Addr:              addr,
		Handler:           newRouter(log.Desugar()),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case <-ctx.Done():
		log.Infow("shutdown signal received")
	case err := <-serveErr:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
