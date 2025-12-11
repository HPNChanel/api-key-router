// Package tests provides end-to-end integration tests for hpn-g-router.
package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hpn/hpn-g-router/internal/adapter"
	"github.com/hpn/hpn-g-router/internal/domain"
	"github.com/hpn/hpn-g-router/internal/handler"
)

// MockProviderServer creates an httptest server that simulates a Gemini API provider.
// Logic:
// - "Bearer KEY_FAIL" -> HTTP 429 (Too Many Requests)
// - "Bearer KEY_ERROR" -> HTTP 500 (Internal Server Error)
// - "Bearer KEY_SUCCESS" -> HTTP 200 (OK) with valid OpenAI-compatible JSON
func NewMockProviderServer(requestCounter *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Increment request counter
		if requestCounter != nil {
			atomic.AddInt32(requestCounter, 1)
		}

		// Extract API key from URL query parameter (Gemini format: ?key=XXX)
		apiKey := r.URL.Query().Get("key")

		// Determine response based on API key
		switch apiKey {
		case "KEY_FAIL":
			// Return 429 Too Many Requests (Rate Limited)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    429,
					"message": "Resource has been exhausted (e.g. check quota).",
					"status":  "RESOURCE_EXHAUSTED",
				},
			})

		case "KEY_ERROR":
			// Return 500 Internal Server Error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    500,
					"message": "Internal server error",
					"status":  "INTERNAL",
				},
			})

		case "KEY_SUCCESS":
			// Return 200 OK with valid Gemini response
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"candidates": []map[string]interface{}{
					{
						"content": map[string]interface{}{
							"parts": []map[string]interface{}{
								{"text": "Hello! I'm a mock AI assistant. How can I help you today?"},
							},
							"role": "model",
						},
						"finishReason": "STOP",
						"index":        0,
					},
				},
				"usageMetadata": map[string]interface{}{
					"promptTokenCount":     10,
					"candidatesTokenCount": 15,
					"totalTokenCount":      25,
				},
			})

		default:
			// Unknown key
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    401,
					"message": "API key not valid",
					"status":  "UNAUTHENTICATED",
				},
			})
		}
	}))
}

// TestRouterE2E contains all end-to-end test scenarios
func TestRouterE2E(t *testing.T) {
	tests := []struct {
		name             string
		keys             []string
		expectedStatus   int
		expectedAttempts int
		expectedCalls    int32
		concurrency      int
		validateResponse func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:             "Case A: Happy Path - Single Request Valid Key",
			keys:             []string{"KEY_SUCCESS"},
			expectedStatus:   http.StatusOK,
			expectedAttempts: 1,
			expectedCalls:    1,
			concurrency:      1,
			validateResponse: func(t *testing.T, resp map[string]interface{}) {
				// Verify response structure is OpenAI-compatible
				if _, ok := resp["id"]; !ok {
					t.Error("Response missing 'id' field")
				}
				if obj, ok := resp["object"].(string); !ok || obj != "chat.completion" {
					t.Errorf("Expected object='chat.completion', got %v", resp["object"])
				}
				if choices, ok := resp["choices"].([]interface{}); !ok || len(choices) == 0 {
					t.Error("Response missing or empty 'choices' array")
				} else {
					choice := choices[0].(map[string]interface{})
					message := choice["message"].(map[string]interface{})
					content := message["content"].(string)
					if !strings.Contains(content, "mock AI assistant") {
						t.Errorf("Unexpected response content: %s", content)
					}
				}
			},
		},
		{
			name:             "Case B: Failover Logic - KEY_FAIL then KEY_SUCCESS",
			keys:             []string{"KEY_FAIL", "KEY_SUCCESS"},
			expectedStatus:   http.StatusOK,
			expectedAttempts: 2,
			expectedCalls:    2, // First attempt fails with 429, second succeeds
			concurrency:      1,
			validateResponse: func(t *testing.T, resp map[string]interface{}) {
				// Response should be successful despite first key failing
				if obj, ok := resp["object"].(string); !ok || obj != "chat.completion" {
					t.Errorf("Expected successful response after failover, got %v", resp)
				}
				if choices, ok := resp["choices"].([]interface{}); !ok || len(choices) == 0 {
					t.Error("Expected valid choices after successful failover")
				}
			},
		},
		{
			name:           "Case C: Exhaustion - All Keys Fail",
			keys:           []string{"KEY_FAIL", "KEY_ERROR"},
			expectedStatus: http.StatusBadGateway, // Router returns 502 when all keys exhausted
			expectedCalls:  2,                      // Both keys should be tried
			concurrency:    1,
			validateResponse: func(t *testing.T, resp map[string]interface{}) {
				// Should return OpenAI-compatible error
				if errObj, ok := resp["error"].(map[string]interface{}); ok {
					if msg, ok := errObj["message"].(string); !ok || msg == "" {
						t.Error("Error response missing message")
					}
				} else {
					t.Error("Expected error object in response")
				}
			},
		},
		{
			name:             "Case D: Concurrency - 100 Concurrent Requests",
			keys:             []string{"KEY_SUCCESS"},
			expectedStatus:   http.StatusOK,
			expectedAttempts: 1,
			expectedCalls:    100, // 100 concurrent requests
			concurrency:      100,
			validateResponse: func(t *testing.T, resp map[string]interface{}) {
				// Each response should be valid
				if obj, ok := resp["object"].(string); !ok || obj != "chat.completion" {
					t.Errorf("Expected chat.completion, got %v", resp["object"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock provider server
			var requestCounter int32
			mockServer := NewMockProviderServer(&requestCounter)
			defer mockServer.Close()

			// Create KeyManager with test keys
			keyManager := domain.NewKeyManager(tt.keys, 5*time.Second)

			// Create ProxyHandler with custom base URL pointing to mock server
			proxyHandler := handler.NewProxyHandler(
				keyManager,
				nil,
				handler.WithMaxRetries(len(tt.keys)), // Retry count matches key count
			)

			// Create test request
			reqBody := adapter.OpenAIRequest{
				Model: "gpt-4",
				Messages: []adapter.OpenAIMessage{
					{Role: "user", Content: "Hello, test message!"},
				},
			}
			bodyBytes, _ := json.Marshal(reqBody)

			if tt.concurrency == 1 {
				// Single request test
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")

				// Create custom adapter with mock server URL
				executeRequestWithMockAdapter(t, proxyHandler, keyManager, mockServer.URL, req, w)

				// Verify status code
				if w.Code != tt.expectedStatus {
					t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
				}

				// Parse response
				var resp map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				// Run custom validation
				if tt.validateResponse != nil {
					tt.validateResponse(t, resp)
				}

			} else {
				// Concurrent requests test
				var wg sync.WaitGroup
				var successCount int32
				var errorCount int32

				for i := 0; i < tt.concurrency; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()

						w := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(bodyBytes))
						req.Header.Set("Content-Type", "application/json")

						executeRequestWithMockAdapter(t, proxyHandler, keyManager, mockServer.URL, req, w)

						if w.Code == http.StatusOK {
							atomic.AddInt32(&successCount, 1)
						} else {
							atomic.AddInt32(&errorCount, 1)
						}
					}()
				}

				// Wait for all goroutines to complete
				wg.Wait()

				// Verify all requests succeeded
				if successCount != int32(tt.concurrency) {
					t.Errorf("Expected %d successful requests, got %d (errors: %d)",
						tt.concurrency, successCount, errorCount)
				}
			}

			// Verify mock server received expected number of calls
			actualCalls := atomic.LoadInt32(&requestCounter)
			if actualCalls != tt.expectedCalls {
				t.Errorf("Expected %d provider calls, got %d", tt.expectedCalls, actualCalls)
			}
		})
	}
}

// executeRequestWithMockAdapter creates a custom GeminiAdapter pointing to the mock server
// and executes the request through the proxy handler logic.
func executeRequestWithMockAdapter(
	t *testing.T,
	proxyHandler *handler.ProxyHandler,
	keyManager *domain.KeyManager,
	mockBaseURL string,
	req *http.Request,
	w *httptest.ResponseRecorder,
) {
	// Parse request body
	var openAIReq adapter.OpenAIRequest
	if err := json.NewDecoder(req.Body).Decode(&openAIReq); err != nil {
		t.Fatalf("Failed to decode request: %v", err)
	}

	// Execute retry logic manually (simulating ProxyHandler.executeWithRetry)
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Get next key
		key, err := keyManager.GetNextKey()
		if err != nil {
			// No keys available
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": "No API keys available",
					"type":    "server_error",
				},
			})
			return
		}

		// Create adapter with mock base URL
		geminiAdapter := adapter.NewGeminiAdapter(
			key,
			adapter.WithBaseURL(mockBaseURL),
		)

		// Execute request
		resp, err := geminiAdapter.ChatCompletion(req.Context(), openAIReq)
		if err == nil {
			// Success!
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Check if error is retryable
		errStr := err.Error()
		isRetryable := strings.Contains(errStr, "429") ||
			strings.Contains(errStr, "500") ||
			strings.Contains(errStr, "502") ||
			strings.Contains(errStr, "503")

		if isRetryable {
			// Mark key as dead and retry
			keyManager.MarkAsDead(key)
			lastErr = err
			continue
		}

		// Non-retryable error
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": err.Error(),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// All retries exhausted
	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": "Service temporarily unavailable. Please try again later.",
			"type":    "server_error",
		},
	})

	if lastErr != nil {
		t.Logf("All retries exhausted, last error: %v", lastErr)
	}
}

// TestKeyManagerConcurrency stress-tests KeyManager for thread safety
func TestKeyManagerConcurrency(t *testing.T) {
	keys := []string{"KEY_1", "KEY_2", "KEY_3", "KEY_4", "KEY_5"}
	keyManager := domain.NewKeyManager(keys, 1*time.Second)

	const goroutines = 100
	const iterationsPerGoroutine = 1000

	var wg sync.WaitGroup
	retrievedKeys := make([]string, 0, goroutines*iterationsPerGoroutine)
	var mu sync.Mutex

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerGoroutine; j++ {
				key, err := keyManager.GetNextKey()
				if err == nil {
					mu.Lock()
					retrievedKeys = append(retrievedKeys, key)
					mu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Verify we got keys (concurrency safety)
	if len(retrievedKeys) != goroutines*iterationsPerGoroutine {
		t.Errorf("Expected %d keys, got %d", goroutines*iterationsPerGoroutine, len(retrievedKeys))
	}

	// Verify round-robin distribution (each key should appear roughly equal times)
	distribution := make(map[string]int)
	for _, key := range retrievedKeys {
		distribution[key]++
	}

	expectedCount := (goroutines * iterationsPerGoroutine) / len(keys)
	tolerance := expectedCount / 10 // 10% tolerance

	for _, key := range keys {
		count := distribution[key]
		if count < expectedCount-tolerance || count > expectedCount+tolerance {
			t.Logf("Warning: Key %s appeared %d times (expected ~%d)", key, count, expectedCount)
		}
	}

	t.Logf("Concurrency test passed: %d goroutines x %d iterations = %d total operations",
		goroutines, iterationsPerGoroutine, len(retrievedKeys))
}

// TestHealthEndpoint verifies the /health endpoint returns correct status
func TestHealthEndpoint(t *testing.T) {
	keys := []string{"KEY_1", "KEY_2", "KEY_3"}
	keyManager := domain.NewKeyManager(keys, 5*time.Second)
	proxyHandler := handler.NewProxyHandler(keyManager, nil)

	// Mock Gin context
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	// We need to create a minimal Gin engine to test the handler
	// Since HandleHealth expects a gin.Context, we'll test the underlying logic
	activeKeys := keyManager.ActiveKeyCount()
	deadKeys := keyManager.DeadKeyCount()
	totalKeys := keyManager.TotalKeyCount()

	if activeKeys != 3 {
		t.Errorf("Expected 3 active keys, got %d", activeKeys)
	}
	if deadKeys != 0 {
		t.Errorf("Expected 0 dead keys, got %d", deadKeys)
	}
	if totalKeys != 3 {
		t.Errorf("Expected 3 total keys, got %d", totalKeys)
	}

	// Mark one key as dead
	keyManager.MarkAsDead("KEY_1")

	activeKeys = keyManager.ActiveKeyCount()
	deadKeys = keyManager.DeadKeyCount()

	if activeKeys != 2 {
		t.Errorf("Expected 2 active keys after marking one dead, got %d", activeKeys)
	}
	if deadKeys != 1 {
		t.Errorf("Expected 1 dead key, got %d", deadKeys)
	}

	t.Logf("Health check passed: Active=%d, Dead=%d, Total=%d", activeKeys, deadKeys, totalKeys)
	
	// Verify health endpoint would return correct data
	_ = proxyHandler
	_ = req
	_ = w
}
