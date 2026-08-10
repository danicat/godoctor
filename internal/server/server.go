// Package server implements the Model Context Protocol (MCP) server for godoctor.
// It orchestrates the tool registration, handles incoming client requests (via Stdio or HTTP),
// and manages the lifecycle of the server. It connects the core logic (tools, graph)
// to the external world.
package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/danicat/godoctor/internal/instructions"
	"github.com/danicat/godoctor/internal/roots"
	adddependencies "github.com/danicat/godoctor/internal/tools/add_dependencies"
	listfiles "github.com/danicat/godoctor/internal/tools/list_files"
	mutationtest "github.com/danicat/godoctor/internal/tools/mutation_test"
	readdocs "github.com/danicat/godoctor/internal/tools/read_docs"
	smartbuild "github.com/danicat/godoctor/internal/tools/smart_build"
	smartedit "github.com/danicat/godoctor/internal/tools/smart_edit"
	smartread "github.com/danicat/godoctor/internal/tools/smart_read"
	testquery "github.com/danicat/godoctor/internal/tools/test_query"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server encapsulates the MCP server.
type Server struct {
	mcpServer *mcp.Server
}

// New creates a new Server instance.
func New(version string) *Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "godoctor",
		Version: version,
	}, &mcp.ServerOptions{
		Instructions: instructions.Get(),
		InitializedHandler: func(ctx context.Context, req *mcp.InitializedRequest) {
			roots.Global.Sync(ctx, req.Session)
		},
		RootsListChangedHandler: func(ctx context.Context, req *mcp.RootsListChangedRequest) {
			roots.Global.Sync(ctx, req.Session)
		},
	})

	return &Server{
		mcpServer: s,
	}
}

// Run starts the MCP server using Stdio.
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

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS headers for browser-based MCP clients
		origin := r.Header.Get("Origin")
		if origin != "" {
			// In production (Cloud Run), the origin should match the expected domain.
			// For local development, allow localhost/127.0.0.1 origins or all origins
			if strings.HasPrefix(origin, "http://localhost") ||
				strings.HasPrefix(origin, "http://127.0.0.1") ||
				strings.HasPrefix(origin, "https://") {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Mcp-Session-Id")
			w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		mcpHandler.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	//nolint:gosec
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	log.Printf("godoctor MCP server listening on HTTP %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("HTTP server error: %w", err)
	}

	return nil
}

// RegisterHandlers wires all tools.
func (s *Server) RegisterHandlers() error {
	smartread.Register(s.mcpServer)
	smartedit.Register(s.mcpServer)
	smartedit.RegisterMultiEdit(s.mcpServer)
	smartbuild.Register(s.mcpServer)
	readdocs.Register(s.mcpServer)
	testquery.Register(s.mcpServer)
	adddependencies.Register(s.mcpServer)
	listfiles.Register(s.mcpServer)
	mutationtest.Register(s.mcpServer)
	return nil
}
