package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"

	"github.com/google/generative-ai-go/genai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/api/option"
)

// Register registers the code_review tool with the server.
func Register(server *mcp.Server, apiKey string) {
	if apiKey != "" {
		reviewHandler, err := NewCodeReviewHandler(apiKey)
		if err != nil {
			log.Printf("Disabling code_review tool: failed to create handler: %v", err)
		} else {
			mcp.AddTool(server, &mcp.Tool{
				Name:        "code_review",
				Description: "Provides an expert-level, AI-powered review of a given Go source file.",
			}, reviewHandler.CodeReviewTool)
		}
	} else {
		log.Printf("API key not set, disabling code_review tool.")
	}
}

// GenerativeModel is an interface that abstracts the genai.GenerativeModel.
// This allows for mocking in tests.
type GenerativeModel interface {
	GenerateContent(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error)
}

// CodeReviewParams defines the input parameters for the code_review tool.
type CodeReviewParams struct {
	FileContent string `json:"file_content"`
	ModelName   string `json:"model_name,omitempty"`
	Hint        string `json:"hint,omitempty"`
}

// ReviewSuggestion defines the structured output for a single review suggestion.
type ReviewSuggestion struct {
	LineNumber  int    `json:"line_number"`
	Principle   string `json:"principle"`
	Comment     string `json:"comment"`
	Suggestion  string `json:"suggestion"`
}

// CodeReviewHandler holds the dependencies for the code review tool.
type CodeReviewHandler struct {
	defaultModel GenerativeModel
	newClient    func(ctx context.Context, opts ...option.ClientOption) (*genai.Client, error)
	apiKey       string
}

// NewCodeReviewHandler creates a new CodeReviewHandler.
func NewCodeReviewHandler(apiKey string) (*CodeReviewHandler, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable must be set")
	}
	handler := &CodeReviewHandler{
		apiKey:    apiKey,
		newClient: genai.NewClient,
	}
	// Initialize a default model to be used when no model is specified in the request.
	client, err := handler.newClient(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	handler.defaultModel = client.GenerativeModel("gemini-1.5-pro")
	return handler, nil
}

// CodeReviewTool performs an AI-powered code review and returns structured data.
func (h *CodeReviewHandler) CodeReviewTool(ctx context.Context, s *mcp.ServerSession, request *mcp.CallToolParamsFor[CodeReviewParams]) (*mcp.CallToolResult, error) {
	code := request.Arguments.FileContent
	if code == "" {
		return nil, fmt.Errorf("file_content cannot be empty")
	}

	model := h.defaultModel
	if request.Arguments.ModelName != "" {
		// If a model name is provided, create a new client and model for this request.
		client, err := h.newClient(ctx, option.WithAPIKey(h.apiKey))
		if err != nil {
			return nil, fmt.Errorf("failed to create genai client for model %s: %w", request.Arguments.ModelName, err)
		}
		model = client.GenerativeModel(request.Arguments.ModelName)
	}

	systemPrompt := `You are an expert Go code reviewer. Your sole purpose is to analyze Go code and provide feedback based on the principles outlined in the official Go community's "CodeReviewComments" wiki (https://github.com/golang/go/wiki/CodeReviewComments) and best practices from the go.dev blog.

Analyze the following code. Identify any areas that violate these principles. For each issue, provide a JSON object with the following fields: "line_number", "principle", "comment", and "suggestion".

Your response MUST be a valid JSON array of these objects. Do not include any other text, explanations, or markdown. If you find no issues, you MUST return an empty array: [].

Example of a valid response:
[
  {
    "line_number": 25,
    "principle": "Clarity",
    "comment": "The variable name 'h' is too short and doesn't convey its purpose.",
    "suggestion": "Consider renaming 'h' to 'handler' for better readability."
  }
]`

	if request.Arguments.Hint != "" {
		systemPrompt = fmt.Sprintf("A user has provided the following hint for your review: \"%s\". Interpret this hint within the context of Go best practices (such as simplicity, clarity, and robustness) and use it to guide your analysis.\n\n%s", request.Arguments.Hint, systemPrompt)
	}

	resp, err := model.GenerateContent(ctx, genai.Text(systemPrompt), genai.Text(code))
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("no response content from model. Check model parameters and API status")
	}

	textContent, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected response format from model, expected genai.Text")
	}

	// Clean the response by trimming markdown and whitespace
	cleanedJSON := regexp.MustCompile("(?s)```json\\s*(.*?)```").ReplaceAllString(string(textContent), "$1")

	var suggestions []ReviewSuggestion
	if err := json.Unmarshal([]byte(cleanedJSON), &suggestions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal suggestions from model response: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: cleanedJSON},
		},
	}, nil
}
