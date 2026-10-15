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
			name:    "host password preserves account casing",
			prompt:  "Deploy@Example.com's password: ",
			want:    KindPassword,
			keyInfo: "Deploy@Example.com",
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

func TestParsePrompt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		prompt  string
		kind    Kind
		keyInfo string
		title   string
		message string
	}{
		{
			name:    "host password",
			prompt:  "deploy@example.com's password: ",
			kind:    KindPassword,
			keyInfo: "deploy@example.com",
			title:   "SSH Password",
			message: "OpenSSH needs the password for deploy@example.com.",
		},
		{
			name:    "key passphrase",
			prompt:  "Enter passphrase for key '/home/user/.ssh/id_ed25519': ",
			kind:    KindPassword,
			keyInfo: "/home/user/.ssh/id_ed25519",
			title:   "SSH Passphrase",
			message: "Unlock the SSH key at /home/user/.ssh/id_ed25519.",
		},
		{
			name:    "security key pin",
			prompt:  "Enter PIN for key '/home/user/.ssh/id_ed25519_sk': ",
			kind:    KindPassword,
			keyInfo: "/home/user/.ssh/id_ed25519_sk",
			title:   "Security Key PIN",
			message: "Your security key wants its PIN.",
		},
		{
			name:    "host key acceptance",
			prompt:  "Are you sure you want to continue connecting (yes/no/[fingerprint])?",
			kind:    KindConfirm,
			title:   "New Host",
			message: "OpenSSH has not seen this host before. Trust its key?",
		},
		{
			name:    "ssh-add key use",
			prompt:  "Allow use of key /home/user/.ssh/id_ed25519?\nKey fingerprint: SHA256:abcdef",
			kind:    KindConfirm,
			keyInfo: "/home/user/.ssh/id_ed25519",
			title:   "Use SSH Key",
			message: "Something wants to use the SSH key at /home/user/.ssh/id_ed25519.",
		},
		{
			name:    "fido user presence",
			prompt:  "Confirm user presence for key ECDSA-SK SHA256:abcdef",
			kind:    KindTouch,
			title:   "Touch Your Security Key",
			message: "Tap your security key to continue.",
		},
		{
			name:    "unknown prompt falls back to the raw text",
			prompt:  "Verification code: ",
			kind:    KindPassword,
			title:   "SSH Password",
			message: "Verification code:",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			req := ParsePrompt(c.prompt)
			require.Equal(t, c.kind, req.Kind)
			require.Equal(t, c.keyInfo, req.KeyInfo)
			require.Equal(t, c.title, req.Title)
			require.True(t, strings.HasPrefix(req.Message, c.message),
				"message %q should start with %q", req.Message, c.message)
			// Every message gets a playful aside appended.
			require.NotEqual(t, c.message, req.Message)
			require.Equal(t, c.prompt, req.Prompt)
		})
	}
}

func TestKeyInfoLabel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "Account", KeyInfoLabel("deploy@example.com's password: ", KindPassword))
	require.Equal(t, "Key", KeyInfoLabel("Enter passphrase for key '/k': ", KindPassword))
	require.Equal(t, "Key", KeyInfoLabel("Enter PIN for key '/k': ", KindPassword))
	require.Equal(t, "Key", KeyInfoLabel("Allow use of key /k?", KindConfirm))
}

func TestDescribeFillsInParsedCopy(t *testing.T) {
	t.Parallel()

	req := Describe(PromptRequest{Prompt: "git@example.com's password: "})
	require.Equal(t, KindPassword, req.Kind)
	require.Equal(t, "git@example.com", req.KeyInfo)
	require.Equal(t, "SSH Password", req.Title)
	require.True(t, strings.HasPrefix(req.Message, "OpenSSH needs the password for git@example.com."))

	// Pre-parsed copy is preserved untouched.
	filled := PromptRequest{
		Prompt:  "prompt",
		Kind:    KindPassword,
		KeyInfo: "key",
		Title:   "T",
		Message: "M",
	}
	require.Equal(t, filled, Describe(filled))
}

func TestWriteAskpassWrapper(t *testing.T) {
	t.Parallel()

	path, cleanup, err := WriteAskpassWrapper("/path/to/crush app")
	require.NoError(t, err)
	defer cleanup()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\nexec '/path/to/crush app' __ssh-askpass \"$@\"\n", string(data))

	// Windows does not honor Unix mode bits on os.WriteFile.
	if runtime.GOOS != "windows" {
		st, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o755), st.Mode().Perm())
	}
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
