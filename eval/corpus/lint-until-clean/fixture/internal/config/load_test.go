package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolkit.conf")
	if err := os.WriteFile(path, []byte("workspace=/tmp/ws\nverbose=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Workspace != "/tmp/ws" {
		t.Fatalf("Workspace = %q", cfg.Workspace)
	}
	if !cfg.Verbose {
		t.Fatal("Verbose = false, want true")
	}
}

func TestLoadDefaultsWorkspace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolkit.conf")
	if err := os.WriteFile(path, []byte("verbose=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Workspace != "." {
		t.Fatalf("Workspace = %q, want .", cfg.Workspace)
	}
}
