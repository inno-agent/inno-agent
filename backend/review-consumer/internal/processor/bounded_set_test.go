package processor

import (
	"fmt"
	"testing"
)

func TestBoundedSet_EvictsOldestWhenFull(t *testing.T) {
	const cap = 5
	s := newBoundedSet(cap)

	// Fill to capacity.
	for i := range cap {
		s.add(fmt.Sprintf("key%d", i), cap)
	}
	if len(s.m) != cap {
		t.Fatalf("expected %d entries, got %d", cap, len(s.m))
	}

	// key0 is the oldest; adding a new entry should evict it.
	s.add("key_new", cap)
	if s.has("key0") {
		t.Error("key0 should have been evicted")
	}
	if !s.has("key_new") {
		t.Error("key_new should be present")
	}
	if len(s.m) != cap {
		t.Fatalf("expected %d entries after eviction, got %d", cap, len(s.m))
	}
}

func TestBoundedSet_DuplicateAddDoesNotGrow(t *testing.T) {
	const cap = 3
	s := newBoundedSet(cap)
	s.add("a", cap)
	s.add("a", cap)
	s.add("a", cap)

	if len(s.m) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(s.m))
	}
}

func TestBoundedSet_HasReturnsFalseForMissing(t *testing.T) {
	s := newBoundedSet(10)
	if s.has("missing") {
		t.Error("expected false for absent key")
	}
}
