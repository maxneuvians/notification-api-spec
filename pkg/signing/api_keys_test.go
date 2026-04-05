package signing

import "testing"

func TestSignAPIKeyToken(t *testing.T) {
	token := "123e4567-e89b-12d3-a456-426614174000"
	signed, err := SignAPIKeyToken(token, "topsecret")
	if err != nil {
		t.Fatalf("SignAPIKeyToken() error = %v", err)
	}
	if signed == "" {
		t.Fatal("expected signed token")
	}
	if signed == token {
		t.Fatal("signed token should differ from plaintext token")
	}
}

func TestSignAPIKeyTokenWithAllKeys(t *testing.T) {
	token := "123e4567-e89b-12d3-a456-426614174000"
	variants, err := SignAPIKeyTokenWithAllKeys(token, []string{"first", "second", "first"})
	if err != nil {
		t.Fatalf("SignAPIKeyTokenWithAllKeys() error = %v", err)
	}
	if len(variants) != 2 {
		t.Fatalf("variants length = %d, want 2", len(variants))
	}

	first, err := SignAPIKeyToken(token, "first")
	if err != nil {
		t.Fatalf("SignAPIKeyToken() error = %v", err)
	}
	if variants[0] != first {
		t.Fatalf("variants[0] = %q, want first configured key variant %q", variants[0], first)
	}
}
