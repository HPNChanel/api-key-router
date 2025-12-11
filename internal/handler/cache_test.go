// Package handler_test provides unit tests for the cache package.
package handler

import (
	"testing"
	"time"
)

// ============================================================================
// FLASH CACHE UNIT TESTS
// ============================================================================

// TestHashRequest verifies that the SHA256 hash function produces consistent hashes.
func TestHashRequest(t *testing.T) {
	t.Log("=== TEST: Hash Request ===")

	body := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}`)

	// Hash should be consistent
	hash1 := HashRequest(body)
	hash2 := HashRequest(body)

	if hash1 != hash2 {
		t.Errorf("Expected consistent hash, got %s != %s", hash1, hash2)
	} else {
		t.Logf("✓ Hash is consistent: %s", hash1[:12]+"...")
	}

	// Different body should produce different hash
	differentBody := []byte(`{"model":"gpt-4","messages":[{"role":"user","content":"world"}]}`)
	hash3 := HashRequest(differentBody)

	if hash1 == hash3 {
		t.Errorf("Expected different hash for different body, got same hash")
	} else {
		t.Log("✓ Different bodies produce different hashes")
	}

	t.Log("=== TEST PASSED: Hash Request ===")
}

// TestFlashCacheGetSet tests basic cache get/set operations.
func TestFlashCacheGetSet(t *testing.T) {
	t.Log("=== TEST: Flash Cache Get/Set ===")

	cache := NewFlashCache()

	key := "test-key-123"
	value := []byte(`{"id":"chatcmpl-123","object":"chat.completion"}`)

	// Initially cache should be empty
	_, found := cache.Get(key)
	if found {
		t.Errorf("Expected cache miss for new key")
	} else {
		t.Log("✓ Cache miss for new key")
	}

	// Set value
	cache.Set(key, value)

	// Now should be found
	cached, found := cache.Get(key)
	if !found {
		t.Errorf("Expected cache hit after set")
	} else {
		t.Log("✓ Cache hit after set")
	}

	// Value should match
	if string(cached) != string(value) {
		t.Errorf("Expected cached value to match, got %s", string(cached))
	} else {
		t.Log("✓ Cached value matches original")
	}

	t.Log("=== TEST PASSED: Flash Cache Get/Set ===")
}

// TestFlashCacheExpiration tests that cache entries expire after TTL.
func TestFlashCacheExpiration(t *testing.T) {
	t.Log("=== TEST: Flash Cache Expiration ===")

	// Use very short TTL for testing (100ms)
	cache := NewFlashCache(WithCacheTTL(100 * time.Millisecond))

	key := "expiring-key"
	value := []byte(`{"expires":"soon"}`)

	// Set value
	cache.Set(key, value)

	// Should be found immediately
	_, found := cache.Get(key)
	if !found {
		t.Errorf("Expected cache hit immediately after set")
	} else {
		t.Log("✓ Cache hit immediately after set")
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should now be expired
	_, found = cache.Get(key)
	if found {
		t.Errorf("Expected cache miss after TTL expiration")
	} else {
		t.Log("✓ Cache miss after TTL expiration")
	}

	t.Log("=== TEST PASSED: Flash Cache Expiration ===")
}

// TestFlashCacheStats tests cache statistics tracking.
func TestFlashCacheStats(t *testing.T) {
	t.Log("=== TEST: Flash Cache Stats ===")

	cache := NewFlashCache()

	// Initial stats
	hits, misses, size := cache.Stats()
	if hits != 0 || misses != 0 || size != 0 {
		t.Errorf("Expected empty stats, got hits=%d misses=%d size=%d", hits, misses, size)
	}

	// One miss
	cache.Get("nonexistent")
	hits, misses, size = cache.Stats()
	if misses != 1 {
		t.Errorf("Expected 1 miss, got %d", misses)
	}

	// Set and hit
	cache.Set("key1", []byte("value1"))
	cache.Get("key1")
	hits, misses, size = cache.Stats()
	if hits != 1 {
		t.Errorf("Expected 1 hit, got %d", hits)
	}
	if size != 1 {
		t.Errorf("Expected size 1, got %d", size)
	}

	t.Logf("✓ Stats tracking works: hits=%d misses=%d size=%d", hits, misses, size)
	t.Log("=== TEST PASSED: Flash Cache Stats ===")
}

// TestFlashCacheConcurrency tests thread safety under concurrent access.
func TestFlashCacheConcurrency(t *testing.T) {
	t.Log("=== TEST: Flash Cache Concurrency ===")

	cache := NewFlashCache()

	// Run 100 concurrent goroutines
	done := make(chan bool, 100)

	for i := 0; i < 100; i++ {
		go func(id int) {
			key := "concurrent-key"
			value := []byte(`{"id":"test"}`)

			// Mix of reads and writes
			if id%2 == 0 {
				cache.Set(key, value)
			} else {
				cache.Get(key)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	t.Log("✓ No race conditions (run with -race to verify)")
	t.Log("=== TEST PASSED: Flash Cache Concurrency ===")
}
