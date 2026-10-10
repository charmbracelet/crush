package honcho

import (
	"os"
	"testing"
)

// TestMain isolates every test in this package from the developer's
// real Honcho configuration.
//
// Client.authorization falls back to the stored OAuth token when no
// API key is set, so without this a test that asserts "no credential
// means no Authorization header" reads ~/.honcho/crush-token.json and
// fails with a real access token printed in its diff. Setting the
// config directory once, here, means no individual test can forget
// and no credential can reach a test log.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "honcho-test-config-*")
	if err != nil {
		panic("honcho: create isolated test config dir: " + err.Error())
	}
	os.Setenv("HONCHO_CONFIG_DIR", dir) //nolint:errcheck

	// Credentials and endpoints also arrive by environment, and a
	// developer with these exported would otherwise change what the
	// tests exercise.
	for _, key := range []string{
		"HONCHO_API_KEY",
		"HONCHO_URL",
		"HONCHO_BASE_URL",
		"HONCHO_WORKSPACE",
		"HONCHO_WORKSPACE_ID",
		"HONCHO_PEER_NAME",
		"HONCHO_AI_PEER",
	} {
		os.Unsetenv(key) //nolint:errcheck
	}

	code := m.Run()
	os.RemoveAll(dir) //nolint:errcheck
	os.Exit(code)
}
