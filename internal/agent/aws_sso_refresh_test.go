package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

func TestExtractAWSSSOURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name: "standard aws sso login output",
			output: `If the browser does not open or you wish to use a different device to authorize this request, open the following URL:
https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH`,
			want: "https://device.sso.us-east-1.amazonaws.com/?user_code=ABCD-EFGH",
		},
		{
			name:   "url only",
			output: "https://device.sso.eu-west-1.amazonaws.com/?user_code=XXXX-YYYY",
			want:   "https://device.sso.eu-west-1.amazonaws.com/?user_code=XXXX-YYYY",
		},
		{
			name:   "no url",
			output: "SSO session expired. Please run aws sso login.",
			want:   "",
		},
		{
			name:   "empty output",
			output: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := extractAWSSSOURL(tt.output); got != tt.want {
				t.Errorf("extractAWSSSOURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// joinedCtx reports when refreshAWSCredentials first waits on it, which
// happens only after the caller has joined the shared login.
type joinedCtx struct {
	context.Context
	once   sync.Once
	joined chan<- struct{}
}

func (c *joinedCtx) Done() <-chan struct{} {
	c.once.Do(func() { c.joined <- struct{}{} })
	return c.Context.Done()
}

// Requests that hit an expired SSO session together share one login, and a
// cancelled one stops waiting without aborting the login for the rest.
func TestAWSRefreshSharesOneLoginAndHonorsCancel(t *testing.T) {
	t.Parallel()
	env := testEnv(t)
	crushJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {"mock": {"id": "mock", "name": "Mock", "type": "openai",
    "base_url": "http://127.0.0.1:9/v1", "api_key": "test-key",
    "models": [{"id": "mock-model", "name": "Mock", "context_window": 8192, "default_max_tokens": 128}]}},
  "models": {"large": {"provider": "mock", "model": "mock-model"},
             "small": {"provider": "mock", "model": "mock-model"}}
}`
	require.NoError(t, os.WriteFile(filepath.Join(env.workingDir, "crush.json"), []byte(crushJSON), 0o644))
	cfg, err := config.Init(t.Context(), env.workingDir, "", false)
	require.NoError(t, err)
	c := &coordinator{cfg: cfg, sessions: env.sessions, messages: env.messages}
	broker := pubsub.NewBroker[notify.Notification]()
	t.Cleanup(broker.Shutdown)
	c.notify = broker

	// The command runs in the working directory, so relative names keep
	// paths out of the shell syntax. Each login appends a marker, then
	// blocks until the test creates the release file.
	runs := filepath.Join(env.workingDir, "runs")
	release := filepath.Join(env.workingDir, "release")
	provider := config.ProviderConfig{
		ID:             "bedrock",
		AWSAuthRefresh: "echo x >> runs; until [ -f release ]; do sleep 0.05; done",
	}
	releaseLogin := func() { _ = os.WriteFile(release, nil, 0o644) }
	t.Cleanup(releaseLogin)

	const callers = 4
	joined := make(chan struct{}, callers)
	canceledCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	canceledErr := make(chan error, 1)
	otherErrs := make(chan error, callers-1)
	for i := range callers {
		if i == 0 {
			ctx := &joinedCtx{Context: canceledCtx, joined: joined}
			go func() { canceledErr <- c.refreshAWSCredentials(ctx, provider) }()
			continue
		}
		ctx := &joinedCtx{Context: t.Context(), joined: joined}
		go func() { otherErrs <- c.refreshAWSCredentials(ctx, provider) }()
	}

	wait := func(ch <-chan struct{}, what string) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(10 * time.Second):
			t.Fatalf("Timed out waiting for %s", what)
		}
	}
	for range callers {
		wait(joined, "callers to join the login")
	}

	// The login is still blocked, so the cancelled caller must return on
	// its own while the others keep waiting.
	cancel()
	select {
	case err := <-canceledErr:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		t.Fatal("Timed out waiting for the cancelled caller")
	}
	select {
	case err := <-otherErrs:
		t.Fatalf("Caller returned before the login finished: %v", err)
	default:
	}

	releaseLogin()
	for range callers - 1 {
		select {
		case err := <-otherErrs:
			require.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Timed out waiting for the shared login")
		}
	}
	out, err := os.ReadFile(runs)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(out), "x"), "one login for every waiting request")
}
