package agent

import (
	"os"
	"strings"
	"testing"
)

const realProviderTestsEnv = "CRUSH_RUN_REAL_PROVIDER_TESTS"

func requireRealProviderTests(t *testing.T) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping real provider test in short mode")
	}

	switch strings.ToLower(strings.TrimSpace(os.Getenv(realProviderTestsEnv))) {
	case "1", "true", "yes", "on":
		return
	default:
		t.Skipf("skipping real provider test; set %s=1 to enable", realProviderTestsEnv)
	}
}
