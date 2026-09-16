package shell

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/charmbracelet/crush/internal/pinentry"
	"github.com/stretchr/testify/require"
)

// fakeGPGShim installs the pinentry package's fake gpg as "gpg" on the
// PATH of a shell and returns its path.
func fakeGPGShim(t *testing.T) (path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake gpg shim only runs on POSIX shells")
	}
	data, err := os.ReadFile(filepath.Join("..", "pinentry", "testdata", "fakegpg.sh"))
	require.NoError(t, err)
	path = filepath.Join(t.TempDir(), "gpg")
	require.NoError(t, os.WriteFile(path, data, 0o755))
	return path
}

// configureIntegration enables the integrated pinentry against the fake
// gpg shim and the default prompt service, and registers cleanup.
func configureIntegration(t *testing.T, gpgPath string) {
	t.Helper()
	t.Cleanup(pinentry.DisableIntegration)
	pinentry.ConfigureIntegration(pinentry.IntegrationConfig{
		Prompter: func(ctx context.Context, req pinentry.PromptRequest) (string, error) {
			return pinentry.DefaultPrompts().Prompt(ctx, req)
		},
		GPGPath: gpgPath,
	})
}

// answerPrompts responds to every prompt request from the default
// prompt service with the given secret.
func answerPrompts(t *testing.T, secret string) {
	t.Helper()
	svc := pinentry.DefaultPrompts()
	ch := svc.Subscribe(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		default:
			close(done)
		}
	})
	go func() {
		for {
			select {
			case <-done:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				svc.Respond(ev.Payload.ID, secret)
			}
		}
	}()
}

func TestGpgHandlerInterceptsCredentialOperations(t *testing.T) {
	gpgPath := fakeGPGShim(t)
	configureIntegration(t, gpgPath)
	answerPrompts(t, "secret")

	sh := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Env:        append(os.Environ(), "FAKE_GPG_MODE=locked"),
	})
	// The pipeline gives gpg a closed stdin instead of the test's
	// terminal, and --detach-sign marks a credential-relevant operation.
	stdout, stderr, err := sh.Exec(context.Background(), "echo data | gpg --detach-sign")
	require.NoError(t, err, "stdout: %s stderr: %s", stdout, stderr)
}

func TestGpgHandlerPassthroughWhenDisabled(t *testing.T) {
	gpgPath := fakeGPGShim(t)
	// Integration disabled: the command must run the plain fake gpg,
	// which in locked mode fails (exit 2) since no loopback flags or
	// prompt are involved.
	t.Setenv("PATH", filepath.Dir(gpgPath))

	sh := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Env:        append(os.Environ(), "FAKE_GPG_MODE=locked"),
	})
	_, _, err := sh.Exec(context.Background(), "echo data | gpg --detach-sign")
	require.Error(t, err)
}

func TestGpgHandlerFallsBackOnUnsupportedLoopback(t *testing.T) {
	gpgPath := fakeGPGShim(t)
	configureIntegration(t, gpgPath)
	// Unsupported loopback: the middleware delegates to the next
	// handler, which runs the plain fake gpg (exit 2, no prompting).
	t.Setenv("PATH", filepath.Dir(gpgPath))

	sh := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Env:        append(os.Environ(), "FAKE_GPG_MODE=unsupported"),
	})
	_, _, err := sh.Exec(context.Background(), "echo data | gpg --detach-sign")
	require.Error(t, err, "plain gpg still fails with the unsupported error")
}

func TestGpgHandlerLeavesNonCredentialGpgAlone(t *testing.T) {
	gpgPath := fakeGPGShim(t)
	configureIntegration(t, gpgPath)
	var prompted atomic.Bool
	svc := pinentry.DefaultPrompts()
	ch := svc.Subscribe(context.Background())
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })
	go func() {
		for {
			select {
			case <-done:
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				prompted.Store(true)
				svc.Respond(ev.Payload.ID, "secret")
			}
		}
	}()

	t.Setenv("PATH", filepath.Dir(gpgPath))
	sh := NewShell(&Options{
		WorkingDir: t.TempDir(),
		Env:        append(os.Environ(), "FAKE_GPG_MODE=cached"),
	})
	// --list-keys is not a credential operation: runs the plain binary.
	_, _, err := sh.Exec(context.Background(), "gpg --list-keys")
	require.NoError(t, err)
	require.False(t, prompted.Load(), "non-credential gpg must not prompt")
}
