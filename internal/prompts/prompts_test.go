package prompts_test

import (
	"context"
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/prompts"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCodeReview(t *testing.T) {
	p := prompts.CodeReview("test")
	if p.Name != "test:go_code_review" {
		t.Errorf("CodeReview() name = %q, want test:go_code_review", p.Name)
	}

	ctx := context.Background()
	req := &mcp.GetPromptRequest{
		Params: &mcp.GetPromptParams{
			Arguments: map[string]string{
				"focus": "concurrency",
			},
		},
	}
	res, err := prompts.CodeReviewHandler(ctx, req)
	if err != nil {
		t.Fatalf("CodeReviewHandler() unexpected error = %v", err)
	}
	if len(res.Messages) == 0 {
		t.Fatal("CodeReviewHandler() returned no messages")
	}
	textContent, ok := res.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatal("CodeReviewHandler() content is not TextContent")
	}
	if !strings.Contains(textContent.Text, "concurrency") {
		t.Errorf("CodeReviewHandler() text missing focus argument, got:\n%s", textContent.Text)
	}
}

func TestImportThis(t *testing.T) {
	p := prompts.ImportThis("")
	if p.Name != "import_this" {
		t.Errorf("ImportThis() name = %q, want import_this", p.Name)
	}

	ctx := context.Background()
	req := &mcp.GetPromptRequest{}
	res, err := prompts.ImportThisHandler(ctx, req)
	if err != nil {
		t.Fatalf("ImportThisHandler() unexpected error = %v", err)
	}
	if len(res.Messages) == 0 {
		t.Fatal("ImportThisHandler() returned no messages")
	}
	textContent, ok := res.Messages[0].Content.(*mcp.TextContent)
	if !ok {
		t.Fatal("ImportThisHandler() content is not TextContent")
	}
	if !strings.Contains(textContent.Text, "effective_go") {
		t.Errorf("ImportThisHandler() text missing effective_go reference, got:\n%s", textContent.Text)
	}
}
