package crypto_test

import (
	"testing"

	appcrypto "github.com/maxneuvians/notification-api-spec/pkg/crypto"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	ciphertext, err := appcrypto.Encrypt("hello", "mysecret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	plaintext, err := appcrypto.Decrypt(ciphertext, []string{"mysecret"})
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if plaintext != "hello" {
		t.Fatalf("plaintext = %q, want hello", plaintext)
	}
}

func TestDecryptSucceedsWithSecondKey(t *testing.T) {
	ciphertext, err := appcrypto.Encrypt("rotated", "old-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	plaintext, err := appcrypto.Decrypt(ciphertext, []string{"new-secret", "old-secret"})
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if plaintext != "rotated" {
		t.Fatalf("plaintext = %q, want rotated", plaintext)
	}
}

func TestDecryptFailsWithoutMatchingKey(t *testing.T) {
	ciphertext, err := appcrypto.Encrypt("hello", "mysecret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if _, err := appcrypto.Decrypt(ciphertext, []string{"other-secret"}); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDecryptPythonFixture(t *testing.T) {
	plaintext, err := appcrypto.Decrypt("fixture-value.p_O2JUjUmWLGKRut-V8XBd1ykQI", []string{"fixture-secret"})
	if err != nil {
		t.Fatalf("decrypt fixture: %v", err)
	}

	if plaintext != "fixture-value" {
		t.Fatalf("plaintext = %q, want fixture-value", plaintext)
	}
}
