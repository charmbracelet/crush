package pinentry

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartIntegrationAndSocketRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("socket wrapper uses a POSIX sh script")
	}

	ctx := context.Background()

	cleanup, err := StartIntegration(ctx, os.Args[0], 0)
	require.NoError(t, err)
	defer func() {
		cleanup()
		DisableIntegration()
	}()
	require.True(t, IntegrationEnabled())

	// The socket path is exported to spawned git processes via
	// GitWrapperEnv.
	var socketPath string
	for _, kv := range GitWrapperEnv() {
		if v, ok := strings.CutPrefix(kv, "CRUSH_PINENTRY_SOCK="); ok {
			socketPath = v
		}
	}
	require.NotEmpty(t, socketPath)

	// Answer prompts from the default service, as the TUI would.
	prompts := DefaultPrompts()
	sub := prompts.Subscribe(ctx)
	answered := make(chan string, 1)
	go func() {
		ev := <-sub
		require.Equal(t, "GPG is waiting", ev.Payload.Prompt)
		require.True(t, prompts.Respond(ev.Payload.ID, "socket-secret"))
		close(answered)
	}()

	// The git wrapper subcommand fetches the credential over the socket.
	secret, err := AskCredentialOverSocket(ctx, socketPath, PromptRequest{
		Prompt: "GPG is waiting",
		Kind:   KindPassphrase,
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
		Prompt: "GPG is waiting",
	})
	require.ErrorIs(t, err, ErrCancelled)

	cleanup()
	require.False(t, IntegrationEnabled(), "cleanup disables integration")
	require.NoFileExists(t, socketPath, "socket is removed on cleanup")
}
