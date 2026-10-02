package config

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryResolution(t *testing.T) {
	t.Parallel()
	defaults, err := ResolveRetry(nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, defaults.MaxRetries)
	require.Equal(t, "safe_step", defaults.StreamPolicy)
	zero := 0
	five := 5
	none := "none"
	policy, err := ResolveRetry(&RetryConfig{MaxRetries: &zero, Jitter: &none}, &RetryConfig{MaxRetries: &five})
	require.NoError(t, err)
	require.Equal(t, five, policy.MaxRetries)
	require.Equal(t, none, policy.Jitter)
	policy, err = ResolveRetry(&RetryConfig{MaxRetries: &five}, &RetryConfig{MaxRetries: &zero})
	require.NoError(t, err)
	require.Zero(t, policy.MaxRetries)
}

func TestRetryValidation(t *testing.T) {
	t.Parallel()
	invalid := []string{
		`{"max_retries":21}`, `{"max_retries":-1}`, `{"initial_delay_ms":-1}`,
		`{"max_delay_ms":0}`, `{"max_elapsed_ms":0}`, `{"jitter":"random"}`,
		`{"stream_policy":"unknown"}`, `{"unknown":true}`,
	}
	for _, raw := range invalid {
		var retry RetryConfig
		err := json.Unmarshal([]byte(raw), &retry)
		if err == nil {
			_, err = ResolveRetry(&retry, nil)
		}
		require.Error(t, err, raw)
	}
	infinity := math.Inf(1)
	_, err := ResolveRetry(&RetryConfig{BackoffMultiplier: &infinity}, nil)
	require.Error(t, err)
	large := 2147483647
	_, err = ResolveRetry(&RetryConfig{MaxElapsedMS: &large}, nil)
	require.Error(t, err)
	first := 30_000
	last := 100
	_, err = ResolveRetry(&RetryConfig{InitialDelayMS: &first}, &RetryConfig{MaxDelayMS: &last})
	require.Error(t, err)
	policy, err := ResolveRetry(&RetryConfig{InitialDelayMS: &last}, nil)
	require.NoError(t, err)
	require.Equal(t, 100*time.Millisecond, policy.InitialDelay)
}

func TestRetryConfigLoadValidation(t *testing.T) {
	t.Parallel()
	_, err := loadFromBytes([][]byte{[]byte(`{"options":{"retry":{"max_retries":21}}}`)})
	require.Error(t, err)
	_, err = loadFromBytes([][]byte{[]byte(`{"providers":{"example":{"retry":{"jitter":"bad"}}}}`)})
	require.Error(t, err)
	_, err = loadFromBytes([][]byte{[]byte(`{"options":{"retry":{"max_retries":0}}}`)})
	require.NoError(t, err)
}

func TestRetryConfigWriteValidation(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "crush.json")
	store := &ConfigStore{config: &Config{}, globalDataPath: path}
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o600))
	require.Error(t, store.writeConfigFields(ScopeGlobal, map[string]any{"options.retry.max_retries": 21}))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(data))
	require.NoError(t, store.writeConfigFields(ScopeGlobal, map[string]any{"options.retry.max_retries": 0}))
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{"options":{"retry":{"max_retries":0}}}`, string(data))
}
