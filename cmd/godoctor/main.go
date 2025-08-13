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
	agentsFlag := fs.Bool("agents", false, "print instructions for agents and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *agentsFlag {
		printInstructions()
		return nil
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
	scribble.Register(server)
	scalpel.Register(server)

	// Register the code_review tool only if an API key is available.
	codereview.Register(server, os.Getenv(apiKeyEnv))
}

func printInstructions() {
	fmt.Print(`
## General Workflow

When working with a Go codebase, a typical workflow involves understanding the code, making changes, and then reviewing those changes. The godoctor tools are designed to assist with each of these stages.

## Tool: godoc

### When to Use

Use the godoc tool whenever you need to understand a piece of Go code. This could be before you modify it, when you are trying to debug it, or when you are exploring a new codebase. It is your primary tool for code comprehension.

**Key Scenarios:**

- **Before Modifying Code:** Before changing a function or type, use godoc to understand its purpose, parameters, and return values.
- **Debugging:** When you encounter a bug, use godoc to inspect the functions involved and understand their expected behavior.
- **Code Exploration:** When you are new to a project, use godoc to explore the public API of different packages.

### How to Use

The godoc tool takes a package_path and an optional symbol_name. See the tool's description for detailed parameter information.

## Tool: scribble

### When to Use

Use the scribble tool to create new Go source files. This tool ensures that the file is created with the correct content and also checks for any initial errors.

**Key Scenarios:**

- **Creating a new Go file:** When you need to create a new Go file with some initial content.

### How to Use

The scribble tool takes the path of the Go file to create and the content of the file as input. See the tool's description for detailed parameter information.

## Tool: scalpel

### When to Use

Use the scalpel tool to edit existing Go source files. This tool is useful for making small changes to a file, such as renaming a variable or changing a function signature.

**Key Scenarios:**

- **Refactoring:** When you are refactoring code, use the scalpel tool to make small, targeted changes.
- **Fixing Bugs:** When you are fixing a bug, use the scalpel tool to apply a patch to a file.

### How to Use

The scalpel tool takes the path of the Go file to edit, the old string to replace, and the new string to replace it with. See the tool's description for detailed parameter information.

## Tool: code_review

### When to Use

Use the code_review tool after you have made changes to the code and before you commit them. This tool acts as an expert Go developer, providing feedback on your changes to ensure they meet the standards of the Go community.

**Key Scenarios:**

- **After Making Changes:** Once you have implemented a new feature or fixed a bug, use the code_review tool to get feedback on your work.
- **Improving Code Quality:** If you are refactoring code, use the code_review tool to ensure your changes are an improvement.
- **Learning Go:** The code_review tool is a great way to learn idiomatic Go. By reviewing your code, you can see where you are deviating from best practices.

### How to Use

The code_review tool takes the content of a Go file as input. See the tool's description for detailed parameter information.
`)
}
