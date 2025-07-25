package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/danicat/godoctor/internal/tool/codereview"
	"github.com/danicat/godoctor/internal/tool/godoc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version = "dev"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("godoctor", flag.ContinueOnError)
	apiKeyEnv := fs.String("api-key-env", "GEMINI_API_KEY", "environment variable for the Gemini API key")
	versionFlag := fs.Bool("version", false, "print the version and exit")
	listenAddr := fs.String("listen", "", "listen address for HTTP transport (e.g., :8080)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *versionFlag {
		fmt.Println(version)
		return nil
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "godoctor", Version: version}, nil)
	addTools(server, *apiKeyEnv)

	if *listenAddr != "" {
		httpServer := &http.Server{
			Addr:    *listenAddr,
			Handler: mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil),
		}
		go func() {
			<-ctx.Done()
			_ = httpServer.Shutdown(context.Background()) // best effort shutdown
		}()
		log.Printf("godoctor listening on %s", *listenAddr)
		return httpServer.ListenAndServe()
	}

	return server.Run(ctx, mcp.NewStdioTransport())
}

func addTools(server *mcp.Server, apiKeyEnv string) {
	// Register the go-doc tool unconditionally.
	godoc.Register(server)

	// Register the code_review tool only if an API key is available.
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey != "" {
		reviewHandler, err := codereview.NewCodeReviewHandler(apiKey)
		if err != nil {
			log.Printf("Disabling code_review tool: failed to create handler: %v", err)
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "code_review",
				Description: "Acts as an expert Go developer, reviewing code for quality, clarity, and idiomatic style based on community best practices.",
			}, reviewHandler.CodeReviewTool)
		}
	} else {
		log.Printf("%s not set, disabling code_review tool.", apiKeyEnv)
	}
}