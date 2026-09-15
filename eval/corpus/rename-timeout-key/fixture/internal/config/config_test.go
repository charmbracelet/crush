package config

import "testing"

func TestParseTimeoutMS(t *testing.T) {
	cfg := Parse("endpoint: http://x\ntimeout_ms: 45\nmax_connections: 4\n")
	if cfg.Endpoint != "http://x" {
		t.Fatalf("endpoint = %q", cfg.Endpoint)
	}
	if cfg.TimeoutMS != 45 {
		t.Fatalf("TimeoutMS = %d, want 45", cfg.TimeoutMS)
	}
	if cfg.MaxConnections != 4 {
		t.Fatalf("MaxConnections = %d, want 4", cfg.MaxConnections)
	}
}

func TestParseMissingKeys(t *testing.T) {
	cfg := Parse("")
	if cfg.TimeoutMS != 0 {
		t.Fatalf("TimeoutMS = %d, want 0", cfg.TimeoutMS)
	}
}
