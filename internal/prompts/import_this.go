package prompts

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const importThisPrompt = `Your mission is to read the following documents:
https://go.dev/doc/effective_go
https://go.dev/wiki/CodeReviewComments
https://google.github.io/styleguide/go/
https://go.dev/doc/modules/layout
https://www.ardanlabs.com/blog/2017/02/package-oriented-design.html
https://go-proverbs.github.io/
https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/

And produce the a comprehensive AGENTS.md file with detailed instructions for LLMs to code Go in an idiomatic, maintainable, testable and easy to read way.

You also should include instructions on how to use the MCP tools you have available (e.g. godoctor, gopls) to achieve the goals above.`

func ImportThis() *mcp.Prompt {
	return &mcp.Prompt{
		Name:        "import_this",
		Description: "This is not the Zen of Python, but it will help you write good code.",
	}
}

func ImportThisHandler(ctx context.Context, session *mcp.ServerSession, params *mcp.GetPromptParams) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: importThisPrompt,
				},
			},
		},
	}, nil
}
