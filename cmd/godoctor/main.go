package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/danicat/godoctor/internal/tool/codereview"
	"github.com/danicat/godoctor/internal/tool/godoc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version = "dev"
)

func main() {
	versionFlag := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "godoctor", Version: version}, nil)
	addTools(server)
	if err := server.Run(context.Background(), mcp.NewStdioTransport()); err != nil {
		log.Fatalf("Error running server: %v", err)
	}
}

func addTools(server *mcp.Server) {
	// Register the go-doc tool unconditionally.
	godoc.Register(server)

	// Register the code_review tool only if an API key is available.
	apiKey := os.Getenv("GEMINI_API_KEY")
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
		log.Println("GEMINI_API_KEY not set, disabling code_review tool.")
	}
}