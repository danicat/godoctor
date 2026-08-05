package read

import (
	"context"
	"testing"
	"time"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSmartReadLatency(t *testing.T) {
	repoDir := "/Users/petruzalek/projects/godoctor"
	roots.Global.Add(nil, repoDir)

	targetFile := repoDir + "/internal/server/server.go"

	start := time.Now()
	res, _, err := readCodeHandler(context.Background(), nil, Params{
		Filenames: []string{targetFile},
	})
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("readCodeHandler failed: %v", err)
	}

	if res.IsError {
		t.Fatalf("tool returned error: %v", res.Content)
	}

	if len(res.Content) == 0 {
		t.Fatal("no content returned")
	}

	text := res.Content[0].(*mcp.TextContent).Text
	t.Logf("smart_read completed in %v, output length: %d chars", duration, len(text))
}
