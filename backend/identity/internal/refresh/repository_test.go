package refresh

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestMint(t *testing.T) {
	plaintext, hash, err := Mint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plaintext == "" {
		t.Fatal("expected non-empty plaintext")
	}
	if len(hash) != sha256.Size {
		t.Fatalf("expected hash of size %d, got %d", sha256.Size, len(hash))
	}

	// Verify hash matches plaintext
	expected := sha256.Sum256([]byte(plaintext))
	if !bytes.Equal(hash, expected[:]) {
		t.Fatal("hash does not match plaintext")
	}
}

func TestMint_Unique(t *testing.T) {
	pt1, _, _ := Mint()
	pt2, _, _ := Mint()
	if pt1 == pt2 {
		t.Fatal("expected unique tokens")
	}
}

func TestHash(t *testing.T) {
	plaintext := "test-refresh-token"
	hash := Hash(plaintext)

	expected := sha256.Sum256([]byte(plaintext))
	if !bytes.Equal(hash, expected[:]) {
		t.Fatal("hash does not match expected")
	}
}

func TestHash_Deterministic(t *testing.T) {
	h1 := Hash("same-input")
	h2 := Hash("same-input")
	if !bytes.Equal(h1, h2) {
		t.Fatal("expected deterministic hash")
	}
}
