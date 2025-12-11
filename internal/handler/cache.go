// Package handler provides HTTP handlers for the API router.
package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hpn/hpn-g-router/internal/ui"
)

// ══════════════════════════════════════════════════════════════════════════════
// THE FLASH CACHE - In-Memory Response Caching
// ══════════════════════════════════════════════════════════════════════════════
//
// Data Structure: Thread-safe map with RWMutex
// Key: SHA256 hash of request body
// Value: Cached API response with TTL
// TTL: 5 minutes (configurable)
//
// ══════════════════════════════════════════════════════════════════════════════

const (
	// DefaultCacheTTL is the default time-to-live for cache entries.
	DefaultCacheTTL = 5 * time.Minute

	// CleanupInterval is how often the cache cleaner runs.
	CleanupInterval = 1 * time.Minute
)

// CacheEntry represents a cached response with expiration time.
type CacheEntry struct {
	Response  []byte    // Serialized JSON response
	ExpireAt  time.Time // When this entry expires
	CreatedAt time.Time // When this entry was created
}

// IsExpired returns true if the cache entry has expired.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpireAt)
}

// FlashCache is a thread-safe in-memory cache for API responses.
type FlashCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
	logger  *slog.Logger

	// Stats
	hits   int64
	misses int64
}

// FlashCacheOption is a functional option for configuring FlashCache.
type FlashCacheOption func(*FlashCache)

// WithCacheTTL sets a custom TTL for cache entries.
func WithCacheTTL(ttl time.Duration) FlashCacheOption {
	return func(c *FlashCache) {
		c.ttl = ttl
	}
}

// WithCacheLogger sets a custom logger.
func WithCacheLogger(logger *slog.Logger) FlashCacheOption {
	return func(c *FlashCache) {
		c.logger = logger
	}
}

// NewFlashCache creates a new FlashCache instance.
// It starts a background goroutine for TTL cleanup.
func NewFlashCache(opts ...FlashCacheOption) *FlashCache {
	c := &FlashCache{
		entries: make(map[string]*CacheEntry),
		ttl:     DefaultCacheTTL,
		logger:  slog.Default(),
	}

	for _, opt := range opts {
		opt(c)
	}

	// Start background cleanup goroutine
	go c.startCleanup()

	return c
}

// HashRequest generates a SHA256 hash of the request body.
// This hash is used as the cache key.
func HashRequest(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response by key.
// Returns the response bytes and a boolean indicating if the entry was found and valid.
func (c *FlashCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	// Check if expired
	if entry.IsExpired() {
		c.mu.Lock()
		delete(c.entries, key)
		c.misses++
		c.mu.Unlock()
		return nil, false
	}

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()

	return entry.Response, true
}

// Set stores a response in the cache with the configured TTL.
func (c *FlashCache) Set(key string, response []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &CacheEntry{
		Response:  response,
		ExpireAt:  time.Now().Add(c.ttl),
		CreatedAt: time.Now(),
	}
}

// startCleanup runs a background goroutine that periodically removes expired entries.
func (c *FlashCache) startCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes all expired entries from the cache.
func (c *FlashCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expired := 0

	for key, entry := range c.entries {
		if now.After(entry.ExpireAt) {
			delete(c.entries, key)
			expired++
		}
	}

	if expired > 0 && c.logger != nil {
		c.logger.Debug("cache cleanup",
			slog.Int("expired_entries", expired),
			slog.Int("remaining_entries", len(c.entries)),
		)
	}
}

// Stats returns cache hit/miss statistics.
func (c *FlashCache) Stats() (hits, misses int64, size int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses, len(c.entries)
}

// ══════════════════════════════════════════════════════════════════════════════
// CACHE MIDDLEWARE
// ══════════════════════════════════════════════════════════════════════════════

// CacheMiddleware returns a Gin middleware that caches API responses.
// Flow:
//  1. Hash the request body (SHA256)
//  2. Check cache: HIT → Return immediately with ⚡ CACHE HIT log
//  3. MISS → Continue to handler, cache the response
func CacheMiddleware(cache *FlashCache, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only cache POST requests to chat completions
		if c.Request.Method != "POST" || 
			(c.Request.URL.Path != "/v1/chat/completions" && c.Request.URL.Path != "/chat/completions") {
			c.Next()
			return
		}

		// Read request body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}

		// Restore body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Generate cache key
		cacheKey := HashRequest(bodyBytes)

		// Check cache
		if cachedResponse, found := cache.Get(cacheKey); found {
			// ⚡ CACHE HIT!
			start := time.Now()
			latency := time.Since(start) // ~0ms

			// Log cache hit
			if logger != nil {
				logger.Info("cache hit",
					slog.String("cache_key", cacheKey[:12]+"..."),
					slog.Duration("latency", latency),
				)
			}

			// Print styled cache hit message
			ui.PrintCacheHit(cacheKey, latency)

			// Set cache hit flag for logging middleware
			c.Set("cache_hit", true)

			// Return cached response directly
			c.Data(http.StatusOK, "application/json", cachedResponse)
			c.Abort()
			return
		}

		// CACHE MISS - Continue to handler
		// Use a response writer wrapper to capture the response
		writer := &responseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = writer

		// Process request
		c.Next()

		// Only cache successful responses (200 OK)
		if c.Writer.Status() == http.StatusOK {
			cache.Set(cacheKey, writer.body.Bytes())

			if logger != nil {
				logger.Debug("response cached",
					slog.String("cache_key", cacheKey[:12]+"..."),
					slog.Int("size_bytes", writer.body.Len()),
				)
			}
		}
	}
}

// responseWriter wraps gin.ResponseWriter to capture the response body.
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write captures the response body while writing to the original writer.
func (w *responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}
