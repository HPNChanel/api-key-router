package adapter

import (
	"reflect"
	"testing"
)

func TestGeminiAdapter_mapToGeminiRequest(t *testing.T) {
	adapter := NewGeminiAdapter("test-api-key")

	tests := []struct {
		name     string
		input    OpenAIRequest
		validate func(*testing.T, GeminiRequest)
	}{
		{
			name: "simple user message",
			input: OpenAIRequest{
				Model: "gpt-4",
				Messages: []OpenAIMessage{
					{Role: "user", Content: "Hello, world!"},
				},
			},
			validate: func(t *testing.T, req GeminiRequest) {
				if len(req.Contents) != 1 {
					t.Errorf("len(Contents) = %d, want 1", len(req.Contents))
				}
				if req.Contents[0].Role != "user" {
					t.Errorf("Contents[0].Role = %s, want user", req.Contents[0].Role)
				}
				if req.Contents[0].Parts[0].Text != "Hello, world!" {
					t.Errorf("Contents[0].Parts[0].Text = %s, want 'Hello, world!'", req.Contents[0].Parts[0].Text)
				}
			},
		},
		{
			name: "assistant role maps to model",
			input: OpenAIRequest{
				Model: "gpt-4",
				Messages: []OpenAIMessage{
					{Role: "user", Content: "Hi"},
					{Role: "assistant", Content: "Hello!"},
					{Role: "user", Content: "How are you?"},
				},
			},
			validate: func(t *testing.T, req GeminiRequest) {
				if len(req.Contents) != 3 {
					t.Errorf("len(Contents) = %d, want 3", len(req.Contents))
				}
				if req.Contents[1].Role != "model" {
					t.Errorf("Contents[1].Role = %s, want model (assistant mapped to model)", req.Contents[1].Role)
				}
			},
		},
		{
			name: "system message becomes systemInstruction",
			input: OpenAIRequest{
				Model: "gpt-4",
				Messages: []OpenAIMessage{
					{Role: "system", Content: "You are a helpful assistant."},
					{Role: "user", Content: "Hi"},
				},
			},
			validate: func(t *testing.T, req GeminiRequest) {
				if len(req.Contents) != 1 {
					t.Errorf("len(Contents) = %d, want 1 (system not in contents)", len(req.Contents))
				}
				if req.SystemInstruction == nil {
					t.Error("SystemInstruction is nil, expected system message")
				} else if req.SystemInstruction.Parts[0].Text != "You are a helpful assistant." {
					t.Errorf("SystemInstruction.Parts[0].Text = %s, want 'You are a helpful assistant.'", req.SystemInstruction.Parts[0].Text)
				}
			},
		},
		{
			name: "generation config mapping",
			input: OpenAIRequest{
				Model:       "gpt-4",
				Messages:    []OpenAIMessage{{Role: "user", Content: "test"}},
				Temperature: ptrFloat(0.8),
				MaxTokens:   ptrInt(100),
				TopP:        ptrFloat(0.9),
				Stop:        []string{"END"},
			},
			validate: func(t *testing.T, req GeminiRequest) {
				if req.GenerationConfig.Temperature == nil || *req.GenerationConfig.Temperature != 0.8 {
					t.Error("Temperature not mapped correctly")
				}
				if req.GenerationConfig.MaxOutputTokens == nil || *req.GenerationConfig.MaxOutputTokens != 100 {
					t.Error("MaxOutputTokens not mapped correctly")
				}
				if req.GenerationConfig.TopP == nil || *req.GenerationConfig.TopP != 0.9 {
					t.Error("TopP not mapped correctly")
				}
				if !reflect.DeepEqual(req.GenerationConfig.StopSequences, []string{"END"}) {
					t.Error("StopSequences not mapped correctly")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.mapToGeminiRequest(tt.input)
			tt.validate(t, result)
		})
	}
}

func TestGeminiAdapter_mapToOpenAIResponse(t *testing.T) {
	adapter := NewGeminiAdapter("test-api-key")

	geminiResp := GeminiResponse{
		Candidates: []GeminiCandidate{
			{
				Content: GeminiContent{
					Parts: []GeminiPart{{Text: "Hello from Gemini!"}},
				},
				FinishReason: "STOP",
				Index:        0,
			},
		},
		UsageMetadata: &GeminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}

	result := adapter.mapToOpenAIResponse(geminiResp, "gpt-4")

	if result.Object != "chat.completion" {
		t.Errorf("Object = %s, want chat.completion", result.Object)
	}
	if result.Model != "gpt-4" {
		t.Errorf("Model = %s, want gpt-4", result.Model)
	}
	if len(result.Choices) != 1 {
		t.Errorf("len(Choices) = %d, want 1", len(result.Choices))
	}
	if result.Choices[0].Message.Role != "assistant" {
		t.Errorf("Choices[0].Message.Role = %s, want assistant", result.Choices[0].Message.Role)
	}
	if result.Choices[0].Message.Content != "Hello from Gemini!" {
		t.Errorf("Choices[0].Message.Content = %s, want 'Hello from Gemini!'", result.Choices[0].Message.Content)
	}
	if result.Choices[0].FinishReason != "stop" {
		t.Errorf("Choices[0].FinishReason = %s, want stop", result.Choices[0].FinishReason)
	}
	if result.Usage.PromptTokens != 10 {
		t.Errorf("Usage.PromptTokens = %d, want 10", result.Usage.PromptTokens)
	}
	if result.Usage.CompletionTokens != 5 {
		t.Errorf("Usage.CompletionTokens = %d, want 5", result.Usage.CompletionTokens)
	}
	if result.Usage.TotalTokens != 15 {
		t.Errorf("Usage.TotalTokens = %d, want 15", result.Usage.TotalTokens)
	}
}

func TestGeminiAdapter_mapModelName(t *testing.T) {
	adapter := NewGeminiAdapter("test-api-key")

	tests := []struct {
		input    string
		expected string
	}{
		{"gpt-4", "gemini-1.5-pro"},
		{"gpt-4-turbo", "gemini-1.5-pro"},
		{"gpt-4o", "gemini-1.5-flash"},
		{"gpt-4o-mini", "gemini-1.5-flash-8b"},
		{"gpt-3.5-turbo", "gemini-1.5-flash"},
		{"gemini-1.5-pro", "gemini-1.5-pro"},
		{"gemini-1.5-flash", "gemini-1.5-flash"},
		{"unknown-model", "unknown-model"}, // Pass-through
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := adapter.mapModelName(tt.input)
			if result != tt.expected {
				t.Errorf("mapModelName(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGeminiAdapter_mapFinishReason(t *testing.T) {
	adapter := NewGeminiAdapter("test-api-key")

	tests := []struct {
		input    string
		expected string
	}{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
		{"RECITATION", "content_filter"},
		{"OTHER", "stop"},
		{"UNKNOWN", "stop"}, // Default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := adapter.mapFinishReason(tt.input)
			if result != tt.expected {
				t.Errorf("mapFinishReason(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGeminiAdapter_Name(t *testing.T) {
	adapter := NewGeminiAdapter("test-api-key")

	if adapter.Name() != "gemini" {
		t.Errorf("Name() = %s, want gemini", adapter.Name())
	}
}

func TestNewGeminiAdapter_Options(t *testing.T) {
	customURL := "https://custom.api.google.com"
	adapter := NewGeminiAdapter(
		"test-api-key",
		WithBaseURL(customURL),
	)

	if adapter.baseURL != customURL {
		t.Errorf("baseURL = %s, want %s", adapter.baseURL, customURL)
	}
}

// Helper functions
func ptrFloat(f float64) *float64 {
	return &f
}

func ptrInt(i int) *int {
	return &i
}
