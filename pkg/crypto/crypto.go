package crypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const signerSalt = "itsdangerous.Signer"

func Encrypt(plaintext, secret string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("secret is required")
	}

	payload := []byte(plaintext)
	sig := hmacSignature(deriveKey(secret), payload)
	return plaintext + "." + base64Encode(sig), nil
}

func Decrypt(ciphertext string, secrets []string) (string, error) {
	payload, sig, ok := strings.Cut(ciphertext, ".")
	if !ok {
		return "", errors.New("invalid ciphertext format")
	}

	decodedSig, err := base64Decode(sig)
	if err != nil {
		return "", fmt.Errorf("decode signature: %w", err)
	}

	for _, secret := range secrets {
		if strings.TrimSpace(secret) == "" {
			continue
		}

		expected := hmacSignature(deriveKey(secret), []byte(payload))
		if hmac.Equal(decodedSig, expected) {
			return payload, nil
		}
	}

	return "", errors.New("signature does not match any configured secret")
}

func deriveKey(secret string) []byte {
	digest := sha1.Sum([]byte(signerSalt + "signer" + secret))
	return digest[:]
}

func hmacSignature(key, value []byte) []byte {
	mac := hmac.New(sha1.New, key)
	mac.Write(value)
	return mac.Sum(nil)
}

func base64Encode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func base64Decode(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
