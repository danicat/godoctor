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
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/genai"
)

// mockGenerator is a mock implementation of the ContentGenerator interface.
type mockGenerator struct {
	GenerateContentFunc func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
}

func (m *mockGenerator) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	if m.GenerateContentFunc != nil {
		return m.GenerateContentFunc(ctx, model, contents, config)
	}
	return nil, fmt.Errorf("mockGenerator.GenerateContent: GenerateContentFunc not implemented")
}

func newTestHandler(t *testing.T, mockResponse string) *ReviewCodeHandler {
	t.Helper()
	generator := &mockGenerator{
		GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
			if strings.Contains(mockResponse, "error") {
				return nil, fmt.Errorf("%s", mockResponse)
			}
			return &genai.GenerateContentResponse{
				Candidates: []*genai.Candidate{
					{
						Content: &genai.Content{
							Parts: []*genai.Part{
								{Text: mockResponse},
							},
						},
					},
				},
			}, nil
		},
	}
	
	// We use WithGenerator to bypass the real client creation
	handler, err := NewReviewCodeHandler(context.Background(), "gemini-2.5-pro", WithGenerator(generator))
	if err != nil {
		t.Fatalf("failed to create test handler: %v", err)
	}
	return handler
}

func TestNewReviewCodeHandler_NoAuth(t *testing.T) {
	// Ensure no auth env vars are set for this test
	os.Unsetenv("GOOGLE_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	os.Unsetenv("GOOGLE_GENAI_USE_VERTEXAI")
	os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	os.Unsetenv("GOOGLE_CLOUD_LOCATION")

	_, err := NewReviewCodeHandler(context.Background(), "gemini-2.5-pro")
	if err == nil {
		t.Fatal("expected an error when creating a handler with no auth, but got nil")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected error message to contain 'authentication failed', but got: %s", err.Error())
	}
}

func TestNewReviewCodeHandler_VertexAI_MissingConfig(t *testing.T) {
	// Set Vertex AI flag but unset config
	os.Setenv("GOOGLE_GENAI_USE_VERTEXAI", "true")
	os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	os.Unsetenv("GOOGLE_CLOUD_LOCATION")
	defer os.Unsetenv("GOOGLE_GENAI_USE_VERTEXAI")

	_, err := NewReviewCodeHandler(context.Background(), "gemini-2.5-pro")
	if err == nil {
		t.Fatal("expected an error when creating a handler with Vertex AI enabled but missing config, but got nil")
	}
	if !strings.Contains(err.Error(), "Vertex AI enabled but missing configuration") {
		t.Errorf("expected error message to contain 'Vertex AI enabled but missing configuration', but got: %s", err.Error())
	}
}

func TestReviewCodeTool_Success(t *testing.T) {
	// 1. Setup
	expectedSuggestions := []ReviewSuggestion{
		{LineNumber: 1, Finding: "Testing", Comment: "This is a test"},
	}
	mockResponse, err := json.Marshal(expectedSuggestions)
	if err != nil {
		t.Fatalf("failed to marshal mock response: %v", err)
	}
	handler := newTestHandler(t, string(mockResponse))

	// 2. Act
	params := ReviewCodeParams{FileContent: "package main"}
	result, _, err := handler.ReviewCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("ReviewCodeTool failed: %v", err)
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

func TestReviewCodeTool_Hint(t *testing.T) {
	// 1. Setup
	expectedSuggestions := []ReviewSuggestion{
		{LineNumber: 1, Finding: "Hint", Comment: "This is a hint test"},
	}
	mockResponse, err := json.Marshal(expectedSuggestions)
	if err != nil {
		t.Fatalf("failed to marshal mock response: %v", err)
	}
	handler := newTestHandler(t, string(mockResponse))

	// 2. Act
	params := ReviewCodeParams{
		FileContent: "package main",
		Hint:        "focus on hints",
	}
	result, _, err := handler.ReviewCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("ReviewCodeTool failed: %v", err)
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

func TestReviewCodeTool_InvalidJSON(t *testing.T) {
	// 1. Setup
	handler := newTestHandler(t, "this is not json")

	// 2. Act
	params := ReviewCodeParams{FileContent: "package main"}
	result, _, err := handler.ReviewCodeTool(context.Background(), nil, params)
	if err != nil {
		t.Fatalf("ReviewCodeTool returned an unexpected error: %v", err)
	}

	// 3. Assert
	if !result.IsError {
		t.Fatal("Expected an error result, but got a successful one")
	}
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("Expected TextContent, but got %T", result.Content[0])
	}
	if !strings.Contains(textContent.Text, "failed to unmarshal suggestions") {
		t.Errorf("Expected a JSON unmarshal error, but got: %s", textContent.Text)
	}
}