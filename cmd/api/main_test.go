package main

import "testing"

func TestListenAddr(t *testing.T) {
	tests := []struct {
		port string
		want string
	}{
		{port: "8080", want: ":8080"},
		{port: ":9090", want: ":9090"},
		{port: "127.0.0.1:7000", want: "127.0.0.1:7000"},
	}

	for _, tc := range tests {
		if got := listenAddr(tc.port); got != tc.want {
			t.Fatalf("listenAddr(%q) = %q, want %q", tc.port, got, tc.want)
		}
	}
}

func TestOpenDBReturnsError(t *testing.T) {
	if _, err := openDB("postgresql://127.0.0.1:1/notify?connect_timeout=1&sslmode=disable", 1); err == nil {
		t.Fatal("expected error, got nil")
	}
}
