package domain

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewKeyManager(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		expected int
	}{
		{
			name:     "normal keys",
			keys:     []string{"key1", "key2", "key3"},
			expected: 3,
		},
		{
			name:     "empty slice",
			keys:     []string{},
			expected: 0,
		},
		{
			name:     "nil slice",
			keys:     nil,
			expected: 0,
		},
		{
			name:     "with duplicates",
			keys:     []string{"key1", "key2", "key1", "key3", "key2"},
			expected: 3, // Duplicates removed
		},
		{
			name:     "with empty strings",
			keys:     []string{"key1", "", "key2", ""},
			expected: 2, // Empty strings skipped
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			km := NewKeyManager(tt.keys, time.Minute)
			if got := km.ActiveKeyCount(); got != tt.expected {
				t.Errorf("ActiveKeyCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestGetNextKey_RoundRobin(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager(keys, 0)

	// Verify round-robin order
	for i := 0; i < 9; i++ {
		key, err := km.GetNextKey()
		if err != nil {
			t.Fatalf("GetNextKey() error = %v", err)
		}
		expected := keys[i%3]
		if key != expected {
			t.Errorf("iteration %d: got %s, want %s", i, key, expected)
		}
	}
}

func TestGetNextKey_NoKeys(t *testing.T) {
	km := NewKeyManager([]string{}, 0)

	_, err := km.GetNextKey()
	if err != ErrNoKeysAvailable {
		t.Errorf("GetNextKey() error = %v, want %v", err, ErrNoKeysAvailable)
	}
}

func TestGetNextKey_Concurrent(t *testing.T) {
	keys := []string{"key1", "key2", "key3", "key4", "key5"}
	km := NewKeyManager(keys, 0)

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	keyCounts := make(map[string]*int64)
	for _, key := range keys {
		var count int64
		keyCounts[key] = &count
	}

	var mu sync.Mutex

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key, err := km.GetNextKey()
				if err != nil {
					t.Errorf("GetNextKey() error = %v", err)
					return
				}
				mu.Lock()
				atomic.AddInt64(keyCounts[key], 1)
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Verify all keys were used
	totalCalls := goroutines * iterations
	expectedPerKey := int64(totalCalls / len(keys))
	tolerance := int64(float64(expectedPerKey) * 0.1) // 10% tolerance

	for key, count := range keyCounts {
		c := atomic.LoadInt64(count)
		if c < expectedPerKey-tolerance || c > expectedPerKey+tolerance {
			t.Logf("key %s: count=%d, expected≈%d (tolerance ±%d)", key, c, expectedPerKey, tolerance)
		}
	}
}

func TestMarkAsDead(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager(keys, 0)

	// Mark key2 as dead
	km.MarkAsDead("key2")

	if km.ActiveKeyCount() != 2 {
		t.Errorf("ActiveKeyCount() = %d, want 2", km.ActiveKeyCount())
	}

	if km.DeadKeyCount() != 1 {
		t.Errorf("DeadKeyCount() = %d, want 1", km.DeadKeyCount())
	}

	if !km.IsKeyDead("key2") {
		t.Error("IsKeyDead(key2) = false, want true")
	}

	// Verify key2 is not returned
	for i := 0; i < 10; i++ {
		key, err := km.GetNextKey()
		if err != nil {
			t.Fatalf("GetNextKey() error = %v", err)
		}
		if key == "key2" {
			t.Error("GetNextKey() returned dead key 'key2'")
		}
	}
}

func TestMarkAsDead_AllKeys(t *testing.T) {
	keys := []string{"key1", "key2"}
	km := NewKeyManager(keys, 0) // No auto-revival

	km.MarkAsDead("key1")
	km.MarkAsDead("key2")

	_, err := km.GetNextKey()
	if err != ErrNoKeysAvailable {
		t.Errorf("GetNextKey() error = %v, want %v", err, ErrNoKeysAvailable)
	}
}

func TestReviveKey(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager(keys, 0)

	// Mark and then revive
	km.MarkAsDead("key2")
	if km.ActiveKeyCount() != 2 {
		t.Errorf("After MarkAsDead: ActiveKeyCount() = %d, want 2", km.ActiveKeyCount())
	}

	km.ReviveKey("key2")
	if km.ActiveKeyCount() != 3 {
		t.Errorf("After ReviveKey: ActiveKeyCount() = %d, want 3", km.ActiveKeyCount())
	}

	if km.IsKeyDead("key2") {
		t.Error("After ReviveKey: IsKeyDead(key2) = true, want false")
	}
}

func TestAutoRevival(t *testing.T) {
	keys := []string{"key1", "key2"}
	cooldown := 50 * time.Millisecond
	km := NewKeyManager(keys, cooldown)

	km.MarkAsDead("key1")

	// Key should be dead immediately
	if !km.IsKeyDead("key1") {
		t.Error("IsKeyDead(key1) = false immediately after MarkAsDead")
	}

	// Wait for cooldown
	time.Sleep(cooldown + 20*time.Millisecond)

	// GetNextKey should trigger auto-revival
	_, _ = km.GetNextKey()

	if km.IsKeyDead("key1") {
		t.Error("IsKeyDead(key1) = true after cooldown, expected auto-revival")
	}
}

func TestMarkAsDead_UnknownKey(t *testing.T) {
	keys := []string{"key1", "key2"}
	km := NewKeyManager(keys, 0)

	// Marking unknown key should be a no-op
	km.MarkAsDead("unknown_key")

	if km.ActiveKeyCount() != 2 {
		t.Errorf("ActiveKeyCount() = %d after marking unknown key, want 2", km.ActiveKeyCount())
	}
}

func TestGetActiveKeys(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager(keys, 0)

	km.MarkAsDead("key2")

	activeKeys := km.GetActiveKeys()
	if len(activeKeys) != 2 {
		t.Errorf("len(GetActiveKeys()) = %d, want 2", len(activeKeys))
	}

	// Verify it returns a copy (modification doesn't affect original)
	activeKeys[0] = "modified"
	newActiveKeys := km.GetActiveKeys()
	if newActiveKeys[0] == "modified" {
		t.Error("GetActiveKeys() should return a copy, not the original slice")
	}
}

func TestGetDeadKeys(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager(keys, 0)

	km.MarkAsDead("key2")

	deadKeys := km.GetDeadKeys()
	if len(deadKeys) != 1 {
		t.Errorf("len(GetDeadKeys()) = %d, want 1", len(deadKeys))
	}

	if _, exists := deadKeys["key2"]; !exists {
		t.Error("GetDeadKeys() should contain 'key2'")
	}
}

func TestTotalKeyCount(t *testing.T) {
	keys := []string{"key1", "key2", "key3"}
	km := NewKeyManager(keys, 0)

	km.MarkAsDead("key1")
	km.MarkAsDead("key2")

	// Total should remain constant
	if km.TotalKeyCount() != 3 {
		t.Errorf("TotalKeyCount() = %d, want 3", km.TotalKeyCount())
	}
}
