package signing

import (
	"compress/zlib"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

func Sign(payload, secret, salt string) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("secret is required")
	}

	return signWithTimestamp(payload, secret, salt, time.Now().Unix())
}

func Unsign(token string, secrets []string, salt string) (string, error) {
	lastSep := strings.LastIndex(token, ".")
	if lastSep == -1 {
		return "", errors.New("invalid signed token")
	}

	payloadWithTimestamp := token[:lastSep]
	signature := token[lastSep+1:]

	timestampSep := strings.LastIndex(payloadWithTimestamp, ".")
	if timestampSep == -1 {
		return "", errors.New("timestamp missing")
	}

	payload := payloadWithTimestamp[:timestampSep]
	for _, secret := range secrets {
		if strings.TrimSpace(secret) == "" {
			continue
		}

		expected := base64Encode(hmacSignature(deriveKey(secret, salt), []byte(payloadWithTimestamp)))
		if hmac.Equal([]byte(signature), []byte(expected)) {
			if _, err := base64Decode(payloadWithTimestamp[timestampSep+1:]); err != nil {
				return "", fmt.Errorf("malformed timestamp: %w", err)
			}
			return payload, nil
		}
	}

	return "", errors.New("signature does not match any configured secret")
}

func Dumps(data any, secret, salt string) (string, error) {
	serialized, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	payload, err := dumpPayload(serialized)
	if err != nil {
		return "", err
	}

	return Sign(payload, secret, salt)
}

func Loads(token string, secrets []string, salt string) (map[string]any, error) {
	payload, err := Unsign(token, secrets, salt)
	if err != nil {
		return nil, err
	}

	jsonPayload, err := loadPayload(payload)
	if err != nil {
		return nil, err
	}

	decoded := make(map[string]any)
	if err := json.Unmarshal(jsonPayload, &decoded); err != nil {
		return nil, err
	}

	return decoded, nil
}

func signWithTimestamp(payload, secret, salt string, timestamp int64) (string, error) {
	timestampBytes := intToBytes(timestamp)
	payloadWithTimestamp := payload + "." + base64Encode(timestampBytes)
	signature := base64Encode(hmacSignature(deriveKey(secret, salt), []byte(payloadWithTimestamp)))
	return payloadWithTimestamp + "." + signature, nil
}

func deriveKey(secret, salt string) []byte {
	digest := sha256.Sum256([]byte(salt + "signer" + secret))
	return digest[:]
}

func hmacSignature(key, value []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(value)
	return mac.Sum(nil)
}

func dumpPayload(jsonPayload []byte) (string, error) {
	compressed, err := compress(jsonPayload)
	if err != nil {
		return "", err
	}

	if len(compressed) < len(jsonPayload)-1 {
		return "." + base64Encode(compressed), nil
	}

	return base64Encode(jsonPayload), nil
}

func loadPayload(payload string) ([]byte, error) {
	compressed := strings.HasPrefix(payload, ".")
	if compressed {
		payload = payload[1:]
	}

	decoded, err := base64Decode(payload)
	if err != nil {
		return nil, err
	}

	if !compressed {
		return decoded, nil
	}

	reader, err := zlib.NewReader(strings.NewReader(string(decoded)))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func compress(data []byte) ([]byte, error) {
	var builder strings.Builder
	writer := zlib.NewWriter(&builder)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return []byte(builder.String()), nil
}

func intToBytes(value int64) []byte {
	if value == 0 {
		return []byte{}
	}

	buf := make([]byte, 8)
	for i := len(buf) - 1; i >= 0; i-- {
		buf[i] = byte(value)
		value >>= 8
	}

	index := 0
	for index < len(buf) && buf[index] == 0 {
		index++
	}
	return buf[index:]
}

func base64Encode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}

func base64Decode(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}
