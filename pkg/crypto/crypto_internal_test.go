package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptRejectsEmptySecret(t *testing.T) {
	if _, err := Encrypt("hello", ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDecryptRejectsMalformedCiphertext(t *testing.T) {
	if _, err := Decrypt("invalid", []string{"secret"}); err == nil {
		t.Fatal("expected invalid ciphertext format error")
	}

	if _, err := Decrypt("payload.***", []string{"secret"}); err == nil {
		t.Fatal("expected base64 decode error")
	}
}

func TestHelpers(t *testing.T) {
	encoded := base64Encode([]byte("hello"))
	decoded, err := base64Decode(encoded)
	if err != nil {
		t.Fatalf("base64Decode() error = %v, want nil", err)
	}
	if !bytes.Equal(decoded, []byte("hello")) {
		t.Fatalf("decoded = %q, want hello", decoded)
	}

	key := deriveKey("secret")
	sig := hmacSignature(key, []byte("payload"))
	if len(sig) == 0 {
		t.Fatal("expected non-empty signature")
	}
}
