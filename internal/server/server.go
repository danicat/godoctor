package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/danicat/godoctor/internal/config"
	"github.com/danicat/godoctor/internal/prompts"
	"github.com/danicat/godoctor/internal/tools/get_documentation"
	"github.com/danicat/godoctor/internal/tools/review_code"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Server struct {
	mcpServer *mcp.Server
	cfg       *config.Config
}

func New(cfg *config.Config, version string) *Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "godoctor",
		Version: version,
	}, nil)

	return &Server{
		mcpServer: s,
		cfg:       cfg,
	}
}

func (s *Server) RegisterHandlers() {
	// Register tools
	get_documentation.Register(s.mcpServer)
	review_code.Register(s.mcpServer, s.cfg.DefaultModel)

	// Register prompts
	s.mcpServer.AddPrompt(prompts.ImportThis("doc"), prompts.ImportThisHandler)
}

func (s *Server) Run(ctx context.Context) error {
	if s.cfg.ListenAddr != "" {
		return s.runHTTP(ctx)
	}
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

func (s *Server) runHTTP(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:    s.cfg.ListenAddr,
		Handler: mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return s.mcpServer }, nil),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("godoctor listening on %s", s.cfg.ListenAddr)
	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
