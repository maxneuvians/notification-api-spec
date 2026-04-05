package signing

import (
	"bytes"
	"testing"
)

func TestSignRejectsEmptySecret(t *testing.T) {
	if _, err := Sign("payload", "", "salt"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUnsignRejectsMalformedTokens(t *testing.T) {
	if _, err := Unsign("invalid", []string{"secret"}, "salt"); err == nil {
		t.Fatal("expected invalid signed token error")
	}

	if _, err := Unsign("payload.signature", []string{"secret"}, "salt"); err == nil {
		t.Fatal("expected timestamp missing error")
	}

	payloadWithTimestamp := "payload.***"
	signature := base64Encode(hmacSignature(deriveKey("secret", "salt"), []byte(payloadWithTimestamp)))
	if _, err := Unsign(payloadWithTimestamp+"."+signature, []string{"secret"}, "salt"); err == nil {
		t.Fatal("expected malformed timestamp error")
	}
}

func TestDumpAndLoadPayload(t *testing.T) {
	smallPayload := []byte(`{"ok":true}`)
	dumped, err := dumpPayload(smallPayload)
	if err != nil {
		t.Fatalf("dumpPayload(small) error = %v, want nil", err)
	}
	if dumped == "" || dumped[0] == '.' {
		t.Fatalf("dumpPayload(small) = %q, want uncompressed base64 payload", dumped)
	}
	loaded, err := loadPayload(dumped)
	if err != nil {
		t.Fatalf("loadPayload(small) error = %v, want nil", err)
	}
	if !bytes.Equal(loaded, smallPayload) {
		t.Fatalf("loaded small payload = %q, want %q", loaded, smallPayload)
	}

	largePayload := bytes.Repeat([]byte("compress-me"), 32)
	dumped, err = dumpPayload(largePayload)
	if err != nil {
		t.Fatalf("dumpPayload(large) error = %v, want nil", err)
	}
	if dumped == "" || dumped[0] != '.' {
		t.Fatalf("dumpPayload(large) = %q, want compressed payload", dumped)
	}
	loaded, err = loadPayload(dumped)
	if err != nil {
		t.Fatalf("loadPayload(large) error = %v, want nil", err)
	}
	if !bytes.Equal(loaded, largePayload) {
		t.Fatalf("loaded large payload = %q, want %q", loaded, largePayload)
	}
}

func TestDumpsAndLoadsErrorPaths(t *testing.T) {
	if _, err := Dumps(map[string]any{"bad": make(chan int)}, "secret", "salt"); err == nil {
		t.Fatal("expected json marshal error")
	}

	if _, err := Loads("invalid", []string{"secret"}, "salt"); err == nil {
		t.Fatal("expected invalid token error")
	}

	payload, err := dumpPayload([]byte(`"plain-string"`))
	if err != nil {
		t.Fatalf("dumpPayload() error = %v, want nil", err)
	}
	token, err := signWithTimestamp(payload, "secret", "salt", 1)
	if err != nil {
		t.Fatalf("signWithTimestamp() error = %v, want nil", err)
	}
	if _, err := Loads(token, []string{"secret"}, "salt"); err == nil {
		t.Fatal("expected json object decode error")
	}
}

func TestHelpers(t *testing.T) {
	if got := intToBytes(0); len(got) != 0 {
		t.Fatalf("intToBytes(0) = %#v, want empty slice", got)
	}
	if got := intToBytes(258); !bytes.Equal(got, []byte{1, 2}) {
		t.Fatalf("intToBytes(258) = %#v, want [1 2]", got)
	}

	encoded := base64Encode([]byte("hello"))
	decoded, err := base64Decode(encoded)
	if err != nil {
		t.Fatalf("base64Decode() error = %v, want nil", err)
	}
	if !bytes.Equal(decoded, []byte("hello")) {
		t.Fatalf("decoded = %q, want hello", decoded)
	}

	if _, err := loadPayload("***"); err == nil {
		t.Fatal("expected invalid payload error")
	}
	if _, err := loadPayload("." + base64Encode([]byte("not-zlib"))); err == nil {
		t.Fatal("expected invalid zlib payload error")
	}
}
