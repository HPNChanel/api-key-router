// Package adapter provides implementations for external AI provider integrations.
package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultGeminiBaseURL is the default Gemini API endpoint.
	DefaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

	// DefaultTimeout is the default HTTP client timeout.
	DefaultTimeout = 30 * time.Second
)

// GeminiAdapter implements AIProvider for Google Gemini API.
// It translates OpenAI-compatible requests to Gemini format and vice versa.
type GeminiAdapter struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// GeminiAdapterOption is a functional option for configuring GeminiAdapter.
type GeminiAdapterOption func(*GeminiAdapter)

// WithBaseURL sets a custom base URL for the Gemini API.
func WithBaseURL(url string) GeminiAdapterOption {
	return func(g *GeminiAdapter) {
		g.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) GeminiAdapterOption {
	return func(g *GeminiAdapter) {
		g.httpClient = client
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) GeminiAdapterOption {
	return func(g *GeminiAdapter) {
		g.httpClient.Timeout = timeout
	}
}

// NewGeminiAdapter creates a new GeminiAdapter with the given API key.
func NewGeminiAdapter(apiKey string, opts ...GeminiAdapterOption) *GeminiAdapter {
	g := &GeminiAdapter{
		apiKey:  apiKey,
		baseURL: DefaultGeminiBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(g)
	}

	return g
}

// Name returns the provider identifier.
func (g *GeminiAdapter) Name() string {
	return "gemini"
}

// ChatCompletion performs a chat completion request using Gemini API.
// It translates the OpenAI request to Gemini format, makes the API call,
// and translates the response back to OpenAI format.
func (g *GeminiAdapter) ChatCompletion(ctx context.Context, req OpenAIRequest) (OpenAIResponse, error) {
	// Map OpenAI request to Gemini request
	geminiReq := g.mapToGeminiRequest(req)

	// Build the API URL
	model := g.mapModelName(req.Model)
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", g.baseURL, model, g.apiKey)

	// Marshal the request body
	body, err := json.Marshal(geminiReq)
	if err != nil {
		return OpenAIResponse{}, fmt.Errorf("failed to marshal gemini request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return OpenAIResponse{}, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return OpenAIResponse{}, fmt.Errorf("failed to execute gemini request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return OpenAIResponse{}, fmt.Errorf("failed to read gemini response: %w", err)
	}

	// Check for API errors
	if resp.StatusCode != http.StatusOK {
		var geminiErr GeminiErrorResponse
		if err := json.Unmarshal(respBody, &geminiErr); err == nil && geminiErr.Error.Message != "" {
			return OpenAIResponse{}, fmt.Errorf("gemini API error [%d]: %s", resp.StatusCode, geminiErr.Error.Message)
		}
		return OpenAIResponse{}, fmt.Errorf("gemini API error [%d]: %s", resp.StatusCode, string(respBody))
	}

	// Parse Gemini response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return OpenAIResponse{}, fmt.Errorf("failed to unmarshal gemini response: %w", err)
	}

	// Map Gemini response to OpenAI response
	return g.mapToOpenAIResponse(geminiResp, req.Model), nil
}

// mapToGeminiRequest converts an OpenAI request to Gemini format.
func (g *GeminiAdapter) mapToGeminiRequest(req OpenAIRequest) GeminiRequest {
	geminiReq := GeminiRequest{
		Contents:         make([]GeminiContent, 0),
		GenerationConfig: GeminiGenerationConfig{},
	}

	var systemInstruction string

	// Process messages and handle role mapping
	for _, msg := range req.Messages {
		switch msg.Role {
		case "system":
			// Gemini doesn't have a system role - prepend to context or use systemInstruction
			systemInstruction = msg.Content
		case "user":
			geminiReq.Contents = append(geminiReq.Contents, GeminiContent{
				Role: "user",
				Parts: []GeminiPart{
					{Text: msg.Content},
				},
			})
		case "assistant":
			// OpenAI "assistant" maps to Gemini "model"
			geminiReq.Contents = append(geminiReq.Contents, GeminiContent{
				Role: "model",
				Parts: []GeminiPart{
					{Text: msg.Content},
				},
			})
		}
	}

	// If there's a system message, add it as systemInstruction
	if systemInstruction != "" {
		geminiReq.SystemInstruction = &GeminiContent{
			Parts: []GeminiPart{
				{Text: systemInstruction},
			},
		}
	}

	// Map generation config
	if req.Temperature != nil {
		geminiReq.GenerationConfig.Temperature = req.Temperature
	}
	if req.MaxTokens != nil {
		geminiReq.GenerationConfig.MaxOutputTokens = req.MaxTokens
	}
	if req.TopP != nil {
		geminiReq.GenerationConfig.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		geminiReq.GenerationConfig.StopSequences = req.Stop
	}

	return geminiReq
}

// mapToOpenAIResponse converts a Gemini response to OpenAI format.
func (g *GeminiAdapter) mapToOpenAIResponse(resp GeminiResponse, model string) OpenAIResponse {
	openAIResp := OpenAIResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: make([]OpenAIChoice, 0),
		Usage:   OpenAIUsage{},
	}

	// Map candidates to choices
	for i, candidate := range resp.Candidates {
		content := ""
		if len(candidate.Content.Parts) > 0 {
			content = candidate.Content.Parts[0].Text
		}

		choice := OpenAIChoice{
			Index: i,
			Message: OpenAIMessage{
				Role:    "assistant",
				Content: content,
			},
			FinishReason: g.mapFinishReason(candidate.FinishReason),
		}

		openAIResp.Choices = append(openAIResp.Choices, choice)
	}

	// Map usage metadata
	if resp.UsageMetadata != nil {
		openAIResp.Usage = OpenAIUsage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return openAIResp
}

// mapModelName converts OpenAI model names to Gemini equivalents.
func (g *GeminiAdapter) mapModelName(model string) string {
	// Map common OpenAI model names to Gemini equivalents
	modelMap := map[string]string{
		"gpt-4":            "gemini-1.5-pro",
		"gpt-4-turbo":      "gemini-1.5-pro",
		"gpt-4o":           "gemini-1.5-flash",
		"gpt-4o-mini":      "gemini-1.5-flash-8b",
		"gpt-3.5-turbo":    "gemini-1.5-flash",
		"gemini-pro":       "gemini-1.5-pro",
		"gemini-1.5-pro":   "gemini-1.5-pro",
		"gemini-1.5-flash": "gemini-1.5-flash",
	}

	if mapped, ok := modelMap[model]; ok {
		return mapped
	}

	// If no mapping found, use the model name as-is (assume it's a Gemini model)
	return model
}

// mapFinishReason converts Gemini finish reasons to OpenAI format.
func (g *GeminiAdapter) mapFinishReason(reason string) string {
	reasonMap := map[string]string{
		"STOP":          "stop",
		"MAX_TOKENS":    "length",
		"SAFETY":        "content_filter",
		"RECITATION":    "content_filter",
		"OTHER":         "stop",
		"FINISH_REASON_UNSPECIFIED": "stop",
	}

	if mapped, ok := reasonMap[reason]; ok {
		return mapped
	}

	return "stop"
}

// ============================================================================
// Gemini API Types
// ============================================================================

// GeminiRequest represents a Gemini generateContent request.
type GeminiRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  GeminiGenerationConfig  `json:"generationConfig,omitempty"`
	SafetySettings    []GeminiSafetySetting   `json:"safetySettings,omitempty"`
}

// GeminiContent represents a content block in Gemini format.
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of a content block.
type GeminiPart struct {
	Text string `json:"text,omitempty"`
}

// GeminiGenerationConfig contains generation parameters.
type GeminiGenerationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	TopK            *int     `json:"topK,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

// GeminiSafetySetting configures content safety filtering.
type GeminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// GeminiResponse represents a Gemini generateContent response.
type GeminiResponse struct {
	Candidates    []GeminiCandidate    `json:"candidates"`
	UsageMetadata *GeminiUsageMetadata `json:"usageMetadata,omitempty"`
}

// GeminiCandidate represents a single generated candidate.
type GeminiCandidate struct {
	Content       GeminiContent        `json:"content"`
	FinishReason  string               `json:"finishReason"`
	Index         int                  `json:"index"`
	SafetyRatings []GeminiSafetyRating `json:"safetyRatings,omitempty"`
}

// GeminiSafetyRating contains safety evaluation for a response.
type GeminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
}

// GeminiUsageMetadata contains token usage information.
type GeminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GeminiErrorResponse represents an error response from Gemini API.
type GeminiErrorResponse struct {
	Error GeminiErrorDetail `json:"error"`
}

// GeminiErrorDetail contains error details.
type GeminiErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}
