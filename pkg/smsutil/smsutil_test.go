package smsutil

import "testing"

func TestNormalizeDowngradesKnownNonGSMCharacters(t *testing.T) {
	input := "Hello\tworld\u200b \u2026 \U0001F347"
	got := Normalize(input)
	want := "Hello world ... ?"
	if got != want {
		t.Fatalf("Normalize() = %q, want %q", got, want)
	}
}

func TestNormalizePreservesHTMLTags(t *testing.T) {
	input := "<b>Hello</b>"
	if got := Normalize(input); got != input {
		t.Fatalf("Normalize() = %q, want %q", got, input)
	}
}

func TestApplyPrefix(t *testing.T) {
	got := ApplyPrefix("Sample service", "hello", true)
	if got != "Sample service: hello" {
		t.Fatalf("ApplyPrefix() = %q, want prefixed content", got)
	}
	if got := ApplyPrefix("", "hello", true); got != "hello" {
		t.Fatalf("ApplyPrefix() with empty service = %q, want hello", got)
	}
}
