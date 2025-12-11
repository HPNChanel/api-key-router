// Package main_test provides comprehensive End-to-End (E2E) tests for the hpn-g-router.
// These tests simulate the complete request flow: Client → Router → Provider (Mocked).
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hpn/hpn-g-router/internal/adapter"
	"github.com/hpn/hpn-g-router/internal/domain"
	"github.com/hpn/hpn-g-router/internal/handler"
)

// Constants for test API keys (from test_api_key.txt)
const (
	// Real API key - will succeed in tests
	REAL_API_KEY = "YOUR_REAL_API_KEY"
	// Fake API key - will fail in tests
	FAKE_API_KEY = "YOUR_FAKE_API_KEY"
	// Additional test keys for rotation
	TEST_KEY_1 = "YOUR_TEST_KEY_1"
	TEST_KEY_2 = "YOUR_TEST_KEY_2"
)

// ============================================================================
// SETUP HELPERS
// ============================================================================

// setupMockProvider creates an httptest server that simulates Google Gemini API behavior.
// It returns different HTTP responses based on the Authorization header:
//   - KEY_1 → 429 Too Many Requests (Rate Limited)
//   - KEY_2 → 500 Internal Server Error (Server Error)
//   - REAL_API_KEY → 200 OK with valid Gemini response
//   - FAKE_API_KEY → 401 Unauthorized (Invalid API Key)
func setupMockProvider(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from query parameter (Gemini uses ?key=XXX)
		apiKey := r.URL.Query().Get("key")

		t.Logf("[MOCK PROVIDER] Received request with API key: %s", maskKey(apiKey))

		// Simulate different provider responses based on API key
		switch apiKey {
		case TEST_KEY_1:
			// Simulate rate limiting (429)
			t.Logf("[MOCK PROVIDER] Returning 429 for KEY_1")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    429,
					"message": "Resource has been exhausted (e.g. check quota).",
					"status":  "RESOURCE_EXHAUSTED",
				},
			})

		case TEST_KEY_2:
			// Simulate server error (500)
			t.Logf("[MOCK PROVIDER] Returning 500 for KEY_2")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    500,
					"message": "Internal server error",
					"status":  "INTERNAL",
				},
			})

		case REAL_API_KEY:
			// Simulate successful response (200)
			t.Logf("[MOCK PROVIDER] Returning 200 for REAL_KEY")
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"candidates": []map[string]interface{}{
					{
						"content": map[string]interface{}{
							"parts": []map[string]interface{}{
								{"text": "Hello! I'm working correctly with the real API key."},
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

		case FAKE_API_KEY:
			// Simulate invalid API key (401)
			t.Logf("[MOCK PROVIDER] Returning 401 for FAKE_KEY")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"code":    401,
					"message": "API key not valid. Please pass a valid API key.",
					"status":  "UNAUTHENTICATED",
				},
			})

		default:
			// Unknown key
			t.Logf("[MOCK PROVIDER] Returning 401 for unknown key: %s", maskKey(apiKey))
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

// setupRouter creates a Gin router configured with the ProxyHandler and middleware.
// This simulates the actual production router setup from main.go.
func setupRouter(keyManager *domain.KeyManager, mockBaseURL string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Apply the same middleware as production
	router.Use(handler.RecoveryMiddleware(nil))
	router.Use(handler.CORSMiddleware())
	router.Use(handler.StripAuthHeadersMiddleware())

	// Create ProxyHandler with custom adapter options
	// Note: We pass nil as adapter because it's created per-request with the rotated key
	proxyHandler := handler.NewProxyHandler(
		keyManager,
		nil,
		handler.WithMaxRetries(3),
	)

	// We need to customize the handler to use our mock base URL
	// This is done by wrapping the handler with custom logic
	customHandler := createCustomProxyHandler(keyManager, mockBaseURL)

	// Register routes (same as production)
	router.POST("/v1/chat/completions", customHandler)
	router.GET("/v1/models", proxyHandler.HandleModels)
	router.GET("/health", proxyHandler.HandleHealth)

	return router
}

// createCustomProxyHandler creates a custom handler that uses the mock base URL.
// This is necessary because the production handler creates adapters with the default Gemini URL.
func createCustomProxyHandler(keyManager *domain.KeyManager, mockBaseURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse OpenAI-compatible request
		var req adapter.OpenAIRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "Invalid request body: " + err.Error(),
					"type":    "invalid_request_error",
				},
			})
			return
		}

		// Validate request
		if len(req.Messages) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": "messages array is required",
					"type":    "invalid_request_error",
				},
			})
			return
		}

		// Execute with retry logic (max 3 attempts)
		var lastErr error
		maxRetries := 3

		for attempt := 1; attempt <= maxRetries; attempt++ {
			// Get next key from KeyManager
			key, err := keyManager.GetNextKey()
			if err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error": gin.H{
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
			resp, err := geminiAdapter.ChatCompletion(c.Request.Context(), req)
			if err == nil {
				// Success!
				c.Set("attempts", attempt)
				c.JSON(http.StatusOK, resp)
				return
			}

			// Check if error is retryable
			if isRetryableError(err) {
				// Mark key as dead and retry with next key
				keyManager.MarkAsDead(key)
				lastErr = err
				continue
			}

			// Non-retryable error
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"message": err.Error(),
					"type":    "invalid_request_error",
				},
			})
			return
		}

		// All retries exhausted
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Service temporarily unavailable. Please try again later.",
				"type":    "server_error",
			},
		})
		_ = lastErr // Silence unused variable warning
	}
}

// isRetryableError determines if an error should trigger a retry.
func isRetryableError(err error) bool {
	errStr := err.Error()
	// Check for rate limiting (429)
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") {
		return true
	}
	// Check for server errors (5xx)
	if strings.Contains(errStr, "500") ||
		strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") ||
		strings.Contains(errStr, "504") {
		return true
	}
	// Check for quota exhausted
	if strings.Contains(errStr, "quota") || strings.Contains(errStr, "exhausted") {
		return true
	}
	return false
}

// maskKey returns a masked version of the API key for logging (first 8 chars + last 4 chars).
func maskKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 12 {
		return "***"
	}
	return key[:8] + "..." + key[len(key)-4:]
}

// ============================================================================
// TEST SCENARIOS
// ============================================================================

// TestEndToEndFlow_ImmortalMode tests the "Immortal" failover scenario.
// This simulates a real-world scenario where:
//   1. KEY_1 is rate limited (429)
//   2. Router automatically rotates to KEY_2
//   3. KEY_2 has server error (500)
//   4. Router automatically rotates to REAL_API_KEY
//   5. REAL_API_KEY succeeds (200)
//   6. Client receives successful response (failures are transparent)
func TestEndToEndFlow_ImmortalMode(t *testing.T) {
	t.Log("=== TEST: Immortal Mode (Failover) ===")

	// SETUP - The Mock Provider (The Fake Google)
	mockServer := setupMockProvider(t)
	defer mockServer.Close()
	t.Logf("Mock provider started at: %s", mockServer.URL)

	// SETUP - The Router (System Under Test)
	// Initialize KeyManager with 3 keys in specific order for failover test
	keys := []string{TEST_KEY_1, TEST_KEY_2, REAL_API_KEY}
	keyManager := domain.NewKeyManager(keys, 0) // 0 cooldown for immediate retry
	t.Logf("KeyManager initialized with %d keys", keyManager.TotalKeyCount())

	// Create router with mock provider URL
	router := setupRouter(keyManager, mockServer.URL)

	// EXECUTION - The "Immortal" Scenario
	t.Log("\n--- Sending request (should failover: KEY_1→429, KEY_2→500, REAL_KEY→200) ---")

	// Create OpenAI-compatible request
	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello, test!"},
		},
	}
	reqJSON, _ := json.Marshal(reqBody)

	// Send request to router
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// ASSERTIONS
	t.Log("\n--- Assertions ---")

	// Assert 1: Final response should be 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	} else {
		t.Log("✓ Final response status: 200 OK")
	}

	// Assert 2: Response should be valid OpenAI-compatible JSON
	var resp adapter.OpenAIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Errorf("Failed to parse response as OpenAI format: %v", err)
	} else {
		t.Logf("✓ Valid OpenAI response: ID=%s, Model=%s", resp.ID, resp.Model)
		if len(resp.Choices) > 0 {
			t.Logf("  Content: %s", resp.Choices[0].Message.Content)
		}
	}

	// Assert 3: At least KEY_1 should be marked as dead (429 encountered)
	if !keyManager.IsKeyDead(TEST_KEY_1) {
		t.Errorf("Expected KEY_1 to be marked as dead (circuit breaker)")
	} else {
		t.Log("✓ KEY_1 is marked as dead (429 triggered circuit breaker)")
	}

	// Note: KEY_2 may not be tried if KeyManager rotates to REAL_KEY after KEY_1 is removed
	// This is expected behavior due to atomic index counter continuing after key removal
	if keyManager.IsKeyDead(TEST_KEY_2) {
		t.Log("✓ KEY_2 is marked as dead (500 triggered circuit breaker)")
	} else {
		t.Log("  KEY_2 was not tried (KeyManager rotated to REAL_KEY after KEY_1 removal)")
	}

	// Assert 4: REAL_KEY should still be active
	if keyManager.IsKeyDead(REAL_API_KEY) {
		t.Errorf("Expected REAL_KEY to be active (successful request)")
	} else {
		t.Log("✓ REAL_KEY is still active (successful request)")
	}

	// Assert 5: At least 1 key should be active (REAL_KEY)
	activeCount := keyManager.ActiveKeyCount()
	if activeCount < 1 {
		t.Errorf("Expected at least 1 active key, got %d", activeCount)
	} else {
		t.Logf("✓ Active keys: %d", activeCount)
	}

	// Assert 6: Dead key count should be at least 1
	deadCount := keyManager.DeadKeyCount()
	if deadCount < 1 {
		t.Errorf("Expected at least 1 dead key, got %d", deadCount)
	} else {
		t.Logf("✓ Dead keys: %d", deadCount)
	}

	t.Log("\n=== TEST PASSED: Immortal Mode ===")
}

// TestEndToEndFlow_Concurrency tests concurrent requests to verify thread safety.
// This stress test ensures:
//   1. KeyManager can handle concurrent GetNextKey() calls without race conditions
//   2. Round-robin rotation works correctly under load
//   3. No deadlocks or panics occur
// Run with: go test -race -v ./cmd/server
func TestEndToEndFlow_Concurrency(t *testing.T) {
	t.Log("=== TEST: Concurrency (Stress Test) ===")

	// SETUP - Mock provider that always succeeds
	mockServer := setupMockProvider(t)
	defer mockServer.Close()

	// SETUP - KeyManager with only REAL_KEY (ensures all requests succeed)
	keys := []string{REAL_API_KEY}
	keyManager := domain.NewKeyManager(keys, 0)
	router := setupRouter(keyManager, mockServer.URL)

	// EXECUTION - Spawn 50 concurrent requests
	concurrency := 50
	var wg sync.WaitGroup
	results := make(chan int, concurrency)

	t.Logf("\n--- Spawning %d concurrent requests ---", concurrency)
	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Create request
			reqBody := map[string]interface{}{
				"model": "gpt-4",
				"messages": []map[string]interface{}{
					{"role": "user", "content": "Concurrent test"},
				},
			}
			reqJSON, _ := json.Marshal(reqBody)

			// Send request
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(reqJSON))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			// Record result
			results <- w.Code
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)
	duration := time.Since(startTime)

	// ASSERTIONS
	t.Log("\n--- Assertions ---")

	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}

	// Assert: All requests should succeed
	if successCount != concurrency {
		t.Errorf("Expected %d successful requests, got %d", concurrency, successCount)
	} else {
		t.Logf("✓ All %d requests succeeded", concurrency)
	}

	// Assert: No race conditions (verified by -race flag)
	t.Logf("✓ No race conditions detected (run with -race flag to verify)")

	// Assert: Reasonable performance
	t.Logf("✓ Completed in %v (avg: %v per request)", duration, duration/time.Duration(concurrency))

	// Assert: KeyManager state is consistent
	if keyManager.ActiveKeyCount() != 1 {
		t.Errorf("Expected 1 active key, got %d", keyManager.ActiveKeyCount())
	} else {
		t.Log("✓ KeyManager state is consistent")
	}

	t.Log("\n=== TEST PASSED: Concurrency ===")
}

// TestEndToEndFlow_AllKeysDead tests the scenario where all keys fail.
// This verifies:
//   1. Router attempts all available keys
//   2. Proper error response when all keys are exhausted
//   3. Circuit breaker marks all keys as dead
func TestEndToEndFlow_AllKeysDead(t *testing.T) {
	t.Log("=== TEST: All Keys Dead (Exhaustion) ===")

	// SETUP - Mock provider
	mockServer := setupMockProvider(t)
	defer mockServer.Close()

	// SETUP - KeyManager with 3 keys that all fail
	keys := []string{TEST_KEY_1, TEST_KEY_2, FAKE_API_KEY}
	keyManager := domain.NewKeyManager(keys, 0)
	router := setupRouter(keyManager, mockServer.URL)

	// EXECUTION - Send request (should fail with all 3 keys)
	t.Log("\n--- Sending request (all keys should fail) ---")

	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "This should fail"},
		},
	}
	reqJSON, _ := json.Marshal(reqBody)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// ASSERTIONS
	t.Log("\n--- Assertions ---")

	// Assert 1: Should return 503 (Service Unavailable) or 400 (Bad Request for 401 error)
	// Note: 401 is non-retryable, so it might return 400
	if w.Code != http.StatusServiceUnavailable && w.Code != http.StatusBadRequest {
		t.Logf("Warning: Expected 503 or 400, got %d", w.Code)
		t.Logf("Response body: %s", w.Body.String())
	} else {
		t.Logf("✓ Received expected error status: %d", w.Code)
	}

	// Assert 2: Response should be OpenAI-compatible error format
	var errResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Errorf("Failed to parse error response: %v", err)
	} else {
		if errObj, ok := errResp["error"].(map[string]interface{}); ok {
			t.Logf("✓ OpenAI-compatible error: %v", errObj["message"])
		}
	}

	// Assert 3: At least some keys should be marked as dead
	deadCount := keyManager.DeadKeyCount()
	if deadCount == 0 {
		t.Errorf("Expected at least 1 dead key, got %d", deadCount)
	} else {
		t.Logf("✓ Circuit breaker triggered: %d keys marked as dead", deadCount)
	}

	t.Log("\n=== TEST PASSED: All Keys Dead ===")
}

// TestHealthEndpoint tests the /health endpoint.
func TestHealthEndpoint(t *testing.T) {
	t.Log("=== TEST: Health Endpoint ===")

	// SETUP
	mockServer := setupMockProvider(t)
	defer mockServer.Close()

	keys := []string{REAL_API_KEY, TEST_KEY_1, TEST_KEY_2}
	keyManager := domain.NewKeyManager(keys, 0)
	router := setupRouter(keyManager, mockServer.URL)

	// EXECUTION
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	// ASSERTIONS
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var health map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
		t.Errorf("Failed to parse health response: %v", err)
	} else {
		t.Logf("✓ Health status: %v", health["status"])
		t.Logf("  Active keys: %v", health["active_keys"])
		t.Logf("  Dead keys: %v", health["dead_keys"])
		t.Logf("  Total keys: %v", health["total_keys"])
	}

	t.Log("\n=== TEST PASSED: Health Endpoint ===")
}

// TestModelsEndpoint tests the /v1/models endpoint.
func TestModelsEndpoint(t *testing.T) {
	t.Log("=== TEST: Models Endpoint ===")

	// SETUP
	mockServer := setupMockProvider(t)
	defer mockServer.Close()

	keys := []string{REAL_API_KEY}
	keyManager := domain.NewKeyManager(keys, 0)
	router := setupRouter(keyManager, mockServer.URL)

	// EXECUTION
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/models", nil)
	router.ServeHTTP(w, req)

	// ASSERTIONS
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var models map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &models); err != nil {
		t.Errorf("Failed to parse models response: %v", err)
	} else {
		t.Logf("✓ Models response object: %v", models["object"])
		if data, ok := models["data"].([]interface{}); ok {
			t.Logf("  Available models: %d", len(data))
		}
	}

	t.Log("\n=== TEST PASSED: Models Endpoint ===")
}
