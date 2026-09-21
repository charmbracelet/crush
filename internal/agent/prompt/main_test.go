package prompt

import (
	"os"
	"testing"
)

// TestMain isolates this package's tests from the developer's own Crush
// setup and keeps them off the network.
//
// config.Init reads the global config and then runs provider model
// discovery against whatever it finds, so without this the tests reach
// out to every local provider the developer happens to have configured.
// That turns a pure prompt-rendering test into a slow one that fails
// when someone's homelab is down. Pointing the global config and data
// paths at an empty temp dir leaves discovery with nothing to contact.
//
// The bundled default providers stay on: config.Init refuses to build a
// store with no providers at all, and the bundled list ships its models
// rather than discovering them.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "crush-prompt-test-*")
	if err != nil {
		panic("prompt: create isolated test config dir: " + err.Error())
	}

	for key, value := range map[string]string{
		"CRUSH_GLOBAL_CONFIG":                dir,
		"CRUSH_GLOBAL_DATA":                  dir,
		"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE": "1",
	} {
		os.Setenv(key, value) //nolint:errcheck
	}

	code := m.Run()
	os.RemoveAll(dir) //nolint:errcheck
	os.Exit(code)
}
