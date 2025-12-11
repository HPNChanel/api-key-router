package domain

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNoKeysAvailable = errors.New("no keys available")

// KeyManager manages a pool of API keys with round-robin rotation and
// circuit-breaker style dead key tracking.
type KeyManager struct {
	keys         []string
	deadKeys     map[string]time.Time
	originalKeys map[string]struct{}
	index        int64
	cooldown     time.Duration
	mu           sync.RWMutex
	deadMu       sync.RWMutex
}

// NewKeyManager returns a KeyManager with the given keys. Dead keys auto-revive
// after cooldown; pass 0 to disable auto-revival.
func NewKeyManager(keys []string, cooldown time.Duration) *KeyManager {
	km := &KeyManager{
		keys:         make([]string, 0, len(keys)),
		deadKeys:     make(map[string]time.Time),
		originalKeys: make(map[string]struct{}),
		cooldown:     cooldown,
	}

	seen := make(map[string]struct{})
	for _, k := range keys {
		if k == "" {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		km.keys = append(km.keys, k)
		km.originalKeys[k] = struct{}{}
	}

	return km
}

// GetNextKey returns the next key via atomic round-robin. Revives expired dead
// keys before selection.
func (km *KeyManager) GetNextKey() (string, error) {
	km.reviveExpired()

	km.mu.RLock()
	n := len(km.keys)
	if n == 0 {
		km.mu.RUnlock()
		return "", ErrNoKeysAvailable
	}

	// atomic increment; returns new value, so use (new-1) % n
	idx := int((atomic.AddInt64(&km.index, 1) - 1) % int64(n))
	key := km.keys[idx]
	km.mu.RUnlock()

	return key, nil
}

// MarkAsDead removes a key from rotation for the cooldown period.
func (km *KeyManager) MarkAsDead(key string) {
	if key == "" {
		return
	}
	if _, ok := km.originalKeys[key]; !ok {
		return
	}

	km.deadMu.Lock()
	km.deadKeys[key] = time.Now()
	km.deadMu.Unlock()

	km.mu.Lock()
	filtered := km.keys[:0]
	for _, k := range km.keys {
		if k != key {
			filtered = append(filtered, k)
		}
	}
	km.keys = filtered
	km.mu.Unlock()
}

// ReviveKey manually restores a dead key to rotation.
func (km *KeyManager) ReviveKey(key string) {
	if key == "" {
		return
	}
	if _, ok := km.originalKeys[key]; !ok {
		return
	}

	km.deadMu.Lock()
	_, wasDead := km.deadKeys[key]
	delete(km.deadKeys, key)
	km.deadMu.Unlock()

	if !wasDead {
		return
	}

	km.mu.Lock()
	for _, k := range km.keys {
		if k == key {
			km.mu.Unlock()
			return
		}
	}
	km.keys = append(km.keys, key)
	km.mu.Unlock()
}

func (km *KeyManager) reviveExpired() {
	if km.cooldown == 0 {
		return
	}

	now := time.Now()
	var revive []string

	km.deadMu.RLock()
	for k, t := range km.deadKeys {
		if now.Sub(t) >= km.cooldown {
			revive = append(revive, k)
		}
	}
	km.deadMu.RUnlock()

	for _, k := range revive {
		km.ReviveKey(k)
	}
}

// ActiveKeyCount returns keys currently in rotation.
func (km *KeyManager) ActiveKeyCount() int {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return len(km.keys)
}

// DeadKeyCount returns keys currently marked dead.
func (km *KeyManager) DeadKeyCount() int {
	km.deadMu.RLock()
	defer km.deadMu.RUnlock()
	return len(km.deadKeys)
}

// TotalKeyCount returns total managed keys (active + dead).
func (km *KeyManager) TotalKeyCount() int {
	return len(km.originalKeys)
}

// GetActiveKeys returns a copy of currently active keys.
func (km *KeyManager) GetActiveKeys() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	res := make([]string, len(km.keys))
	copy(res, km.keys)
	return res
}

// GetDeadKeys returns a copy of dead keys with their timestamps.
func (km *KeyManager) GetDeadKeys() map[string]time.Time {
	km.deadMu.RLock()
	defer km.deadMu.RUnlock()

	res := make(map[string]time.Time, len(km.deadKeys))
	for k, v := range km.deadKeys {
		res[k] = v
	}
	return res
}

// IsKeyDead reports whether a key is currently marked dead.
func (km *KeyManager) IsKeyDead(key string) bool {
	km.deadMu.RLock()
	defer km.deadMu.RUnlock()
	_, dead := km.deadKeys[key]
	return dead
}
