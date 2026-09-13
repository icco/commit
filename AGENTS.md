# AGENTS.md

Guidance for coding agents working on commit.

## Project Overview

Microservice written in Go (`github.com/nat/commit`) that returns randomized commit messages over HTTP (`GET /` returns JSON `{"message": "..."}`). Supports optional `?name=<name>`.

## Commands

```sh
go test ./...    # Run tests
go vet ./...     # Vet code
go run main.go   # Run service locally
go build .       # Build binary
```

## Architecture & Conventions

- `main.go` — HTTP server setup, message selection, and handlers.
- Standard Go conventions and `gutil` logging where applicable.
- PR titles and commits must follow Conventional Commits with lowercase subjects.
