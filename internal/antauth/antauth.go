// Package antauth lets Crush use the login of Anthropic's `ant` command line
// tool.
//
// `ant auth login` signs in to the Anthropic Console through the browser and
// keeps a short-lived access token in its own config directory. Crush never
// reads or stores that token itself. It only notices that a login exists and,
// when a request needs a token, asks `ant` for a fresh one with
// `ant auth print-credentials --access-token`, which refreshes it as needed.
//
// This is a different credential from the Claude Code subscription sign-in
// Crush used to offer.
package antauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Command is the name of the CLI whose login is used.
const Command = "ant"

// tokenTTL is how long a fetched token is reused. `ant` refreshes a token
// shortly before it expires, so a short reuse window keeps requests from
// spawning a process each time without outliving the token.
const tokenTTL = 30 * time.Second

// fetchTimeout bounds one call to `ant`, which may refresh over the network.
const fetchTimeout = 15 * time.Second

// ErrNoLogin means the `ant` login is missing, expired beyond refresh, or the
// command failed.
var ErrNoLogin = errors.New("no usable ant login: run `ant auth login`")

// configDir returns the directory `ant` keeps its configuration in, following
// the same lookup order as the CLI.
func configDir(getenv func(string) string) string {
	if dir := getenv("ANTHROPIC_CONFIG_DIR"); dir != "" {
		return dir
	}
	if xdg := getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "anthropic")
	}
	if home := getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "anthropic")
	}
	return ""
}

// profile returns the name of the active profile.
func profile(getenv func(string) string, dir string) string {
	if p := getenv("ANTHROPIC_PROFILE"); p != "" {
		return p
	}
	if data, err := os.ReadFile(filepath.Join(dir, "active_config")); err == nil {
		if name := strings.TrimSpace(string(data)); name != "" {
			return name
		}
	}
	return "default"
}

// Detect reports whether an `ant` login exists: the command is on the PATH
// and the active profile has a credentials file. It reads nothing from the
// file and makes no network request, so it is cheap enough to call while
// loading configuration. The login may still have expired.
func Detect(getenv func(string) string) bool {
	return detect(getenv, exec.LookPath)
}

func detect(getenv func(string) string, lookPath func(string) (string, error)) bool {
	dir := configDir(getenv)
	if dir == "" {
		return false
	}
	creds := filepath.Join(dir, "credentials", profile(getenv, dir)+".json")
	if info, err := os.Stat(creds); err != nil || info.IsDir() {
		return false
	}
	_, err := lookPath(Command)
	return err == nil
}

// Tokens fetches access tokens from the `ant` command.
type Tokens struct {
	command string

	mu      sync.Mutex
	token   string
	fetched time.Time
	now     func() time.Time
}

// NewTokens returns a token source that runs command. Use [Command] outside
// of tests.
func NewTokens(command string) *Tokens {
	return &Tokens{command: command, now: time.Now}
}

// Token returns a bearer access token. The token is reused for a short
// while, so calling this for every request is fine.
func (t *Tokens) Token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.token != "" && t.now().Sub(t.fetched) < tokenTTL {
		return t.token, nil
	}

	token, err := t.fetch(ctx)
	if err != nil {
		t.token = ""
		return "", err
	}
	t.token, t.fetched = token, t.now()
	return token, nil
}

// Invalidate drops the cached token, so the next call asks `ant` again. Call
// it when the API rejects a token.
func (t *Tokens) Invalidate() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.token = ""
}

func (t *Tokens) fetch(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, t.command, "auth", "print-credentials", "--access-token")
	// A key in the environment would override the login inside `ant`, which
	// is the opposite of what asking for the login's token means.
	cmd.Env = withoutAuthEnv(os.Environ())
	out, err := cmd.Output()
	if err != nil {
		// The output may hold a token and the error text may echo it, so
		// neither is included.
		return "", ErrNoLogin
	}
	token := strings.TrimSpace(string(out))
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", fmt.Errorf("%w: unexpected output from %s", ErrNoLogin, t.command)
	}
	return token, nil
}

func withoutAuthEnv(env []string) []string {
	kept := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if name == "ANTHROPIC_API_KEY" || name == "ANTHROPIC_AUTH_TOKEN" {
			continue
		}
		kept = append(kept, kv)
	}
	return kept
}

// Transport signs requests with the `ant` login's token. It sets the
// Authorization header and removes any API key header, since the Anthropic
// client library may add one from the environment.
type Transport struct {
	// Base is the transport requests are passed on to. A nil Base uses
	// [http.DefaultTransport].
	Base http.RoundTripper
	// Tokens supplies the access token.
	Tokens *Tokens
}

var _ http.RoundTripper = (*Transport)(nil)

// RoundTrip implements [http.RoundTripper].
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.Tokens.Token(req.Context())
	if err != nil {
		return nil, err
	}

	// RoundTrippers must not modify the caller's request.
	signed := req.Clone(req.Context())
	signed.Header.Del("X-Api-Key")
	signed.Header.Set("Authorization", "Bearer "+token)

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(signed)
	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		// The token may have been revoked or refreshed elsewhere; ask
		// `ant` again next time.
		t.Tokens.Invalidate()
	}
	return resp, err
}
