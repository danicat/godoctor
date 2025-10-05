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

package review_code

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"github.com/danicat/godoctor/internal/mcp/result"
	"github.com/google/generative-ai-go/genai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/api/option"
)

// generativeModel is an interface that abstracts the generative model.
type generativeModel interface {
	GenerateContent(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error)
}

// Register registers the review_code tool with the server.
func Register(server *mcp.Server, apiKey string) {
	if apiKey == "" {
		log.Printf("API key not set, disabling review_code tool.")
		return
	}
	reviewHandler, err := NewReviewCodeHandler(context.Background(), apiKey)
	if err != nil {
		log.Printf("Disabling review_code tool: failed to create handler: %v", err)
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "review_code",
		Title:       "Go Code Review",
		Description: "Performs an expert code review of Go source code. The tool returns a JSON array of suggestions, each containing a 'line_number', a 'finding' describing the issue, and a 'comment' with a recommendation. Use this tool to verify the quality of your changes before finalizing your work.",
	}, reviewHandler.ReviewCodeTool)
}

// ReviewCodeParams defines the input parameters for the review_code tool.
type ReviewCodeParams struct {
	FileContent string `json:"file_content"`
	ModelName   string `json:"model_name,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// ReviewSuggestion defines the structured output for a single review suggestion.
type ReviewSuggestion struct {
	LineNumber int    `json:"line_number"`
	Finding    string `json:"finding"`
	Comment    string `json:"comment"`
}

// ReviewCodeHandler holds the dependencies for the review code tool.
type ReviewCodeHandler struct {
	client       *genai.Client
	defaultModel generativeModel
}

// Option is a function that configures a ReviewCodeHandler.
type Option func(*ReviewCodeHandler)

// WithClient sets the genai.Client for the ReviewCodeHandler.
func WithClient(client *genai.Client) Option {
	return func(h *ReviewCodeHandler) {
		h.client = client
	}
}

// NewReviewCodeHandler creates a new ReviewCodeHandler.
func NewReviewCodeHandler(ctx context.Context, apiKey string, opts ...Option) (*ReviewCodeHandler, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key must not be empty")
	}
	handler := &ReviewCodeHandler{}
	for _, opt := range opts {
		opt(handler)
	}
	if handler.client == nil {
		client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create genai client: %w", err)
		}
		handler.client = client
	}
	return handler, nil
}

var jsonMarkdownRegex = regexp.MustCompile("(?s)```json\\s*(.*?)```")

// ReviewCodeTool performs an AI-powered code review and returns structured data.
func (h *ReviewCodeHandler) ReviewCodeTool(ctx context.Context, request *mcp.CallToolRequest, args ReviewCodeParams) (*mcp.CallToolResult, any, error) {
	code := args.FileContent
	if code == "" {
		return result.NewError("file_content cannot be empty"), nil, nil
	}

	modelName := "gemini-1.5-pro-latest"
	if args.ModelName != "" {
		modelName = args.ModelName
	}

	var model generativeModel
	if h.defaultModel != nil {
		model = h.defaultModel
	} else {
		model = h.client.GenerativeModel(modelName)
	}

	systemPrompt := `You are an expert Go code reviewer. Your sole purpose is to analyze Go code and provide feedback based on the principles of idiomatic Go, as outlined in the following guidelines.

**Core Principles:**
*   **Simplicity:** Is the code simple and straightforward? Does it avoid unnecessary complexity?
*   **Readability:** Is the code easy to read and understand?
*   **Clarity:** Does the code clearly express its intent?
*   **Concurrency:** Is concurrency used safely and correctly?
*   **Interfaces:** Are interfaces small and focused?

**Formatting and Style:**
*   **gofmt:** Assume the code has been formatted with gofmt.
*   **Naming:** Are package, variable, function, and interface names idiomatic?
*   **Comments:** Are comments clear, concise, and helpful? Do they explain *why*, not *what*?

**Language Idioms:**
*   **Error Handling:** Is error handling correct? Are errors wrapped to provide context?
*   **Interfaces:** Are interfaces used effectively?
*   **Concurrency:** Are goroutines and channels used appropriately? Is shared data protected?
*   **Data Structures:** Are slices, maps, and structs used correctly?
*   **Control Structures:** Are 'if', 'for', 'switch', and 'defer' used in a standard way?

**Testability:**
*   Is the code easy to test? Are there any dependencies that make testing difficult?

**Your Task:**
Analyze the following code. Identify any areas that violate these principles. For each issue, provide a JSON object with the following fields: "line_number", "finding", and "comment".

Your response MUST be a valid JSON array of these objects. Do not include any other text, explanations, or markdown. If you find no issues, you MUST return an empty array: [].

Example of a valid response:
[
  {
    "line_number": 25,
    "finding": "The variable name 'serverUrl' doesn't comply with Go naming standards.",
    "comment": "Initialisms should have all upper or all lower case: use serverURL instead."
  }
]`

	if args.Hint != "" {
		systemPrompt = fmt.Sprintf("A user has provided the following hint for your review: \"%s\".\nInterpret this hint within the context of Go best practices (such as simplicity, clarity, and robustness) and use it to guide your analysis.\n\n%s", args.Hint, systemPrompt)
	}

	resp, err := model.GenerateContent(ctx, genai.Text(systemPrompt), genai.Text(code))
	if err != nil {
		return result.NewError("failed to generate content: %v", err), nil, nil
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
		return result.NewError("no response content from model. Check model parameters and API status"), nil, nil
	}

	textContent, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return result.NewError("unexpected response format from model, expected genai.Text"), nil, nil
	}

	// Clean the response by trimming markdown and whitespace
	cleanedJSON := jsonMarkdownRegex.ReplaceAllString(string(textContent), "$1")

	var suggestions []ReviewSuggestion
	if err := json.Unmarshal([]byte(cleanedJSON), &suggestions); err != nil {
		return result.NewError("failed to unmarshal suggestions from model response: %v", err), nil, nil
	}

	return result.NewText(cleanedJSON), nil, nil
}
