package codereview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockGenerator is a mock implementation of the GenerativeModel interface.
type mockGenerator struct {
	GenerateContentFunc func(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error)
}

func (m *mockGenerator) GenerateContent(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, parts...)
	}
	return nil, fmt.Errorf("mockGenerator.GenerateContent: GenerateContentFunc not implemented")
}

func newTestHandler(t *testing.T, mockResponse string) *CodeReviewHandler {
	t.Helper()
	generator := &mockGenerator{
		GenerateContentFunc: func(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
			// The actual response from the mock is what we pass in.
			// It might be a valid JSON string, or just garbage text.
			// It might also be an error.
			if strings.Contains(mockResponse, "error") {
				return nil, fmt.Errorf("%s", mockResponse)
			}
			return &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []genai.Part{genai.Text(mockResponse)},
						},
					},
				},
			}, nil
		},
	}
	return &CodeReviewHandler{defaultModel: generator}
}

func TestNewCodeReviewHandler_NoAPIKey(t *testing.T) {
	_, err := NewCodeReviewHandler("")
	if err == nil {
		t.Fatal("expected an error when creating a handler with no API key, but got nil")
	}
	if !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Errorf("expected error message to contain 'GEMINI_API_KEY', but got: %s", err.Error())
	}
}

func TestCodeReviewTool_Success(t *testing.T) {
	// 1. Setup
	expectedSuggestions := []ReviewSuggestion{
		{LineNumber: 1, Principle: "Testing", Comment: "This is a test", Suggestion: "Good job"},
	}
	mockResponse, err := json.Marshal(expectedSuggestions)
	if err != nil {
		t.Fatalf("failed to marshal mock response: %v", err)
	}
	handler := newTestHandler(t, string(mockResponse))

	// 2. Act
	params := &mcp.CallToolParamsFor[CodeReviewParams]{
		Arguments: CodeReviewParams{FileContent: "package main"},
	}
	result, err := handler.CodeReviewTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("CodeReviewTool failed: %v", err)
	}

	// 3. Assert
	if result.IsError {
		t.Fatalf("Expected a successful result, but got an error: %v", result.Content)
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, but got %T", result.Content[0])
	}

	var suggestions []ReviewSuggestion
	if err := json.Unmarshal([]byte(textContent.Text), &suggestions); err != nil {
		t.Fatalf("Failed to unmarshal suggestions from text content: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Comment != "This is a test" {
		t.Errorf("Unexpected suggestions received: %+v", suggestions)
	}
}

func TestCodeReviewTool_Hint(t *testing.T) {
	// 1. Setup
	expectedSuggestions := []ReviewSuggestion{
		{LineNumber: 1, Principle: "Hint", Comment: "This is a hint test", Suggestion: "Good job"},
	}
	mockResponse, err := json.Marshal(expectedSuggestions)
	if err != nil {
		t.Fatalf("failed to marshal mock response: %v", err)
	}
	handler := newTestHandler(t, string(mockResponse))

	// 2. Act
	params := &mcp.CallToolParamsFor[CodeReviewParams]{
		Arguments: CodeReviewParams{
			FileContent: "package main",
			Hint:        "focus on hints",
		},
	}
	result, err := handler.CodeReviewTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("CodeReviewTool failed: %v", err)
	}

	// 3. Assert
	if result.IsError {
		t.Fatalf("Expected a successful result, but got an error: %v", result.Content)
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, but got %T", result.Content[0])
	}

	var suggestions []ReviewSuggestion
	if err := json.Unmarshal([]byte(textContent.Text), &suggestions); err != nil {
		t.Fatalf("Failed to unmarshal suggestions from text content: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Comment != "This is a hint test" {
		t.Errorf("Unexpected suggestions received: %+v", suggestions)
	}
}

func TestCodeReviewTool_InvalidJSON(t *testing.T) {
	// 1. Setup
	handler := newTestHandler(t, "this is not json")

	// 2. Act
	params := &mcp.CallToolParamsFor[CodeReviewParams]{
		Arguments: CodeReviewParams{FileContent: "package main"},
	}
	_, err := handler.CodeReviewTool(context.Background(), nil, params)

	// 3. Assert
	if err == nil {
		t.Fatal("Expected an error result, but got a successful one")
	}
	if !strings.Contains(err.Error(), "failed to unmarshal suggestions") {
		t.Errorf("Expected a JSON unmarshal error, but got: %s", err.Error())
	}
}
