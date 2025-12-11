// Package handler provides HTTP handlers for the API router.
package handler

import (
	"fmt"
	"strings"
	"sync"
	"unicode"
)

// OpenAI pricing per 1 million tokens (USD)
const (
	// InputPricePerMillion is the cost per million input tokens ($0.50)
	InputPricePerMillion = 0.50
	// OutputPricePerMillion is the cost per million output tokens ($1.50)
	OutputPricePerMillion = 1.50
	// TokensPerWord is the approximation ratio (1 word ≈ 1.3 tokens)
	TokensPerWord = 1.3
)

// CostEstimator tracks token usage and calculates money saved.
// It uses a global counter that persists across requests.
type CostEstimator struct {
	mu         sync.RWMutex
	totalSaved float64
}

// globalCostEstimator is the singleton instance for tracking total savings.
var globalCostEstimator = &CostEstimator{}

// GetTotalSaved returns the total money saved across all requests.
func GetTotalSaved() float64 {
	globalCostEstimator.mu.RLock()
	defer globalCostEstimator.mu.RUnlock()
	return globalCostEstimator.totalSaved
}

// AddSavings adds to the total savings counter (thread-safe).
func AddSavings(amount float64) float64 {
	globalCostEstimator.mu.Lock()
	defer globalCostEstimator.mu.Unlock()
	globalCostEstimator.totalSaved += amount
	return globalCostEstimator.totalSaved
}

// ResetSavings resets the total savings counter (useful for testing).
func ResetSavings() {
	globalCostEstimator.mu.Lock()
	defer globalCostEstimator.mu.Unlock()
	globalCostEstimator.totalSaved = 0
}

// EstimateTokens estimates the number of tokens in a text string.
// Uses a lightweight approximation: 1 word ≈ 1.3 tokens.
// This avoids external dependencies while providing reasonable accuracy.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Count words by splitting on whitespace and punctuation
	wordCount := 0
	inWord := false

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			if !inWord {
				wordCount++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	// Apply the 1.3 multiplier and round up
	tokens := int(float64(wordCount) * TokensPerWord)
	if tokens == 0 && wordCount > 0 {
		tokens = 1 // Minimum 1 token if there's any text
	}

	return tokens
}

// CalculateCost calculates the equivalent OpenAI API cost in USD.
// Returns the cost based on OpenAI's pricing:
// - Input: $0.50 per million tokens
// - Output: $1.50 per million tokens
func CalculateCost(inputTokens, outputTokens int) float64 {
	inputCost := (float64(inputTokens) / 1_000_000) * InputPricePerMillion
	outputCost := (float64(outputTokens) / 1_000_000) * OutputPricePerMillion
	return inputCost + outputCost
}

// ExtractInputText extracts all text content from OpenAI-compatible messages.
// It concatenates all message contents for token counting.
func ExtractInputText(messages []map[string]interface{}) string {
	var builder strings.Builder

	for _, msg := range messages {
		if content, ok := msg["content"].(string); ok {
			builder.WriteString(content)
			builder.WriteString(" ")
		}
	}

	return builder.String()
}

// FormatMoneySaved formats the savings as a human-readable string.
func FormatMoneySaved(amount float64) string {
	if amount < 0.0001 {
		return fmt.Sprintf("$%.6f", amount)
	} else if amount < 0.01 {
		return fmt.Sprintf("$%.4f", amount)
	}
	return fmt.Sprintf("$%.2f", amount)
}

// FormatTotalSaved formats the total savings with appropriate precision.
func FormatTotalSaved(amount float64) string {
	if amount < 0.01 {
		return fmt.Sprintf("$%.4f", amount)
	}
	return fmt.Sprintf("$%.2f", amount)
}

// CostMetrics holds the cost calculation results for a single request.
type CostMetrics struct {
	InputTokens  int
	OutputTokens int
	MoneySaved   float64
	TotalSaved   float64
}

// CalculateRequestCost calculates cost metrics for a request/response pair.
func CalculateRequestCost(inputText, outputText string) CostMetrics {
	inputTokens := EstimateTokens(inputText)
	outputTokens := EstimateTokens(outputText)
	moneySaved := CalculateCost(inputTokens, outputTokens)
	totalSaved := AddSavings(moneySaved)

	return CostMetrics{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		MoneySaved:   moneySaved,
		TotalSaved:   totalSaved,
	}
}
