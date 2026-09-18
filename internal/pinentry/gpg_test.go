package pinentry

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// writeFakeGPG installs the test gpg shim into a temp dir and returns
// its path.
func writeFakeGPG(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake gpg shim only runs on POSIX shells")
	}
	data, err := os.ReadFile(filepath.Join("testdata", "fakegpg.sh"))
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "fake-gpg")
	require.NoError(t, os.WriteFile(path, data, 0o755))
	return path
}

func testRunner(gpgPath string, prompter Prompter) *Runner {
	return &Runner{GPGPath: gpgPath, Prompter: prompter}
}

func testOptions(mode string, stdin string, args ...string) RunOptions {
	argv := append([]string{"gpg", "--detach-sign"}, args...)
	return RunOptions{
		Args:   argv,
		Env:    append(os.Environ(), "FAKE_GPG_MODE="+mode),
		Stdin:  strings.NewReader(stdin),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
}

// recordingPrompter records requests and returns a fixed secret.
type recordingPrompter struct {
	calls   int
	reqs    []PromptRequest
	secrets []string
}

func (r *recordingPrompter) prompter() Prompter {
	return func(ctx context.Context, req PromptRequest) (string, error) {
		r.calls++
		r.reqs = append(r.reqs, req)
		secret := "secret"
		if len(r.secrets) > r.calls-1 {
			secret = r.secrets[r.calls-1]
		}
		return secret, nil
	}
}

func TestRunnerSucceedsWhenCached(t *testing.T) {
	r := testRunner(writeFakeGPG(t), nil)
	// Cached mode: no credential needed, probe succeeds with a nil
	// prompter (no prompting at all).
	err := r.Run(context.Background(), testOptions("cached", "data"))
	require.NoError(t, err)
}

func TestRunnerPromptsAndSucceeds(t *testing.T) {
	rec := &recordingPrompter{}
	r := testRunner(writeFakeGPG(t), rec.prompter())
	err := r.Run(context.Background(), testOptions("locked", "data"))
	require.NoError(t, err)
	require.Equal(t, 1, rec.calls)
	require.Equal(t, KindPassphrase, rec.reqs[0].Kind)
	require.Equal(t, "Test Key <test@example.com>", rec.reqs[0].KeyInfo, "user id hint is surfaced")
	// The first prompt carries no error: the probe failure was an empty
	// passphrase, not a user attempt.
	require.Empty(t, rec.reqs[0].Error)
}

func TestRunnerRetriesBadPassphraseThenGivesUp(t *testing.T) {
	rec := &recordingPrompter{secrets: []string{"wrong", "wrong", "wrong"}}
	r := testRunner(writeFakeGPG(t), rec.prompter())
	err := r.Run(context.Background(), testOptions("locked", "data"))
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrFallback))
	require.Equal(t, maxPassphraseAttempts, rec.calls)
	// Retry counts increment per attempt.
	require.Equal(t, 0, rec.reqs[0].RetryCount)
	require.Equal(t, 1, rec.reqs[1].RetryCount)
	require.Equal(t, 2, rec.reqs[2].RetryCount)
	// Only genuine retries carry the error that rejected the previous
	// credential; the first prompt does not.
	require.Empty(t, rec.reqs[0].Error)
	require.NotEmpty(t, rec.reqs[1].Error)
	require.NotEmpty(t, rec.reqs[2].Error)
}

func TestRunnerFallsBackWhenLoopbackUnsupported(t *testing.T) {
	rec := &recordingPrompter{}
	r := testRunner(writeFakeGPG(t), rec.prompter())
	err := r.Run(context.Background(), testOptions("unsupported", "data"))
	require.ErrorIs(t, err, ErrFallback)
	require.Equal(t, 0, rec.calls, "no prompt on an unsupported loopback")
}

func TestRunnerGenuineFailureNoPrompt(t *testing.T) {
	rec := &recordingPrompter{}
	r := testRunner(writeFakeGPG(t), rec.prompter())
	err := r.Run(context.Background(), testOptions("other", "data"))
	var ee *ExitError
	require.ErrorAs(t, err, &ee)
	require.Equal(t, 1, ee.Code)
	require.Equal(t, 0, rec.calls)
}

func TestRunWithNilPrompterFallsBack(t *testing.T) {
	r := testRunner(writeFakeGPG(t), nil)
	err := r.Run(context.Background(), testOptions("locked", "data"))
	require.ErrorIs(t, err, ErrFallback)
}

func TestHasCredentialOperation(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"gpg", "--sign", "f"}, true},
		{[]string{"gpg", "-s", "f"}, true},
		{[]string{"gpg", "-sbau", "keyid"}, true},
		{[]string{"gpg2", "-c"}, true},
		{[]string{"gpg", "-d", "f.gpg"}, true},
		{[]string{"gpg", "--clearsign", "f"}, true},
		{[]string{"gpg", "--detach-sign", "f"}, true},
		{[]string{"gpg", "--symmetric", "f"}, true},
		{[]string{"gpg", "--list-keys"}, false},
		{[]string{"gpg", "--verify", "sig", "f"}, false},
		{[]string{"gpg", "-e", "f"}, false},
		{[]string{"git", "commit", "-S"}, false},
	}
	for _, c := range cases {
		require.Equal(t, c.want, hasCredentialOperation(c.args), "args: %v", c.args)
	}
}

func TestShouldInterceptAndWrapperEnv(t *testing.T) {
	defer DisableIntegration()
	ConfigureIntegration(IntegrationConfig{
		Prompter: func(ctx context.Context, req PromptRequest) (string, error) {
			return defaultPrompts.Prompt(ctx, req)
		},
		WrapperPath: "/tmp/x/gpg-wrapper",
		SocketPath:  "/tmp/x/pinentry.sock",
	})

	require.True(t, ShouldIntercept([]string{"gpg", "--sign", "f"}))
	require.True(t, ShouldIntercept([]string{"gpg2", "-s", "f"}))
	require.False(t, ShouldIntercept([]string{"git", "commit", "-S"}))
	require.False(t, ShouldIntercept([]string{"gpg", "--list-keys"}))
	require.False(t, ShouldIntercept([]string{"gpg"}))
	require.False(t, ShouldIntercept([]string{"gpg", "--passphrase-fd", "0", "-s"}))
	require.False(t, ShouldIntercept([]string{"gpg", "--pinentry-mode", "loopback", "-s"}))

	env := GitWrapperEnv()
	require.Contains(t, env, "GIT_CONFIG_COUNT=1")
	require.Contains(t, env, "GIT_CONFIG_KEY_0=gpg.program")
	require.Contains(t, env, "GIT_CONFIG_VALUE_0=/tmp/x/gpg-wrapper")
	require.Contains(t, env, "CRUSH_PINENTRY_SOCK=/tmp/x/pinentry.sock")

	DisableIntegration()
	require.False(t, IntegrationEnabled())
	require.Nil(t, GitWrapperEnv())
}

func TestWriteGPGWrapper(t *testing.T) {
	path, cleanup, err := WriteGPGWrapper("/path/to/crush app")
	require.NoError(t, err)

	// Windows has no POSIX sh, so no wrapper is written there.
	if runtime.GOOS == "windows" {
		require.Empty(t, path)
		require.Nil(t, cleanup)
		return
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\nexec '/path/to/crush app' __pinentry-gpg \"$@\"\n", string(data))

	st, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), st.Mode().Perm())
}

func TestPassphraseCache(t *testing.T) {
	c := newPassphraseCache(time.Millisecond)
	got, ok := c.get("key")
	require.False(t, ok)
	require.Empty(t, got)

	c.set("key", "secret")
	got, ok = c.get("key")
	require.True(t, ok)
	require.Equal(t, "secret", got)

	// Overwrites replace and drop the timed entry.
	c.set("key", "newsecret")
	got, ok = c.get("key")
	require.True(t, ok)
	require.Equal(t, "newsecret", got)

	time.Sleep(2 * time.Millisecond)
	_, ok = c.get("key")
	require.False(t, ok, "expired entries are dropped")
}
