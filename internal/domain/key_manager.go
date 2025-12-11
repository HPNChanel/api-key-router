// Package domain contains the core business entities and value objects.
package domain

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoKeysAvailable is returned when all keys are dead or the pool is empty.
var ErrNoKeysAvailable = errors.New("no keys available in the pool")

// KeyManager implements a thread-safe circular buffer for round-robin key selection.
// It uses atomic operations for the index counter and RWMutex for slice protection.
type KeyManager struct {
	// keys holds the list of active API keys.
	keys []string

	// deadKeys tracks temporarily removed keys with their death timestamp.
	// Key recovery is automatic based on cooldown duration.
	deadKeys map[string]time.Time

	// index is the atomic counter for round-robin selection.
	// Using int64 for atomic.AddInt64 compatibility.
	index int64

	// mu protects the keys slice during reads and writes.
	mu sync.RWMutex

	// deadMu protects the deadKeys map (separate mutex to reduce contention).
	deadMu sync.RWMutex

	// cooldown specifies how long a key remains dead before auto-revival.
	cooldown time.Duration

	// originalKeys stores the initial key set for revival operations.
	originalKeys map[string]struct{}
}

// NewKeyManager creates a new KeyManager with the given keys and cooldown duration.
// The cooldown duration determines how long a key stays dead before automatic revival.
// Pass 0 for cooldown to disable automatic revival (manual ReviveKey only).
func NewKeyManager(keys []string, cooldown time.Duration) *KeyManager {
	km := &KeyManager{
		keys:         make([]string, 0, len(keys)),
		deadKeys:     make(map[string]time.Time),
		index:        0,
		cooldown:     cooldown,
		originalKeys: make(map[string]struct{}),
	}

	// Initialize with unique keys only
	seen := make(map[string]struct{})
	for _, key := range keys {
		if key == "" {
			continue // Skip empty keys
		}
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			km.keys = append(km.keys, key)
			km.originalKeys[key] = struct{}{}
		}
	}

	return km
}

// GetNextKey returns the next available key using round-robin selection.
// This method is safe for concurrent use.
//
// Performance characteristics:
//   - Lock-free index increment via atomic.AddInt64
//   - Read lock only for slice access
//   - O(n) worst case when reviving dead keys, O(1) typical case
//
// Returns ErrNoKeysAvailable if no keys are available.
func (km *KeyManager) GetNextKey() (string, error) {
	// First, try to revive any expired dead keys
	km.reviveExpiredKeys()

	km.mu.RLock()
	keyCount := len(km.keys)
	if keyCount == 0 {
		km.mu.RUnlock()
		return "", ErrNoKeysAvailable
	}

	// Atomic increment and modulo for round-robin
	// Using atomic.AddInt64 returns the NEW value, so subtract 1 for current index
	newIdx := atomic.AddInt64(&km.index, 1)
	selectedIdx := int((newIdx - 1) % int64(keyCount))

	key := km.keys[selectedIdx]
	km.mu.RUnlock()

	return key, nil
}

// MarkAsDead temporarily removes a key from the rotation.
// This implements the Circuit Breaker pattern - when a key fails,
// it's removed from rotation for the cooldown duration.
//
// Thread-safe: uses write locks on both keys slice and deadKeys map.
func (km *KeyManager) MarkAsDead(key string) {
	if key == "" {
		return
	}

	// Check if this key is in our original set
	if _, exists := km.originalKeys[key]; !exists {
		return // Not a managed key
	}

	// Add to dead keys map with timestamp
	km.deadMu.Lock()
	km.deadKeys[key] = time.Now()
	km.deadMu.Unlock()

	// Remove from active keys slice
	km.mu.Lock()
	defer km.mu.Unlock()

	// Find and remove the key (maintain order for predictable round-robin)
	newKeys := make([]string, 0, len(km.keys))
	for _, k := range km.keys {
		if k != key {
			newKeys = append(newKeys, k)
		}
	}
	km.keys = newKeys
}

// ReviveKey manually restores a dead key to the rotation.
// Use this for manual circuit breaker reset or health check recovery.
//
// Thread-safe: uses write locks on both deadKeys map and keys slice.
func (km *KeyManager) ReviveKey(key string) {
	if key == "" {
		return
	}

	// Check if this key is in our original set
	if _, exists := km.originalKeys[key]; !exists {
		return // Not a managed key
	}

	// Remove from dead keys map
	km.deadMu.Lock()
	_, wasDead := km.deadKeys[key]
	delete(km.deadKeys, key)
	km.deadMu.Unlock()

	if !wasDead {
		return // Key wasn't dead, nothing to do
	}

	// Add back to active keys slice
	km.mu.Lock()
	defer km.mu.Unlock()

	// Check if already present (shouldn't happen, but safety first)
	for _, k := range km.keys {
		if k == key {
			return // Already active
		}
	}

	km.keys = append(km.keys, key)
}

// reviveExpiredKeys checks all dead keys and revives those past their cooldown.
// This is called internally by GetNextKey for automatic recovery.
func (km *KeyManager) reviveExpiredKeys() {
	if km.cooldown == 0 {
		return // Auto-revival disabled
	}

	now := time.Now()
	var keysToRevive []string

	km.deadMu.RLock()
	for key, deadTime := range km.deadKeys {
		if now.Sub(deadTime) >= km.cooldown {
			keysToRevive = append(keysToRevive, key)
		}
	}
	km.deadMu.RUnlock()

	// Revive expired keys
	for _, key := range keysToRevive {
		km.ReviveKey(key)
	}
}

// ActiveKeyCount returns the number of keys currently in rotation.
func (km *KeyManager) ActiveKeyCount() int {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return len(km.keys)
}

// DeadKeyCount returns the number of keys currently marked as dead.
func (km *KeyManager) DeadKeyCount() int {
	km.deadMu.RLock()
	defer km.deadMu.RUnlock()
	return len(km.deadKeys)
}

// TotalKeyCount returns the total number of managed keys (active + dead).
func (km *KeyManager) TotalKeyCount() int {
	return len(km.originalKeys)
}

// GetActiveKeys returns a copy of all currently active keys.
// Useful for debugging and monitoring.
func (km *KeyManager) GetActiveKeys() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	result := make([]string, len(km.keys))
	copy(result, km.keys)
	return result
}

// GetDeadKeys returns a copy of all currently dead keys with their death timestamps.
// Useful for debugging and monitoring circuit breaker state.
func (km *KeyManager) GetDeadKeys() map[string]time.Time {
	km.deadMu.RLock()
	defer km.deadMu.RUnlock()

	result := make(map[string]time.Time, len(km.deadKeys))
	for k, v := range km.deadKeys {
		result[k] = v
	}
	return result
}

// IsKeyDead checks if a specific key is currently marked as dead.
func (km *KeyManager) IsKeyDead(key string) bool {
	km.deadMu.RLock()
	defer km.deadMu.RUnlock()
	_, isDead := km.deadKeys[key]
	return isDead
}
