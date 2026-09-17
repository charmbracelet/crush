package sshaskpass

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifyPrompt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		prompt  string
		want    Kind
		keyInfo string
	}{
		{
			name:    "host password",
			prompt:  "deploy@example.com's password: ",
			want:    KindPassword,
			keyInfo: "deploy@example.com",
		},
		{
			name:    "key passphrase",
			prompt:  "Enter passphrase for key '/home/user/.ssh/id_ed25519': ",
			want:    KindPassword,
			keyInfo: "/home/user/.ssh/id_ed25519",
		},
		{
			name:   "fido user presence",
			prompt: "Confirm user presence for key ECDSA-SK SHA256:abcdef",
			want:   KindTouch,
		},
		{
			name:   "touch the device",
			prompt: "Please touch the device. Confirm user presence for key ECDSA-SK SHA256:abcdef",
			want:   KindTouch,
		},
		{
			name:   "host key acceptance",
			prompt: "Are you sure you want to continue connecting (yes/no/[fingerprint])?",
			want:   KindConfirm,
		},
		{
			name:   "ssh-add key use",
			prompt: "Allow use of key /home/user/.ssh/id_ed25519?\nKey fingerprint: SHA256:abcdef",
			want:   KindConfirm,
		},
		{
			name:   "unknown prompt defaults to password",
			prompt: "Verification code: ",
			want:   KindPassword,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			kind, keyInfo := ClassifyPrompt(c.prompt)
			require.Equal(t, c.want, kind)
			require.Equal(t, c.keyInfo, keyInfo)
		})
	}
}

func TestWriteAskpassWrapper(t *testing.T) {
	t.Parallel()

	path, cleanup, err := WriteAskpassWrapper("/path/to/crush app")
	require.NoError(t, err)
	defer cleanup()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\nexec '/path/to/crush app' __ssh-askpass \"$@\"\n", string(data))

	st, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o755), st.Mode().Perm())
}

func TestAskpassEnv(t *testing.T) {
	t.Parallel()
	defer DisableIntegration()

	ConfigureIntegration(IntegrationConfig{
		AskpassPath: "/tmp/x/ssh-askpass",
		SocketPath:  "/tmp/x/ssh.sock",
	})

	env := AskpassEnv()
	require.Contains(t, env, "SSH_ASKPASS=/tmp/x/ssh-askpass")
	require.Contains(t, env, "SSH_ASKPASS_REQUIRE=force")
	require.Contains(t, env, "CRUSH_SSH_SOCK=/tmp/x/ssh.sock")

	DisableIntegration()
	require.Nil(t, AskpassEnv())
	require.False(t, IntegrationEnabled())
}

func TestStartIntegrationAndSocketRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("askpass wrapper uses a POSIX sh script")
	}

	ctx := context.Background()

	cleanup, err := StartIntegration(ctx, os.Args[0])
	require.NoError(t, err)
	defer func() {
		cleanup()
		DisableIntegration()
	}()
	require.True(t, IntegrationEnabled())

	// The socket path is exported to spawned ssh processes.
	var socketPath string
	for _, kv := range AskpassEnv() {
		if v, ok := strings.CutPrefix(kv, "CRUSH_SSH_SOCK="); ok {
			socketPath = v
		}
	}
	require.NotEmpty(t, socketPath, "AskpassEnv exports the socket path")

	// Answer prompts from the default service, as the TUI would.
	prompts := DefaultPrompts()
	sub := prompts.Subscribe(ctx)
	answered := make(chan struct{})
	go func() {
		ev := <-sub
		require.Equal(t, "deploy@example.com's password: ", ev.Payload.Prompt)
		require.Equal(t, KindPassword, ev.Payload.Kind)
		require.Equal(t, "deploy@example.com", ev.Payload.KeyInfo)
		require.True(t, prompts.Respond(ev.Payload.ID, "socket-secret"))
		close(answered)
	}()

	// The askpass wrapper subcommand fetches the credential over the
	// socket, after classifying the prompt.
	kind, keyInfo := ClassifyPrompt("deploy@example.com's password: ")
	secret, err := AskCredentialOverSocket(ctx, socketPath, PromptRequest{
		Prompt:  "deploy@example.com's password: ",
		KeyInfo: keyInfo,
		Kind:    kind,
	})
	require.NoError(t, err)
	require.Equal(t, "socket-secret", secret)
	<-answered

	// Cancellation round-trips as ErrCancelled.
	go func() {
		ev := <-sub
		require.True(t, prompts.Cancel(ev.Payload.ID))
	}()
	_, err = AskCredentialOverSocket(ctx, socketPath, PromptRequest{
		Prompt: "prompt",
	})
	require.ErrorIs(t, err, ErrCancelled)

	cleanup()
	require.False(t, IntegrationEnabled(), "cleanup disables integration")
	require.NoFileExists(t, socketPath, "socket is removed on cleanup")
}

// fakeAskpassShim installs a fake askpass program that echoes a fixed
// answer, proving the SSH_ASKPASS env injected into shells points at an
// executable the ssh client can run.
func fakeAskpassShim(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake askpass shim only runs on POSIX shells")
	}
	path := filepath.Join(t.TempDir(), "ssh-askpass")
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\necho secret\n"), 0o755))
	return path
}

func TestAskpassEnvFromConfiguredIntegration(t *testing.T) {
	t.Parallel()
	defer DisableIntegration()

	askpass := fakeAskpassShim(t)
	ConfigureIntegration(IntegrationConfig{
		AskpassPath: askpass,
		SocketPath:  "/tmp/x/ssh.sock",
	})
	require.Contains(t, AskpassEnv(), "SSH_ASKPASS="+askpass)
}
