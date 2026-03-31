package auth

import (
	"bytes"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	key := DeriveKey("test-secret")
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
	// Same input produces same output
	key2 := DeriveKey("test-secret")
	if !bytes.Equal(key, key2) {
		t.Error("DeriveKey should be deterministic")
	}
	// Different input produces different output
	key3 := DeriveKey("other-secret")
	if bytes.Equal(key, key3) {
		t.Error("different secrets should produce different keys")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := DeriveKey("my-jwt-secret")
	plaintext := []byte("hello world sensitive data")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("decrypted %q != original %q", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := DeriveKey("secret-1")
	key2 := DeriveKey("secret-2")

	ciphertext, err := Encrypt([]byte("data"), key1)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}

	_, err = Decrypt(ciphertext, key2)
	if err == nil {
		t.Error("Decrypt with wrong key should fail")
	}
}

func TestDecryptTooShort(t *testing.T) {
	key := DeriveKey("secret")
	_, err := Decrypt([]byte("short"), key)
	if err == nil {
		t.Error("Decrypt with too-short ciphertext should fail")
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key := DeriveKey("secret")
	plaintext := []byte("same data")

	ct1, _ := Encrypt(plaintext, key)
	ct2, _ := Encrypt(plaintext, key)

	if bytes.Equal(ct1, ct2) {
		t.Error("two encryptions of the same data should produce different ciphertexts (random nonce)")
	}
}
