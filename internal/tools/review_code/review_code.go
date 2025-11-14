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
	"os"
	"regexp"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"
)

// Register registers the review_code tool with the server.
func Register(server *mcp.Server, defaultModel string) {
	reviewHandler, err := NewReviewCodeHandler(context.Background(), defaultModel)
	if err != nil {
		log.Printf("Disabling review_code tool: failed to create handler: %v", err)
		return
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "code_review",
		Title:       "Go Code Review",
		Description: "Analyzes Go code for style, correctness, and idioms. Returns structured suggestions with line numbers and recommendations to improve quality.",
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

// ContentGenerator abstracts the generative model for testing.
type ContentGenerator interface {
	GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

// RealGenerator wraps the actual GenAI client.
type RealGenerator struct {
	client *genai.Client
}

func (r *RealGenerator) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	return r.client.Models.GenerateContent(ctx, model, contents, config)
}

// ReviewCodeHandler holds the dependencies for the review code tool.
type ReviewCodeHandler struct {
	generator    ContentGenerator
	defaultModel string
}

// Option is a function that configures a ReviewCodeHandler.
type Option func(*ReviewCodeHandler)

// WithGenerator sets the ContentGenerator for the ReviewCodeHandler.
func WithGenerator(generator ContentGenerator) Option {
	return func(h *ReviewCodeHandler) {
		h.generator = generator
	}
}

// NewReviewCodeHandler creates a new ReviewCodeHandler.
func NewReviewCodeHandler(ctx context.Context, defaultModel string, opts ...Option) (*ReviewCodeHandler, error) {
	handler := &ReviewCodeHandler{
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
				return nil, fmt.Errorf("Vertex AI enabled but missing configuration: set GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION")
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
				return nil, fmt.Errorf("authentication failed: set GOOGLE_API_KEY (or GEMINI_API_KEY) for Gemini API, or set GOOGLE_GENAI_USE_VERTEXAI=true with GOOGLE_CLOUD_PROJECT and GOOGLE_CLOUD_LOCATION for Vertex AI")
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

var jsonMarkdownRegex = regexp.MustCompile("(?s)```json\\s*(.*?)```")

// ReviewCodeTool performs an AI-powered code review and returns structured data.
func (h *ReviewCodeHandler) ReviewCodeTool(ctx context.Context, request *mcp.CallToolRequest, args ReviewCodeParams) (*mcp.CallToolResult, []ReviewSuggestion, error) {
	code := args.FileContent
	if code == "" {
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

	// Construct the request using the new SDK
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: systemPrompt},
				{Text: code},
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

	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil || len(resp.Candidates[0].Content.Parts) == 0 {
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
	textContent := part.Text

	// Clean the response by trimming markdown and whitespace
	cleanedJSON := jsonMarkdownRegex.ReplaceAllString(textContent, "$1")

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
	}, suggestions, nil
}
