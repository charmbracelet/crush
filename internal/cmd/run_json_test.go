package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

// fakeUsage looks usage up by exact session ID, like both real lookups do.
func fakeUsage(store map[string]sessionUsage) func(context.Context, string) (sessionUsage, error) {
	return func(_ context.Context, id string) (sessionUsage, error) {
		u, ok := store[id]
		if !ok {
			return sessionUsage{}, fmt.Errorf("session not found: %s", id)
		}
		return u, nil
	}
}

// fakeRun mimics what both real run paths do: report the session, stream
// text, then end with the extra newline.
func fakeRun(id, text string, err error) func(io.Writer, func(string)) error {
	return func(out io.Writer, onSession func(string)) error {
		onSession(id)
		fmt.Fprint(out, text)
		fmt.Fprintln(out)
		return err
	}
}

func decodeOne(t *testing.T, b []byte) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	var m map[string]any
	require.NoError(t, dec.Decode(&m))
	require.False(t, dec.More(), "stdout must hold exactly one JSON object")
	return m
}

func TestRunJSONSuccessShape(t *testing.T) {
	t.Parallel()
	var w bytes.Buffer
	err := runJSON(t.Context(), &w,
		fakeUsage(map[string]sessionUsage{"s1": {promptTokens: 12, completionTokens: 34, cost: 0.5}}),
		fakeRun("s1", "hello <b>world</b>\nline2", nil))
	require.NoError(t, err)

	m := decodeOne(t, w.Bytes())
	require.Equal(t, map[string]any{
		"session_id":        "s1",
		"text":              "hello <b>world</b>\nline2",
		"prompt_tokens":     float64(12),
		"completion_tokens": float64(34),
		"cost":              0.5,
	}, m)
	require.Contains(t, w.String(), "<b>world</b>", "HTML must not be escaped")
}

func TestRunJSONErrorShape(t *testing.T) {
	t.Parallel()
	var w bytes.Buffer
	boom := errors.New("agent processing failed: boom")
	err := runJSON(t.Context(), &w,
		fakeUsage(map[string]sessionUsage{"s1": {promptTokens: 7}}),
		fakeRun("s1", "partial", boom))
	require.ErrorIs(t, err, boom, "error is returned so the process exits 1")

	m := decodeOne(t, w.Bytes())
	require.Equal(t, "agent processing failed: boom", m["error"])
	require.Equal(t, "s1", m["session_id"])
	require.Equal(t, "partial", m["text"])
	require.Equal(t, float64(7), m["prompt_tokens"])

	// Failure before any session exists still prints one object.
	w.Reset()
	err = runJSON(t.Context(), &w, fakeUsage(nil),
		func(io.Writer, func(string)) error { return errors.New("no model") })
	require.Error(t, err)
	m = decodeOne(t, w.Bytes())
	require.Equal(t, "", m["session_id"])
	require.Equal(t, "no model", m["error"])
}

func TestRunJSONReportsOwnSession(t *testing.T) {
	t.Parallel()
	store := map[string]sessionUsage{
		"a": {promptTokens: 1, completionTokens: 2, cost: 0.1},
		"b": {promptTokens: 10, completionTokens: 20, cost: 0.2},
	}
	var wa, wb bytes.Buffer
	require.NoError(t, runJSON(t.Context(), &wa, fakeUsage(store), fakeRun("a", "A", nil)))
	require.NoError(t, runJSON(t.Context(), &wb, fakeUsage(store), fakeRun("b", "B", nil)))

	ma, mb := decodeOne(t, wa.Bytes()), decodeOne(t, wb.Bytes())
	require.Equal(t, "a", ma["session_id"])
	require.Equal(t, float64(1), ma["prompt_tokens"])
	require.Equal(t, "b", mb["session_id"])
	require.Equal(t, float64(20), mb["completion_tokens"])
}

func TestRunJSONContinueSession(t *testing.T) {
	t.Parallel()
	var w bytes.Buffer
	// A continued run reports the existing session; its accumulated usage
	// is what comes back.
	err := runJSON(t.Context(), &w,
		fakeUsage(map[string]sessionUsage{"old": {promptTokens: 99, completionTokens: 1}}),
		fakeRun("old", "again", nil))
	require.NoError(t, err)
	m := decodeOne(t, w.Bytes())
	require.Equal(t, "old", m["session_id"])
	require.Equal(t, float64(99), m["prompt_tokens"])
}

func TestRunJSONUsageLookupFailureIsReported(t *testing.T) {
	t.Parallel()
	var w bytes.Buffer
	err := runJSON(t.Context(), &w, fakeUsage(nil), fakeRun("gone", "x", nil))
	require.Error(t, err)
	m := decodeOne(t, w.Bytes())
	require.Contains(t, m["error"], "failed to read session usage")
}

func TestRunFlagErrorIsUsageError(t *testing.T) {
	t.Parallel()
	err := runCmd.FlagErrorFunc()(runCmd, errors.New("unknown flag: --nope"))
	var ue *usageError
	require.ErrorAs(t, err, &ue)
}

// runCLI runs `crush <args>` through rootCmd in a clean sandbox and returns
// what it wrote to stdout plus the error Execute returned.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = oldStdout
		runCmd.Flags().VisitAll(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	})
	rootCmd.SetArgs(args)
	execErr := rootCmd.ExecuteContext(t.Context())
	require.NoError(t, w.Close())
	os.Stdout = oldStdout
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out), execErr
}

// Failures that happen before the agent starts must still print the one
// JSON object (and return an error so the process exits 1).
func TestRunJSONPreRunFailuresStillPrintJSON(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("CRUSH_CLIENT_SERVER", "")
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasSuffix(k, "_API_KEY") || strings.HasSuffix(k, "_API_TOKEN") {
			t.Setenv(k, "")
		}
	}
	proj := filepath.Join(dir, "proj")
	require.NoError(t, os.MkdirAll(proj, 0o755))
	t.Chdir(proj) // Restored on cleanup; the command resolves the cwd itself.
	common := []string{"run", "--json", "--data-dir", filepath.Join(dir, "dd")}

	// No provider configured.
	out, err := runCLI(t, append(common, "hello")...)
	require.Error(t, err)
	m := decodeOne(t, []byte(out))
	require.Contains(t, m["error"], "no providers configured")

	// Provider configured, unknown --session.
	conf := `{"providers":{"fake":{"type":"openai-compat","base_url":"http://127.0.0.1:1/v1","api_key":"x","models":[{"id":"m","name":"m","context_window":1000,"default_max_tokens":100}]}},"models":{"large":{"provider":"fake","model":"m"},"small":{"provider":"fake","model":"m"}},"options":{"disable_provider_auto_update":true,"disable_metrics":true}}`
	require.NoError(t, os.WriteFile(filepath.Join(proj, "crush.json"), []byte(conf), 0o644))
	out, err = runCLI(t, append(common, "-s", "doesnotexist", "hello")...)
	require.Error(t, err)
	m = decodeOne(t, []byte(out))
	require.NotEmpty(t, m["error"])
	require.Equal(t, "", m["session_id"])

	// Usage errors stay off stdout.
	out, err = runCLI(t, append(common, "-s", "x", "-C", "hello")...)
	require.Error(t, err)
	require.Empty(t, out)
}
