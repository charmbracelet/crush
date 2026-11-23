package backend

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Bypass trust verification so project configs written to temp
	// directories are loaded without a trust prompt.
	os.Setenv("CRUSH_TRUST_ALL", "1")
	os.Exit(m.Run())
}
