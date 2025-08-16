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

func newTestHandler(t *testing.T, mockResponse string) *ReviewCodeHandler {
	t.Helper()
	generator := &mockGenerator{
		GenerateContentFunc: func(ctx context.Context, parts ...genai.Part) (*genai.GenerateContentResponse, error) {
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
	return &ReviewCodeHandler{defaultModel: generator}
}

func TestNewReviewCodeHandler_NoAPIKey(t *testing.T) {
	_, err := NewReviewCodeHandler(context.Background(), "")
	if err == nil {
		t.Fatal("expected an error when creating a handler with no API key, but got nil")
	}
	if !strings.Contains(err.Error(), "API key must not be empty") {
		t.Errorf("expected error message to contain 'API key must not be empty', but got: %s", err.Error())
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
	params := &mcp.CallToolParamsFor[ReviewCodeParams]{
		Arguments: ReviewCodeParams{FileContent: "package main"},
	}
	result, err := handler.ReviewCodeTool(context.Background(), nil, params)
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
	params := &mcp.CallToolParamsFor[ReviewCodeParams]{
		Arguments: ReviewCodeParams{
			FileContent: "package main",
			Hint:        "focus on hints",
		},
	}
	result, err := handler.ReviewCodeTool(context.Background(), nil, params)
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
	params := &mcp.CallToolParamsFor[ReviewCodeParams]{
		Arguments: ReviewCodeParams{FileContent: "package main"},
	}
	result, err := handler.ReviewCodeTool(context.Background(), nil, params)
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
