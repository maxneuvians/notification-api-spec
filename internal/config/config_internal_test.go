package config

import "testing"

func TestJSONStringMapUnmarshalText(t *testing.T) {
	var parsed jsonStringMap

	if err := parsed.UnmarshalText(nil); err != nil {
		t.Fatalf("UnmarshalText(nil) error = %v, want nil", err)
	}
	if len(parsed) != 0 {
		t.Fatalf("len(parsed) = %d, want 0", len(parsed))
	}

	if err := parsed.UnmarshalText([]byte(`{"alpha":"beta"}`)); err != nil {
		t.Fatalf("UnmarshalText(valid) error = %v, want nil", err)
	}
	if parsed["alpha"] != "beta" {
		t.Fatalf("parsed[alpha] = %q, want beta", parsed["alpha"])
	}

	if err := parsed.UnmarshalText([]byte(`{`)); err == nil {
		t.Fatal("expected error for invalid json")
	}
}
