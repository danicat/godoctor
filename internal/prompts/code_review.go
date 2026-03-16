package prompts

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const codeReviewPrompt = `You are conducting a senior-level Go code review. Apply this checklist systematically.

## Interface Design
- Are interfaces defined by the **consumer** (where used), not the producer?
- Do interfaces have 1-3 methods? (Interface Segregation Principle)
- Do functions return concrete types, not interfaces?

## Concurrency
- Does every goroutine have a clear lifecycle? (context cancellation, WaitGroup, or ErrGroup)
- Are structs with sync.Mutex passed by pointer, never copied?
- Are channels closed by the sender? Is select used to prevent deadlocks?
- Are synchronous functions preferred over async ones?

## Error Handling
- Are errors wrapped with fmt.Errorf("doing x: %%w", err)?
- Are error strings lowercase, no punctuation?
- Is errors.Is / errors.As used for typed error checking?
- Is every error checked? No silent _ drops?

## API Design
- Do constructors use Functional Options or Config structs?
- Are receiver names consistent (1-2 letter abbreviation, not "this" or "self")?
- Are receiver types consistent across methods (all pointer or all value)?

## Naming & Style
- MixedCaps everywhere? (no snake_case, no ALL_CAPS)
- Short names for small scopes (i, ctx), descriptive for exported symbols?
- Initialisms in consistent case? (URL not Url, ID not Id)

## Non-Obvious Pitfalls
- crypto/rand for keys, never math/rand
- var t []string (nil slice) preferred over t := []string{} (empty slice)
- Indent error flow: handle error first, keep happy path at minimal indent

## After Review
- Run smart_build to verify all fixes compile and tests pass.
- Run modernize_code to catch outdated patterns.
- For an unbiased second opinion from a different model, use code_review.`

// CodeReview creates the definition for the 'go_code_review' prompt.
func CodeReview(namespace string) *mcp.Prompt {
	name := "go_code_review"
	if namespace != "" {
		name = namespace + ":" + name
	}
	return &mcp.Prompt{
		Name:        name,
		Title:       "Go Code Review",
		Description: "Senior-level Go code review checklist covering concurrency, interfaces, error handling, and GoDoctor tool integration.",
		Arguments: []*mcp.PromptArgument{
			{Name: "focus", Description: "Optional area to focus the review on (e.g. concurrency, error-handling)", Required: false},
		},
	}
}

// CodeReviewHandler generates the content for the 'go_code_review' prompt.
func CodeReviewHandler(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	prompt := codeReviewPrompt
	if focus := req.Params.Arguments["focus"]; focus != "" {
		prompt = fmt.Sprintf("**Focus this review specifically on: %s**\n\n%s", focus, prompt)
	}

	return &mcp.GetPromptResult{
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: prompt,
				},
			},
		},
	}, nil
}
