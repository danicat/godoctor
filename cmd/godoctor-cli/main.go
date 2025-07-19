package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

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

	if flag.NArg() < 1 {
		fmt.Println("Usage: godoctor-cli <symbol>")
		os.Exit(1)
	}
	symbol := flag.Arg(0)

	// Create a context that can be cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Handle signals for graceful shutdown.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigc
		cancel()
	}()

	// Find the godoctor binary.
	godoctorPath, err := exec.LookPath("./godoctor")
	if err != nil {
		log.Fatalf("could not find godoctor binary: %v", err)
	}

	// Connect to the server
	transport := mcp.NewCommandTransport(exec.CommandContext(ctx, godoctorPath))
	client := mcp.NewClient(&mcp.Implementation{Name: "godoctor-cli", Version: version}, nil)
	session, err := client.Connect(ctx, transport)
	if err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer session.Close()

	// Call the getDoc tool
	params := &mcp.CallToolParams{
		Name: "getDoc",
		Arguments: map[string]any{
			"symbol": symbol,
		},
	}
	res, err := session.CallTool(ctx, params)
	if err != nil {
		log.Fatalf("CallTool failed: %v", err)
	}

	if res.IsError {
		if len(res.Content) > 0 {
			if textContent, ok := res.Content[0].(*mcp.TextContent); ok {
				log.Fatalf("Server returned an error: %s", textContent.Text)
			}
		}
		log.Fatal("Server returned an unspecified error.")
	}

	// Print the documentation
	for _, content := range res.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			fmt.Println(textContent.Text)
		}
	}
}
