package secretbox_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/secretbox"
)

func makeKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("rand: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestSecretBox_RoundTrip(t *testing.T) {
	sb, err := secretbox.NewFromBase64Key(makeKey(t))
	if err != nil {
		t.Fatalf("NewFromBase64Key: %v", err)
	}

	plain := []byte("super-secret-refresh-token-value")
	ct, nonce, err := sb.Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	got, err := sb.Decrypt(ct, nonce)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(plain, got) {
		t.Fatalf("round-trip mismatch: got %q, want %q", got, plain)
	}
}

func TestSecretBox_DifferentNonceEachCall(t *testing.T) {
	sb, err := secretbox.NewFromBase64Key(makeKey(t))
	if err != nil {
		t.Fatalf("NewFromBase64Key: %v", err)
	}

	plain := []byte("hello")
	_, n1, _ := sb.Encrypt(plain)
	_, n2, _ := sb.Encrypt(plain)

	if bytes.Equal(n1, n2) {
		t.Fatal("expected different nonces on each Encrypt call")
	}
}

func TestSecretBox_TamperedCiphertext(t *testing.T) {
	sb, err := secretbox.NewFromBase64Key(makeKey(t))
	if err != nil {
		t.Fatalf("NewFromBase64Key: %v", err)
	}

	ct, nonce, _ := sb.Encrypt([]byte("data"))
	ct[0] ^= 0xff // tamper

	_, err = sb.Decrypt(ct, nonce)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}

func TestSecretBox_InvalidKeyLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err := secretbox.NewFromBase64Key(short)
	if err == nil {
		t.Fatal("expected error for non-32-byte key")
	}
}
