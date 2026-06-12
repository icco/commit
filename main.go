// Package main serves a simple HTTP endpoint that returns a random message.
package main

import (
	"math/rand/v2"
	"net/http"
	"os"
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
	render.JSON(log, w, http.StatusOK, messageResponse{
		Message: messages[rand.IntN(len(messages))], //nolint:gosec // non-cryptographic random selection
	})
}

func main() {
	log := logging.Must(logging.NewLogger(service))
	defer logging.Sync(log)

	r := chi.NewRouter()
	r.Use(logging.Middleware(log.Desugar()))

	r.Get("/", messageHandler)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Infow("listening", zap.String("addr", addr))
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalw("server stopped", zap.Error(err))
	}
}
