package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	version     = "dev"
	searchPaths = []string{"godoctor"}
)

type codeReviewArgs struct {
	FileContent string `json:"file_content"`
	Hint        string `json:"hint,omitempty"`
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Flags
	versionFlag := flag.Bool("version", false, "print the version and exit")
	reviewFile := flag.String("review", "", "path to a file to be reviewed by the code_review tool, or - for stdin")
	hint := flag.String("hint", "", "a hint for the code reviewer")
	serverPathFlag := flag.String("server", "", "path to the godoctor server binary")
	helpFlag := flag.Bool("help", false, "print this help message")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <package_path> [symbol_name]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Reviewer mode: %s -review <file_path> [-hint \"focus on X\"]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if *helpFlag {
		flag.Usage()
		os.Exit(0)
	}

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	// Create a context that can be cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Handle signals for graceful shutdown.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigc
		cancel()
	}()

	// Find the godoctor binary by checking common locations.
	godoctorPath, err := findGoDoctor(*serverPathFlag)
	if err != nil {
		return err
	}

	// Connect to the server
	transport := mcp.NewCommandTransport(exec.CommandContext(ctx, godoctorPath))
	client := mcp.NewClient(&mcp.Implementation{Name: "godoctor-cli", Version: version}, nil)
	session, err := client.Connect(ctx, transport)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			log.Printf("error closing session: %v", err)
		}
	}()

	// Determine which tool to call
	if *reviewFile != "" {
		return callCodeReview(ctx, session, *reviewFile, *hint)
	}
	return callGoDoc(ctx, session, flag.Args())
}

func findGoDoctor(godoctorPath string) (string, error) {
	if godoctorPath != "" {
		searchPaths = append([]string{godoctorPath}, searchPaths...)
	}
	for _, path := range searchPaths {
		foundPath, err := exec.LookPath(path)
		if err == nil {
			return foundPath, nil
		}
	}
	return "", fmt.Errorf("could not find godoctor binary, searched paths: %v", searchPaths)
}

func callTool(ctx context.Context, session *mcp.ClientSession, toolName string, args any) error {
	params := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}
	res, err := session.CallTool(ctx, params)
	if err != nil {
		return fmt.Errorf("CallTool '%s' failed: %w", toolName, err)
	}
	return printResult(res)
}

func callCodeReview(ctx context.Context, session *mcp.ClientSession, filePath, hint string) error {
	var reader io.Reader
	if filePath == "-" {
		reader = os.Stdin
	} else {
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open file %q: %w", filePath, err)
		}
		defer file.Close()
		reader = file
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	args := codeReviewArgs{
		FileContent: string(content),
		Hint:        hint,
	}
	return callTool(ctx, session, "code_review", args)
}

func callGoDoc(ctx context.Context, session *mcp.ClientSession, args []string) error {
	if len(args) < 1 || len(args) > 2 {
		flag.Usage()
		os.Exit(1)
	}
	pkgPath := args[0]
	symbolName := ""
	if len(args) == 2 {
		symbolName = args[1]
	}

	toolArgs := map[string]any{
		"package_path": pkgPath,
		"symbol_name":  symbolName,
	}
	return callTool(ctx, session, "go-doc", toolArgs)
}

func printResult(res *mcp.CallToolResult) error {
	if res.IsError {
		if len(res.Content) > 0 {
			if textContent, ok := res.Content[0].(*mcp.TextContent); ok {
				return fmt.Errorf("server returned an error: %s", textContent.Text)
			}
		}
		return fmt.Errorf("server returned an unspecified error")
	}

	for _, content := range res.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			// Try to unmarshal the text as JSON for pretty printing.
			var data any
			if err := json.Unmarshal([]byte(textContent.Text), &data); err == nil {
				// It's JSON, so pretty print it.
				prettyJSON, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					// Fallback to printing the raw text if re-marshaling fails.
					log.Printf("Warning: Failed to re-marshal JSON for printing: %v. Printing raw text.", err)
					fmt.Println(textContent.Text)
				} else {
					fmt.Println(string(prettyJSON))
				}
			} else {
				// It's not JSON, so print it as plain text.
				fmt.Println(textContent.Text)
			}
		} else {
			log.Printf("Received unhandled content type: %T\n", content)
		}
	}
	return nil
}
