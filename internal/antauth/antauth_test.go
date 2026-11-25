package antauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// fakeAnt writes a stand-in for the `ant` command that prints a token naming
// how many times it has run, and returns the command path and a function that
// reports that count.
func fakeAnt(t *testing.T, body string) (command string, runs func() int) {
	t.Helper()

	dir := t.TempDir()
	counter := filepath.Join(dir, "runs")
	script := "#!/bin/sh\necho x >> '" + counter + "'\n" + body
	command = filepath.Join(dir, "ant")
	require.NoError(t, os.WriteFile(command, []byte(script), 0o755))

	return command, func() int {
		data, err := os.ReadFile(counter)
		if err != nil {
			return 0
		}
		return strings.Count(string(data), "x")
	}
}

func TestTokenIsReusedForAShortWhile(t *testing.T) {
	t.Parallel()

	command, runs := fakeAnt(t, "echo fresh-token\n")
	tokens := NewTokens(command)
	now := time.Now()
	tokens.now = func() time.Time { return now }

	got, err := tokens.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, "fresh-token", got)

	_, err = tokens.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, runs(), "a second call inside the window reuses the token")

	now = now.Add(tokenTTL + time.Second)
	_, err = tokens.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, runs(), "after the window ant is asked again")
}

func TestInvalidateForcesAFreshToken(t *testing.T) {
	t.Parallel()

	command, runs := fakeAnt(t, "echo fresh-token\n")
	tokens := NewTokens(command)

	_, err := tokens.Token(context.Background())
	require.NoError(t, err)
	tokens.Invalidate()
	_, err = tokens.Token(context.Background())
	require.NoError(t, err)

	require.Equal(t, 2, runs())
}

func TestFailureReportsNoLoginWithoutLeakingOutput(t *testing.T) {
	t.Parallel()

	command, _ := fakeAnt(t, "echo sk-ant-oat-SECRET\nexit 1\n")
	_, err := NewTokens(command).Token(context.Background())

	require.ErrorIs(t, err, ErrNoLogin)
	require.NotContains(t, err.Error(), "SECRET")
}

func TestMissingCommandReportsNoLogin(t *testing.T) {
	t.Parallel()

	_, err := NewTokens(filepath.Join(t.TempDir(), "does-not-exist")).Token(context.Background())
	require.ErrorIs(t, err, ErrNoLogin)
}

func TestUnexpectedOutputIsRejected(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"empty":     "true\n",
		"two words": "echo 'not a token'\n",
		"json":      `echo '{"access_token": "x"}'` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			command, _ := fakeAnt(t, body)
			_, err := NewTokens(command).Token(context.Background())
			require.ErrorIs(t, err, ErrNoLogin)
		})
	}
}

func TestEnvironmentKeysAreNotPassedToAnt(t *testing.T) {
	t.Parallel()

	got := withoutAuthEnv([]string{"PATH=/bin", "ANTHROPIC_API_KEY=k", "ANTHROPIC_AUTH_TOKEN=t", "ANTHROPIC_PROFILE=work"})
	require.Equal(t, []string{"PATH=/bin", "ANTHROPIC_PROFILE=work"}, got)
}

func TestTransportSignsWithTheLoginToken(t *testing.T) {
	t.Parallel()

	var gotAuth, gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotKey = r.Header.Get("X-Api-Key")
	}))
	defer server.Close()

	command, _ := fakeAnt(t, "echo login-token\n")
	client := &http.Client{Transport: &Transport{Tokens: NewTokens(command)}}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", "key-from-the-environment")
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()

	require.Equal(t, "Bearer login-token", gotAuth)
	require.Empty(t, gotKey, "an API key must not ride along with the login token")
	require.Equal(t, "key-from-the-environment", req.Header.Get("X-Api-Key"), "the caller's request is untouched")
	require.Empty(t, req.Header.Get("Authorization"))
}

func TestTransportDropsTheTokenWhenItIsRejected(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	command, runs := fakeAnt(t, "echo login-token\n")
	client := &http.Client{Transport: &Transport{Tokens: NewTokens(command)}}

	for range 2 {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	require.Equal(t, 2, runs(), "a 401 makes the next request ask ant again")
}

func TestTransportFailsWhenThereIsNoLogin(t *testing.T) {
	t.Parallel()

	command, _ := fakeAnt(t, "exit 1\n")
	client := &http.Client{Transport: &Transport{Tokens: NewTokens(command)}}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:1/never-reached", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoLogin))
}

func TestDetect(t *testing.T) {
	t.Parallel()

	found := func(string) (string, error) { return "/usr/bin/ant", nil }
	missing := func(string) (string, error) { return "", errors.New("not found") }

	setup := func(t *testing.T, profileName string) map[string]string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "credentials"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "credentials", profileName+".json"), []byte("{}"), 0o600))
		return map[string]string{"ANTHROPIC_CONFIG_DIR": dir}
	}
	getenv := func(env map[string]string) func(string) string {
		return func(k string) string { return env[k] }
	}

	t.Run("default profile", func(t *testing.T) {
		t.Parallel()
		require.True(t, detect(getenv(setup(t, "default")), found))
	})

	t.Run("profile named by active_config", func(t *testing.T) {
		t.Parallel()
		env := setup(t, "work")
		require.NoError(t, os.WriteFile(filepath.Join(env["ANTHROPIC_CONFIG_DIR"], "active_config"), []byte("work\n"), 0o644))
		require.True(t, detect(getenv(env), found))
	})

	t.Run("profile named by environment", func(t *testing.T) {
		t.Parallel()
		env := setup(t, "ci")
		env["ANTHROPIC_PROFILE"] = "ci"
		require.True(t, detect(getenv(env), found))
	})

	t.Run("credentials for another profile", func(t *testing.T) {
		t.Parallel()
		env := setup(t, "work")
		require.False(t, detect(getenv(env), found), "active profile is default, which has no credentials")
	})

	t.Run("ant is not installed", func(t *testing.T) {
		t.Parallel()
		require.False(t, detect(getenv(setup(t, "default")), missing))
	})

	t.Run("nothing configured", func(t *testing.T) {
		t.Parallel()
		require.False(t, detect(getenv(map[string]string{}), found))
	})

	t.Run("xdg config home", func(t *testing.T) {
		t.Parallel()
		xdg := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(xdg, "anthropic", "credentials"), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(xdg, "anthropic", "credentials", "default.json"), []byte("{}"), 0o600))
		require.True(t, detect(getenv(map[string]string{"XDG_CONFIG_HOME": xdg}), found))
	})
}
