// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	"github.com/danicat/godoctor/internal/prompts"
	"github.com/danicat/godoctor/internal/tools/codereview"
	"github.com/danicat/godoctor/internal/tools/godoc"
	"github.com/danicat/godoctor/internal/tools/scalpel"
	"github.com/danicat/godoctor/internal/tools/scribble"
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
	addPrompts(server)

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
	const namespace = "doc"
	// Register the go-doc tool unconditionally.
	godoc.Register(server, namespace)
	scribble.Register(server, namespace)
	scalpel.Register(server, namespace)

	// Register the code_review tool only if an API key is available.
	codereview.Register(server, os.Getenv(apiKeyEnv), namespace)
}

func addPrompts(server *mcp.Server) {
	const namespace = "doc"
	server.AddPrompt(prompts.ImportThis(namespace), prompts.ImportThisHandler)
	server.AddPrompt(prompts.Describe(namespace), prompts.DescribeHandler)
}
