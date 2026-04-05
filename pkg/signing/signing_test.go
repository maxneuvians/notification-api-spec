package signing_test

import (
	"testing"

	"github.com/maxneuvians/notification-api-spec/pkg/signing"
)

func TestSignUnsignRoundTrip(t *testing.T) {
	token, err := signing.Sign("hello", "topsecret", "notify")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	payload, err := signing.Unsign(token, []string{"topsecret"}, "notify")
	if err != nil {
		t.Fatalf("unsign: %v", err)
	}

	if payload != "hello" {
		t.Fatalf("payload = %q, want hello", payload)
	}
}

func TestUnsignFailsWithWrongSecret(t *testing.T) {
	token, err := signing.Sign("hello", "topsecret", "notify")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	if _, err := signing.Unsign(token, []string{"wrong-secret"}, "notify"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadsReturnsTypedMap(t *testing.T) {
	token, err := signing.Dumps(map[string]any{"user_id": "abc"}, "topsecret", "notify")
	if err != nil {
		t.Fatalf("dumps: %v", err)
	}

	data, err := signing.Loads(token, []string{"topsecret"}, "notify")
	if err != nil {
		t.Fatalf("loads: %v", err)
	}

	if data["user_id"] != "abc" {
		t.Fatalf("user_id = %#v, want abc", data["user_id"])
	}
}

func TestUnsignPythonFixture(t *testing.T) {
	payload, err := signing.Unsign("fixture-payload.ZVPxAA.FFIg803Kuri4l4-0voYHZSzTISZ5GXJJhdi6N4ikZ5c", []string{"fixture-sign-secret"}, "fixture-salt")
	if err != nil {
		t.Fatalf("unsign fixture: %v", err)
	}

	if payload != "fixture-payload" {
		t.Fatalf("payload = %q, want fixture-payload", payload)
	}
}

func TestLoadsPythonFixture(t *testing.T) {
	data, err := signing.Loads("eyJ1c2VyX2lkIjoiYWJjIn0.ZVPxAA.8nHHBJuOoerch1Jr3F_PcmiD5Hur5N7gT9LpbXLCE_s", []string{"fixture-sign-secret"}, "fixture-salt")
	if err != nil {
		t.Fatalf("loads fixture: %v", err)
	}

	if data["user_id"] != "abc" {
		t.Fatalf("user_id = %#v, want abc", data["user_id"])
	}
}
