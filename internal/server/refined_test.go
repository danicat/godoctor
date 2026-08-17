package server_test

import (
	"context"
	"strings"
	"testing"

	readdocs "github.com/danicat/godoctor/internal/tools/read_docs"
	"github.com/danicat/godoctor/internal/tools/selene"
	smartbuild "github.com/danicat/godoctor/internal/tools/smart_build"
	smarttest "github.com/danicat/godoctor/internal/tools/smart_test"
	testquery "github.com/danicat/godoctor/internal/tools/test_query"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRefinedTools_ReadDocsFormats(t *testing.T) {
	ctx := context.Background()
	const fmtPkg = "fmt"

	// Happy Path: Markdown (Default)
	res, _, err := readdocs.Handler(ctx, nil, readdocs.Params{
		ImportPath: fmtPkg,
		SymbolName: "Println",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "func Println") {
		t.Error("Expected markdown to contain function signature")
	}

	// Happy Path: JSON
	resJSON, _, err := readdocs.Handler(ctx, nil, readdocs.Params{
		ImportPath: fmtPkg,
		SymbolName: "Println",
		Format:     "json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(resJSON.Content[0].(*mcp.TextContent).Text), "{") {
		t.Error("Expected JSON object")
	}

	// Sad Path: Invalid Format
	resErr, _, _ := readdocs.Handler(ctx, nil, readdocs.Params{
		ImportPath: fmtPkg,
		Format:     "yaml",
	})
	if !resErr.IsError {
		t.Error("Expected error for invalid format")
	}
}

func TestRefinedTools_RelativePathRejection(t *testing.T) {
	ctx := context.Background()

	// smart_build
	buildRes, _, err := smartbuild.Handler(ctx, nil, smartbuild.Params{Dir: "./local"})
	if err != nil || !buildRes.IsError ||
		!strings.Contains(buildRes.Content[0].(*mcp.TextContent).Text, "dir is required and must be an absolute path") {
		t.Errorf("expected smart_build to reject relative path, got res=%v, err=%v", buildRes, err)
	}

	// smart_test
	testRes, _, err := smarttest.Handler(ctx, nil, smarttest.Params{Dir: "relative/path"})
	if err != nil || !testRes.IsError ||
		!strings.Contains(testRes.Content[0].(*mcp.TextContent).Text, "dir is required and must be an absolute path") {
		t.Errorf("expected smart_test to reject relative path, got res=%v, err=%v", testRes, err)
	}

	// test_query
	tqRes, _, err := testquery.Handler(ctx, nil, testquery.Params{Dir: ".", Query: "SELECT 1"})
	if err != nil || !tqRes.IsError ||
		!strings.Contains(tqRes.Content[0].(*mcp.TextContent).Text, "dir is required and must be an absolute path") {
		t.Errorf("expected test_query to reject relative path, got res=%v, err=%v", testqResErrorMessage(tqRes), err)
	}

	// selene
	mutRes, _, err := selene.Handler(ctx, nil, selene.Params{Dir: "relative"})
	if err != nil || !mutRes.IsError ||
		!strings.Contains(mutRes.Content[0].(*mcp.TextContent).Text, "dir is required and must be an absolute path") {
		t.Errorf("expected selene to reject relative path, got res=%v, err=%v", mutRes, err)
	}
}

func testqResErrorMessage(res *mcp.CallToolResult) string {
	if res != nil && len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
