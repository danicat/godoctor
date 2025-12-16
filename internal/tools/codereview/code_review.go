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

// Package codereview implements the AI-powered code review tool.
package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"
)

var (
	// ErrVertexAIMissingConfig indicates that Vertex AI is enabled but project/location configuration is missing.
	ErrVertexAIMissingConfig = fmt.Errorf("vertex AI enabled but missing configuration")

	// ErrAuthFailed indicates that no valid authentication credentials were found.
	ErrAuthFailed = fmt.Errorf("authentication failed")
)

// Register registers the code_review tool with the server.
func Register(server *mcp.Server, defaultModel string) {
	reviewHandler, err := NewHandler(context.Background(), defaultModel)
	if err != nil {
		log.Printf("Disabling code_review tool: failed to create handler: %v", err)
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:  "review_code",
		Title: "Review Go Code",
		Description: "Analyzes Go source code for idiomatic style, correctness, and best practices. " +
			"Returns structured suggestions to improve code quality.",
	}, reviewHandler.Tool)
}

// Params defines the input parameters for the code_review tool.
type Params struct {
	FileContent string `json:"file_content"`
	ModelName   string `json:"model_name,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// ReviewSuggestion defines the structured output for a single review suggestion.
type ReviewSuggestion struct {
	LineNumber int    `json:"line_number"`
	Severity   string `json:"severity"` // "error", "warning", "suggestion"
	Finding    string `json:"finding"`
	Comment    string `json:"comment"`
}

// ReviewResult defines the structured output for the code_review tool.
type ReviewResult struct {
	Suggestions []ReviewSuggestion `json:"suggestions"`
}

// ContentGenerator abstracts the generative model for testing.
type ContentGenerator interface {
	GenerateContent(ctx context.Context, model string, contents []*genai.Content,
		config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

// RealGenerator wraps the actual GenAI client.
type RealGenerator struct {
	client *genai.Client
}

// GenerateContent generates content using the underlying GenAI client.
func (r *RealGenerator) GenerateContent(ctx context.Context, model string, contents []*genai.Content,
	config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	return r.client.Models.GenerateContent(ctx, model, contents, config)
}

// Handler holds the dependencies for the review code tool.
type Handler struct {
	generator    ContentGenerator
	defaultModel string
}

// Option is a function that configures a Handler.
type Option func(*Handler)

// WithGenerator sets the ContentGenerator for the Handler.
func WithGenerator(generator ContentGenerator) Option {
	return func(h *Handler) {
		h.generator = generator
	}
}

// NewHandler creates a new Handler.
func NewHandler(ctx context.Context, defaultModel string, opts ...Option) (*Handler, error) {
	handler := &Handler{
		defaultModel: defaultModel,
	}
	for _, opt := range opts {
		opt(handler)
	}

	if handler.generator == nil {
		var config *genai.ClientConfig

		// Check if Vertex AI is explicitly requested
		useVertex := os.Getenv("GOOGLE_GENAI_USE_VERTEXAI")
		if useVertex == "true" || useVertex == "1" {
			project := os.Getenv("GOOGLE_CLOUD_PROJECT")
			location := os.Getenv("GOOGLE_CLOUD_LOCATION")

			if project == "" || location == "" {
				return nil, fmt.Errorf("%w: set GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION", ErrVertexAIMissingConfig)
			}

			config = &genai.ClientConfig{
				Project:  project,
				Location: location,
				Backend:  genai.BackendVertexAI,
			}
		} else {
			// Default to Gemini API
			apiKey := os.Getenv("GOOGLE_API_KEY")
			if apiKey == "" {
				apiKey = os.Getenv("GEMINI_API_KEY")
			}

			if apiKey == "" {
				return nil, fmt.Errorf("%w: set GOOGLE_API_KEY (or GEMINI_API_KEY) " +
					"for Gemini API, or set GOOGLE_GENAI_USE_VERTEXAI=true with GOOGLE_CLOUD_PROJECT " +
					"and GOOGLE_CLOUD_LOCATION for Vertex AI", ErrAuthFailed)
			}

			config = &genai.ClientConfig{
				APIKey:  apiKey,
				Backend: genai.BackendGeminiAPI,
			}
		}

		client, err := genai.NewClient(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create genai client: %w", err)
		}
		handler.generator = &RealGenerator{client: client}
	}
	return handler, nil
}

var jsonMarkdownRegex = regexp.MustCompile("(?s)```json" + "\\s*(.*?)" + "```")

// Tool performs an AI-powered code review and returns structured data.
func (h *Handler) Tool(ctx context.Context, _ *mcp.CallToolRequest, args Params) (
	*mcp.CallToolResult, *ReviewResult, error) {
	if args.FileContent == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "file_content cannot be empty"},
			},
		}, nil, nil
	}

	modelName := h.defaultModel
	if args.ModelName != "" {
		modelName = args.ModelName
	}

	systemPrompt := constructSystemPrompt(args.Hint)

	// Construct the request using the new SDK
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: systemPrompt},
				{Text: args.FileContent},
			},
		},
	}

	resp, err := h.generator.GenerateContent(ctx, modelName, contents, nil)
	if err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to generate content: %v", err)},
			},
		}, nil, nil
	}

	if !isValidResponse(resp) {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "no response content from model. Check model parameters and API status"},
			},
		}, nil, nil
	}

	// Extract text from the first part of the first candidate
	part := resp.Candidates[0].Content.Parts[0]
	if part.Text == "" {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "unexpected response format from model, expected text content"},
			},
		}, nil, nil
	}

	return parseReviewResponse(part.Text)
}

func parseReviewResponse(text string) (*mcp.CallToolResult, *ReviewResult, error) {
	// Clean the response by trimming markdown and whitespace
	cleanedJSON := jsonMarkdownRegex.ReplaceAllString(text, "$1")

	var suggestions []ReviewSuggestion
	if err := json.Unmarshal([]byte(cleanedJSON), &suggestions); err != nil {
		return &mcp.CallToolResult{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("failed to unmarshal suggestions from model response: %v", err)},
			},
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: cleanedJSON},
		},
	}, &ReviewResult{Suggestions: suggestions}, nil
}

func isValidResponse(resp *genai.GenerateContentResponse) bool {
	return resp != nil && len(resp.Candidates) > 0 &&
		resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0
}

func constructSystemPrompt(hint string) string {
	prompt := `You are an expert Go code reviewer. Your sole purpose is to analyze Go code and provide feedback
based on the principles of idiomatic Go, as outlined in the following guidelines.

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
Analyze the following code. Identify any areas that violate these principles. For each issue, provide a JSON object
with the following fields: "line_number", "severity", "finding", and "comment".

*   **severity**: Must be one of "error", "warning", or "suggestion".
    *   "error": Critical bugs, race conditions, or panic risks.
    *   "warning": Logic errors, potential bugs, or significant non-idiomatic usage.
    *   "suggestion": Style improvements, naming consistency, or minor optimizations.

Your response MUST be a valid JSON array of these objects. Do not include any other text, explanations, or markdown.
If you find no issues, you MUST return an empty array: [].

Example of a valid response:
[
  {
    "line_number": 25,
    "severity": "suggestion",
    "finding": "The variable name 'serverUrl' doesn't comply with Go naming standards.",
    "comment": "Initialisms should have all upper or all lower case: use serverURL instead."
  },
  {
    "line_number": 42,
    "severity": "error",
    "finding": "Unhandled error from 'os.Open'",
    "comment": "Always check errors before using the returned values."
  }
]`

	if hint != "" {
		prompt = fmt.Sprintf("A user has provided the following hint for your review: \"%s\".\n"+
			"Interpret this hint within the context of Go best practices (such as simplicity, clarity, and robustness) " +
			"and use it to guide your analysis.\n\n%s", hint, prompt)
	}
	return prompt
}