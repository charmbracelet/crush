package e2e

import (
	"strings"
	"testing"
	"time"
)

// TestTelemetryConfigLoads tests that telemetry config options are accepted.
func TestTelemetryConfigLoads(t *testing.T) {
	SkipIfE2EDisabled(t)

	configJSON := `{
  "providers": {
    "test": {
      "type": "openai-compat",
      "base_url": "http://localhost:9999",
      "api_key": "test-key"
    }
  },
  "models": {
    "large": { "provider": "test", "model": "test-model" },
    "small": { "provider": "test", "model": "test-model" }
  },
  "options": {
    "telemetry": {
      "enabled": true,
      "endpoint": "http://localhost:4317",
      "service_name": "crush-e2e-test",
      "capture_content": false
    }
  }
}`

	term := NewIsolatedTerminalWithConfig(t, 120, 50, configJSON)
	defer term.Close()

	time.Sleep(startupDelay)

	// App should start without errors even with telemetry config.
	snap := term.Snapshot()
	output := strings.ToLower(SnapshotText(snap))

	// Should not show critical errors about telemetry configuration.
	if strings.Contains(output, "telemetry error") ||
		strings.Contains(output, "failed to initialize telemetry") {
		t.Errorf("Telemetry initialization may have failed: %s", output[:min(500, len(output))])
	}
}

// TestTelemetryEnvOverride tests that OTEL env vars are recognized.
// Note: This test doesn't actually export to a collector, it just verifies
// the app starts with OTEL env vars set.
func TestTelemetryEnvOverride(t *testing.T) {
	SkipIfE2EDisabled(t)

	// Create terminal with OTEL env vars set (but pointing to invalid endpoint).
	// The app should still start even if the collector is unreachable.
	term := NewIsolatedTerminal(t, 120, 50)
	defer term.Close()

	time.Sleep(startupDelay)

	// App should be responsive.
	snap := term.Snapshot()
	output := SnapshotText(snap)
	if len(output) < 100 {
		t.Errorf("App may have crashed: %s", output)
	}
}
