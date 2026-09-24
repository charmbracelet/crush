// Package login authenticates Crush with OAuth-capable platforms from the
// command line. In interactive terminals it runs a small Bubble Tea program
// that guides the user through opening a browser and waiting for the
// callback. When stdin is not a terminal, it falls back to a plain,
// keypress-free flow suitable for scripts and SSH sessions.
package login

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/x/term"
	"github.com/pkg/browser"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/charmbracelet/crush/internal/oauth/copilot"
	"github.com/charmbracelet/crush/internal/oauth/hyper"
	"github.com/charmbracelet/crush/internal/oauth/openai"
)

// Platforms accepted by Run.
const (
	PlatformHyper   = "hyper"
	PlatformCopilot = "copilot"
	PlatformOpenAI  = "openai"
)

// startMessages are the first lines printed by the non-interactive flow.
var startMessages = map[string]string{
	PlatformHyper:   "Initiating device authorization...",
	PlatformCopilot: "Requesting device code from GitHub...",
	PlatformOpenAI:  "Starting browser authorization...",
}

// titles are the provider names shown in the mini TUI header.
var titles = map[string]string{
	PlatformHyper:   "Charm Hyper",
	PlatformCopilot: "GitHub Copilot",
	PlatformOpenAI:  "ChatGPT",
}

// flow is the provider-agnostic surface the login UIs drive. Start is
// called once to kick off the flow; Wait then blocks until the user
// completes authorization in the browser, and Close releases any
// resources the flow holds.
type flow interface {
	Start(ctx context.Context) (url string, userCode string, err error)
	Wait(ctx context.Context) (*oauth.Token, error)
	Close()
}

// Run authenticates with the given platform and returns the resulting
// OAuth token. It runs the mini TUI when stdin is a terminal and the plain
// non-interactive flow otherwise.
func Run(ctx context.Context, platform string) (*oauth.Token, error) {
	newFlow, err := flowFor(platform)
	if err != nil {
		return nil, err
	}

	if term.IsTerminal(os.Stdin.Fd()) {
		return runTUI(platform, newFlow)
	}
	return runCLI(ctx, platform, newFlow)
}

// runCLI performs the full flow without a TUI and without requiring any
// keypresses, so it can be driven from scripts and other non-interactive
// sessions.
func runCLI(ctx context.Context, platform string, newFlow func() flow) (*oauth.Token, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	f := newFlow()

	fmt.Println(startMessages[platform])
	url, userCode, err := f.Start(ctx)
	if err != nil {
		return nil, err
	}

	if userCode != "" {
		fmt.Printf("Your code: %s\n", userCode)
	}
	fmt.Printf("Open this URL and enter the code: %s\n", url)

	openBrowser(url)

	fmt.Println("Waiting for authorization...")
	token, err := f.Wait(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Println("Exchanging token...")
	return token, nil
}

// openBrowser opens the given URL in the user's default browser. If the
// browser can't be opened, the URL is printed for the user to open
// manually.
func openBrowser(rawURL string) {
	if err := browser.OpenURL(rawURL); err != nil {
		fmt.Printf("Please open this URL to authenticate:\n  %s\n", rawURL)
	}
}

// flowFor returns the constructor for the platform's flow.
func flowFor(platform string) (func() flow, error) {
	switch platform {
	case PlatformHyper:
		return func() flow { return &hyperFlow{} }, nil
	case PlatformCopilot:
		return func() flow { return &copilotFlow{} }, nil
	case PlatformOpenAI:
		return func() flow { return &openaiFlow{} }, nil
	default:
		return nil, fmt.Errorf("unknown platform: %s", platform)
	}
}

// hyperFlow runs the Charm Hyper device code flow.
type hyperFlow struct {
	deviceCode string
	expiresIn  int
}

func (f *hyperFlow) Start(ctx context.Context) (string, string, error) {
	resp, err := hyper.InitiateDeviceAuth(ctx)
	if err != nil {
		return "", "", err
	}
	f.deviceCode = resp.DeviceCode
	f.expiresIn = resp.ExpiresIn
	return resp.VerificationURL, resp.UserCode, nil
}

func (f *hyperFlow) Wait(ctx context.Context) (*oauth.Token, error) {
	refreshToken, err := hyper.PollForToken(ctx, f.deviceCode, f.expiresIn)
	if err != nil {
		return nil, err
	}

	token, err := hyper.ExchangeToken(ctx, refreshToken)
	if err != nil {
		return nil, err
	}

	introspect, err := hyper.IntrospectToken(ctx, token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("token introspection failed: %w", err)
	}
	if !introspect.Active {
		return nil, errors.New("access token is not active")
	}
	return token, nil
}

func (hyperFlow) Close() {}

// copilotFlow runs the GitHub Copilot device code flow.
type copilotFlow struct {
	deviceCode *copilot.DeviceCode
}

func (f *copilotFlow) Start(ctx context.Context) (string, string, error) {
	dc, err := copilot.RequestDeviceCode(ctx)
	if err != nil {
		return "", "", err
	}
	f.deviceCode = dc
	return dc.VerificationURI, dc.UserCode, nil
}

func (f *copilotFlow) Wait(ctx context.Context) (*oauth.Token, error) {
	return copilot.PollForToken(ctx, f.deviceCode)
}

func (copilotFlow) Close() {}

// openaiFlow runs the ChatGPT (OpenAI) authorization code flow with a
// loopback callback server.
type openaiFlow struct {
	f *openai.BrowserFlow
}

func (f *openaiFlow) Start(_ context.Context) (string, string, error) {
	bf, err := openai.StartBrowserFlow()
	if err != nil {
		return "", "", err
	}
	f.f = bf
	// The handoff page opens the authorization URL in a tab that can close
	// itself when finished; opening the raw URL would leave the tab behind,
	// since browsers refuse to close it after a consent flow.
	return bf.StartURL(), "", nil
}

func (f *openaiFlow) Wait(ctx context.Context) (*oauth.Token, error) {
	return f.f.Wait(ctx)
}

func (f *openaiFlow) Close() {
	if f.f != nil {
		f.f.Close()
	}
}
