---
name: go-bootstrap
description: "Guidelines and conventions for bootstrapping Go modules, setting up project structures, and establishing reliable production patterns. Activate on module initialization or setup requests."
---

# Go Project Bootstrapping & Layout Conventions

This skill provides step-by-step instructions for establishing a modern, robust, and idiomatic Go module layout. It strictly follows official Go guidelines and emphasizes simplicity, compiler-safety, and production-ready architectures.

## 1. Trigger Conditions
Activate this skill whenever the user asks to:
- Initialize a Go project or module.
- Create a new Go service or application.
- Scaffold or design a project directory layout.
- Set up a standard library HTTP server or database connection pool.

## 2. Layout Conventions
We adhere strictly to the official [go.dev/doc/modules/layout](https://go.dev/doc/modules/layout) conventions.

### Flat Layout (Recommended for Simple Services & Libraries)
Keep all Go files in the root directory. It is simpler, avoids redundant packaging, and prevents import cycles.
```
my-service/
├── go.mod
├── go.sum
├── main.go
├── server.go
├── database.go
└── server_test.go
```
*Rule of thumb:* Start flat. Do not over-engineer with nested folders like `pkg/` or `internal/` until scale or the need for multiple binaries warrants it.

### Nested Layout (Recommended for Multiple Binaries or Private Packages)
Use a nested structure only when creating multiple executable binaries or protecting private package APIs from external imports.
```
my-app/
├── go.mod
├── go.sum
├── cmd/
│   ├── api-server/
│   │   └── main.go
│   └── cli-tool/
│       └── main.go
└── internal/
    ├── auth/
    │   └── auth.go
    └── db/
        └── db.go
```
- **`cmd/`**: Contains subdirectories for each executable binary. Keep code in `cmd/` minimal (only `main.go` and flag parsing/initialization).
- **`internal/`**: Contains private application packages. Go compiler strictly blocks other modules from importing anything under `internal/`.
- **`pkg/`**: **NEVER** use `pkg/` in modern idiomatic Go. It is a legacy/non-standard convention that adds redundant path depth.

## 3. Production Patterns (Go 1.24+)

### HTTP Server with Graceful Shutdown
Always implement graceful shutdown on HTTP servers to prevent dropped requests during deployments.

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func run(ctx context.Context) error {
	// Setup termination signal catching
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for shutdown or error
	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Println("Shutting down server gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
```

### Database Connection Pooling (`database/sql`)
Ensure connection limits and idle timeouts are configured on your `*sql.DB` instance to prevent leaks and handle transient failures.

```go
package main

import (
	"context"
	"database/sql"
	"time"
)

func initDB(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn) // or other driver
	if err != nil {
		return nil, err
	}

	// Establish connection pool limits
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(3 * time.Minute)

	// Verify database is reachable with a timeout
	pingCtx, cancel := context.WithTimeout(ctx, 3 * time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
```

## 4. Bootstrapping Workflow
1. Use `project_init` to create the target directory and run `go mod init <module_path>`.
2. Determine if project needs a flat layout or a nested layout. Default to **flat**.
3. Create core files (`main.go`, `server.go`, etc.) using `smart_edit`.
4. Add any required dependencies using `add_dependency`.
5. Run `smart_build` to verify the entire module compiles and tests successfully.
