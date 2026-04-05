package main

import "testing"

func TestWorkerOpenDBReturnsError(t *testing.T) {
	if _, err := openDB("postgresql://127.0.0.1:1/notify?connect_timeout=1&sslmode=disable", 1); err == nil {
		t.Fatal("expected error, got nil")
	}
}
