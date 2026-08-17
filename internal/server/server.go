// Package server implements the Model Context Protocol (MCP) server for godoctor.
// It orchestrates tool registration, handles incoming client requests (via Stdio or HTTP),
// and manages the lifecycle of the server. It connects the core logic (tools, graph)
// to the external world.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/danicat/godoctor/internal/config"
	"github.com/danicat/godoctor/internal/instructions"
	readdocs "github.com/danicat/godoctor/internal/tools/read_docs"
	"github.com/danicat/godoctor/internal/tools/selene"
	smartbuild "github.com/danicat/godoctor/internal/tools/smart_build"
	smartedit "github.com/danicat/godoctor/internal/tools/smart_edit"
	smarttest "github.com/danicat/godoctor/internal/tools/smart_test"
	testquery "github.com/danicat/godoctor/internal/tools/test_query"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Option defines a functional option for configuring a Server.
type Option func(*Server)

// WithServerConfig configures allowed origins and timeouts from config.ServerConfig.
func WithServerConfig(cfg config.ServerConfig) Option {
	return func(s *Server) {
		if len(cfg.AllowedOrigins) > 0 {
			s.allowedOrigins = cfg.AllowedOrigins
		}
		if cfg.ReadTimeout > 0 {
			s.readTimeout = cfg.ReadTimeout
		}
		if cfg.WriteTimeout > 0 {
			s.writeTimeout = cfg.WriteTimeout
		}
		if cfg.IdleTimeout > 0 {
			s.idleTimeout = cfg.IdleTimeout
		}
		if cfg.ShutdownTimeout > 0 {
			s.shutdownTimeout = cfg.ShutdownTimeout
		}
	}
}

// Server encapsulates the MCP server and its lifecycle configuration.
type Server struct {
	mcpServer       *mcp.Server
	registerOnce    sync.Once
	allowedOrigins  []string
	instructions    string
	readTimeout     time.Duration
	writeTimeout    time.Duration
	idleTimeout     time.Duration
	shutdownTimeout time.Duration
}

// New creates a new Server instance.
func New(version string, opts ...Option) *Server {
	srv := &Server{
		instructions:    instructions.Get(),
		readTimeout:     30 * time.Second,
		writeTimeout:    5 * time.Minute,
		idleTimeout:     120 * time.Second,
		shutdownTimeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(srv)
	}

	srv.mcpServer = mcp.NewServer(&mcp.Implementation{
		Name:    "godoctor",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: srv.instructions,
	})

	return srv
}

// Run starts the MCP server using Stdio transport.
func (s *Server) Run(ctx context.Context) error {
	if err := s.RegisterHandlers(); err != nil {
		return fmt.Errorf("failed to register handlers: %w", err)
	}
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// ServeHTTP starts the server over HTTP using StreamableHTTP.
func (s *Server) ServeHTTP(ctx context.Context, addr string) error {
	if err := s.RegisterHandlers(); err != nil {
		return fmt.Errorf("failed to register handlers: %w", err)
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
		return s.mcpServer
	}, nil)

	readTimeout := s.readTimeout
	if readTimeout <= 0 {
		readTimeout = 30 * time.Second
	}
	writeTimeout := s.writeTimeout
	if writeTimeout <= 0 {
		writeTimeout = 5 * time.Minute
	}
	idleTimeout := s.idleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 120 * time.Second
	}
	shutdownTimeout := s.shutdownTimeout
	if shutdownTimeout <= 0 {
		shutdownTimeout = 10 * time.Second
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.createHTTPHandler(mcpHandler),
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	serverDone := make(chan struct{})
	var shutdownErr error
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				log.Printf("HTTP server shutdown error: %v", err)
				shutdownErr = err
			}
		case <-serverDone:
			return
		}
	}()

	log.Printf("godoctor MCP server listening on HTTP %s", addr)
	err := srv.ListenAndServe()
	close(serverDone)
	wg.Wait()

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	if shutdownErr != nil {
		return fmt.Errorf("HTTP server shutdown error: %w", shutdownErr)
	}

	return nil
}

func (s *Server) createHTTPHandler(mcpHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered in HTTP handler: %v", rec)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		origin := r.Header.Get("Origin")
		isAllowed := origin != "" && s.isAllowedOrigin(origin)

		if isAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == http.MethodOptions {
			if origin != "" && !isAllowed {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		mcpHandler.ServeHTTP(w, r)
	})
}

// isAllowedOrigin strictly validates CORS origins against localhost / loopback or configured allowed list.
func (s *Server) isAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range s.allowedOrigins {
		if allowed == origin || allowed == "*" {
			return true
		}
	}

	u, err := url.Parse(origin)
	if err != nil {
		return false
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}

	hostname := strings.ToLower(u.Hostname())
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || hostname == "[::1]" {
		return true
	}

	return false
}

// RegisterHandlers wires all tools idempotently.
func (s *Server) RegisterHandlers() error {
	s.registerOnce.Do(func() {
		smartedit.Register(s.mcpServer)
		smartbuild.Register(s.mcpServer)
		smarttest.Register(s.mcpServer)
		readdocs.Register(s.mcpServer)
		testquery.Register(s.mcpServer)
		selene.Register(s.mcpServer)
	})
	return nil
}
