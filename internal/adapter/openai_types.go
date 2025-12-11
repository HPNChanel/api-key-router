// Package adapter provides implementations for external AI provider integrations.
package adapter

// OpenAI-compatible request/response types.
// These types mirror the OpenAI API format for maximum compatibility.

// OpenAIRequest represents an OpenAI chat completion request.
type OpenAIRequest struct {
	// Model specifies which model to use (e.g., "gpt-4", "gemini-pro").
	Model string `json:"model"`

	// Messages contains the conversation history.
	Messages []OpenAIMessage `json:"messages"`

	// Temperature controls randomness (0.0-2.0). Optional.
	Temperature *float64 `json:"temperature,omitempty"`

	// MaxTokens limits the response length. Optional.
	MaxTokens *int `json:"max_tokens,omitempty"`

	// TopP is nucleus sampling parameter. Optional.
	TopP *float64 `json:"top_p,omitempty"`

	// N specifies how many completions to generate. Optional.
	N *int `json:"n,omitempty"`

	// Stream enables server-sent events for streaming. Optional.
	Stream bool `json:"stream,omitempty"`

	// Stop sequences to halt generation. Optional.
	Stop []string `json:"stop,omitempty"`

	// PresencePenalty penalizes new tokens based on presence in text. Optional.
	PresencePenalty *float64 `json:"presence_penalty,omitempty"`

	// FrequencyPenalty penalizes new tokens based on frequency in text. Optional.
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`

	// User is a unique identifier for the end-user. Optional.
	User string `json:"user,omitempty"`
}

// OpenAIMessage represents a single message in the conversation.
type OpenAIMessage struct {
	// Role is one of: "system", "user", "assistant", "function".
	Role string `json:"role"`

	// Content is the message text content.
	Content string `json:"content"`

	// Name is an optional name for the participant. Optional.
	Name string `json:"name,omitempty"`

	// FunctionCall contains function call information if role is "assistant". Optional.
	FunctionCall *OpenAIFunctionCall `json:"function_call,omitempty"`
}

// OpenAIFunctionCall represents a function call made by the model.
type OpenAIFunctionCall struct {
	// Name is the function name to call.
	Name string `json:"name"`

	// Arguments is the JSON string of function arguments.
	Arguments string `json:"arguments"`
}

// OpenAIResponse represents an OpenAI chat completion response.
type OpenAIResponse struct {
	// ID is the unique identifier for this completion.
	ID string `json:"id"`

	// Object is always "chat.completion".
	Object string `json:"object"`

	// Created is the Unix timestamp of when the completion was created.
	Created int64 `json:"created"`

	// Model is the model used for completion.
	Model string `json:"model"`

	// Choices contains the generated completions.
	Choices []OpenAIChoice `json:"choices"`

	// Usage contains token usage statistics.
	Usage OpenAIUsage `json:"usage"`

	// SystemFingerprint is the backend configuration fingerprint. Optional.
	SystemFingerprint string `json:"system_fingerprint,omitempty"`
}

// OpenAIChoice represents a single completion choice.
type OpenAIChoice struct {
	// Index is the position of this choice in the list.
	Index int `json:"index"`

	// Message contains the generated message.
	Message OpenAIMessage `json:"message"`

	// FinishReason indicates why the model stopped generating.
	// Values: "stop", "length", "function_call", "content_filter", null.
	FinishReason string `json:"finish_reason"`

	// Logprobs contains log probability information. Optional.
	Logprobs interface{} `json:"logprobs,omitempty"`
}

// OpenAIUsage contains token usage statistics.
type OpenAIUsage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int `json:"total_tokens"`
}

// OpenAIError represents an error response from OpenAI-compatible APIs.
type OpenAIError struct {
	Error OpenAIErrorDetail `json:"error"`
}

// OpenAIErrorDetail contains the error details.
type OpenAIErrorDetail struct {
	// Message is the human-readable error message.
	Message string `json:"message"`

	// Type categorizes the error (e.g., "invalid_request_error").
	Type string `json:"type"`

	// Param is the parameter that caused the error. Optional.
	Param string `json:"param,omitempty"`

	// Code is the error code. Optional.
	Code string `json:"code,omitempty"`
}
