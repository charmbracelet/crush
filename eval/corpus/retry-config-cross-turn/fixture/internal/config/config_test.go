package config

import "testing"

func TestParseEndpoint(t *testing.T) {
	cfg := Parse("endpoint: http://x\n")
	if cfg.Endpoint != "http://x" {
		t.Fatalf("Endpoint = %q", cfg.Endpoint)
	}
}

func TestParseEmpty(t *testing.T) {
	cfg := Parse("")
	if cfg.Endpoint != "" {
		t.Fatalf("Endpoint = %q, want empty", cfg.Endpoint)
	}
}
