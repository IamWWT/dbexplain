package query

import (
	"sync"
	"testing"
)

func TestQueryLock_BasicLockUnlock(t *testing.T) {
	ql := NewQueryLock()

	if !ql.Lock("label1") {
		t.Fatal("first Lock should succeed")
	}

	// Same label should fail while locked
	if ql.Lock("label1") {
		t.Fatal("second Lock on same label should fail")
	}

	ql.Unlock("label1")

	// After unlock, Lock should succeed again
	if !ql.Lock("label1") {
		t.Fatal("Lock after Unlock should succeed")
	}
	ql.Unlock("label1")
}

func TestQueryLock_DifferentLabelsIndependent(t *testing.T) {
	ql := NewQueryLock()

	if !ql.Lock("labelA") {
		t.Fatal("Lock labelA should succeed")
	}
	if !ql.Lock("labelB") {
		t.Fatal("Lock labelB should succeed (different label)")
	}

	// Both still locked
	if ql.Lock("labelA") {
		t.Fatal("re-Lock labelA should fail")
	}
	if ql.Lock("labelB") {
		t.Fatal("re-Lock labelB should fail")
	}

	ql.Unlock("labelA")
	ql.Unlock("labelB")
}

func TestQueryLock_ConcurrentSameLabel(t *testing.T) {
	ql := NewQueryLock()
	var wg sync.WaitGroup

	// Goroutine 1 acquires and holds the lock
	wg.Add(1)
	var locked2 bool
	ready := make(chan struct{})

	go func() {
		defer wg.Done()
		// Wait for main to acquire first
		<-ready
		locked2 = ql.Lock("concurrent")
	}()

	// Main acquires lock first
	if !ql.Lock("concurrent") {
		t.Fatal("first Lock should succeed")
	}

	// Signal goroutine to try
	close(ready)
	wg.Wait()

	if locked2 {
		t.Fatal("concurrent Lock on same label should fail")
	}

	ql.Unlock("concurrent")

	// Now it should succeed
	if !ql.Lock("concurrent") {
		t.Fatal("Lock after goroutine finished should succeed")
	}
	ql.Unlock("concurrent")
}

func TestQueryLock_ConcurrentDifferentLabels(t *testing.T) {
	ql := NewQueryLock()
	var wg sync.WaitGroup
	results := make([]bool, 3)

	// Acquire label X in main
	if !ql.Lock("X") {
		t.Fatal("Lock X should succeed")
	}

	// Try label Y from goroutines concurrently
	for i := 0; i < 3; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = ql.Lock("Y")
		}()
	}
	wg.Wait()

	// Exactly one goroutine should acquire Y
	acquired := 0
	for _, r := range results {
		if r {
			acquired++
		}
	}
	if acquired != 1 {
		t.Errorf("expected exactly 1 goroutine to acquire label Y, got %d", acquired)
	}

	ql.Unlock("X")
	ql.Unlock("Y")
}

func TestQueryLock_UnlockMissingLabel(t *testing.T) {
	ql := NewQueryLock()

	// Unlock a label that was never locked — should not panic
	ql.Unlock("nonexistent")
}

func TestQueryLock_UnlockTwice(t *testing.T) {
	// NOTE: double-unlock of sync.Mutex causes a fatal error (not a recoverable
	// panic), so we can't test that directly. Instead, verify that Lock/Unlock
	// works correctly and that the lock is properly released.
	ql := NewQueryLock()

	if !ql.Lock("twice") {
		t.Fatal("Lock should succeed")
	}
	ql.Unlock("twice")

	// After proper unlock, re-lock should succeed
	if !ql.Lock("twice") {
		t.Fatal("re-Lock after Unlock should succeed")
	}
	ql.Unlock("twice")
}

func TestQueryLock_ManyLabels(t *testing.T) {
	ql := NewQueryLock()
	const N = 100

	// Lock many unique labels
	for i := 0; i < N; i++ {
		label := "label" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		if !ql.Lock(label) {
			t.Fatalf("Lock %s should succeed", label)
		}
	}
	// Verify all still locked
	for i := 0; i < N; i++ {
		label := "label" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		if ql.Lock(label) {
			t.Fatalf("second Lock on %s should fail", label)
		}
	}
	// Unlock all
	for i := 0; i < N; i++ {
		label := "label" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		ql.Unlock(label)
	}
}

func TestQueryLock_ConcurrentLockUnlockCycle(t *testing.T) {
	ql := NewQueryLock()
	var wg sync.WaitGroup
	const cycles = 50
	var ok1, ok2 int

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < cycles; i++ {
			if ql.Lock("cycle") {
				ok1++
				ql.Unlock("cycle")
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < cycles; i++ {
			if ql.Lock("cycle") {
				ok2++
				ql.Unlock("cycle")
			}
		}
	}()

	wg.Wait()

	// At least some lock acquisitions should succeed (not livelock)
	if ok1+ok2 == 0 {
		t.Fatal("no lock acquisitions succeeded — possible livelock")
	}
	t.Logf("goroutine 1 acquired lock %d/%d times, goroutine 2: %d/%d", ok1, cycles, ok2, cycles)
}

func TestNewQueryLock(t *testing.T) {
	ql := NewQueryLock()
	if ql == nil {
		t.Fatal("NewQueryLock should not return nil")
	}
	if ql.locks == nil {
		t.Fatal("locks map should be initialized")
	}
}

func TestQueryLock_DoubleLockUnlockSequence(t *testing.T) {
	ql := NewQueryLock()

	// Lock -> Unlock -> Lock -> Unlock (normal usage pattern)
	for i := 0; i < 5; i++ {
		if !ql.Lock("seq") {
			t.Fatalf("iteration %d: Lock should succeed", i)
		}
		ql.Unlock("seq")
	}
}
