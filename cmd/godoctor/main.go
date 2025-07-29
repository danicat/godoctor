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
	"github.com/danicat/godoctor/internal/tool/scalpel"
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
	instructionsFlag := fs.Bool("instructions", false, "print instructions and exit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *instructionsFlag {
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
	godoc.Register(server)
	scalpel.Register(server)
	codereview.Register(server, apiKeyEnv)
}

func printInstructions() {
	fmt.Print(`
## General Workflow

When working with a Go codebase, a typical workflow involves understanding the code, making changes, and then reviewing those changes. The godoctor tools are designed to assist with each of these stages.

## Tool: go-doc

### When to Use

Use the go-doc tool whenever you need to understand a piece of Go code. This could be before you modify it, when you are trying to debug it, or when you are exploring a new codebase. It is your primary tool for code comprehension.

**Key Scenarios:**

- **Before Modifying Code:** Before changing a function or type, use go-doc to understand its purpose, parameters, and return values.
- **Debugging:** When you encounter a bug, use go-doc to inspect the functions involved and understand their expected behavior.
- **Code Exploration:** When you are new to a project, use go-doc to explore the public API of different packages.

### How to Use

The go-doc tool takes a package_path and an optional symbol_name. See the tool's description for detailed parameter information.

## Tool: code-review

### When to Use

Use the code-review tool after you have made changes to the code and before you commit them. This tool acts as an expert Go developer, providing feedback on your changes to ensure they meet the standards of the Go community.

**Key Scenarios:**

- **After Making Changes:** Once you have implemented a new feature or fixed a bug, use the code-review tool to get feedback on your work.
- **Improving Code Quality:** If you are refactoring code, use the code-review tool to ensure your changes are an improvement.
- **Learning Go:** The code-review tool is a great way to learn idiomatic Go. By reviewing your code, you can see where you are deviating from best practices.

### How to Use

The code-review tool takes the content of a Go file as input. See the tool's description for detailed parameter information.
`)
}
