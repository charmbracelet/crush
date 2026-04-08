// Package copilot provides GitHub Copilot integration.
package copilot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/charmbracelet/crushcl/internal/log"
)

const (
	// UserAgent is the User-Agent header sent with Copilot API requests.
	UserAgent = "GitHubCopilotChat/0.32.4"
	// EditorVersion is the editor version header.
	EditorVersion = "vscode/1.105.1"
	// EditorPluginVersion is the Copilot chat plugin version.
	EditorPluginVersion = "copilot-chat/0.32.4"
	// IntegrationID is the Copilot integration ID.
	IntegrationID = "vscode-chat"

	// SignupURL is the URL for signing up for GitHub Copilot.
	SignupURL = "https://github.com/github-copilot/signup?editor=crush"
	// FreeURL is the URL for getting free access to Copilot Pro.
	FreeURL = "https://docs.github.com/en/copilot/how-tos/manage-your-account/get-free-access-to-copilot-pro"
)

var assistantRolePattern = regexp.MustCompile(`"role"\s*:\s*"assistant"`)

// Headers returns the HTTP headers to include with Copilot API requests.
func Headers() map[string]string {
	return map[string]string{
		"User-Agent":             UserAgent,
		"Editor-Version":         EditorVersion,
		"Editor-Plugin-Version":  EditorPluginVersion,
		"Copilot-Integration-Id": IntegrationID,
	}
}

// NewClient creates a new HTTP client with a custom transport that adds the
// X-Initiator header based on message history in the request body.
func NewClient(isSubAgent, debug bool) *http.Client {
	return &http.Client{
		Transport: &initiatorTransport{debug: debug, isSubAgent: isSubAgent},
	}
}

type initiatorTransport struct {
	debug      bool
	isSubAgent bool
}

func (t *initiatorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const (
		xInitiatorHeader = "X-Initiator"
		userInitiator    = "user"
		agentInitiator   = "agent"
	)

	if req == nil {
		return nil, fmt.Errorf("HTTP request is nil")
	}
	if req.Body == http.NoBody {
		// No body to inspect; default to user.
		req.Header.Set(xInitiatorHeader, userInitiator)
		slog.Debug("Setting X-Initiator header to user (no request body)")
		return t.roundTrip(req)
	}

	// Clone request to avoid modifying the original.
	req = req.Clone(req.Context())

	// Read the original body into bytes so we can examine it.
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	defer req.Body.Close()

	// Restore the original body using the preserved bytes.
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Check for assistant messages using regex to handle whitespace
	// variations in the JSON while avoiding full unmarshalling overhead.
	initiator := userInitiator
	if assistantRolePattern.Match(bodyBytes) || t.isSubAgent {
		slog.Debug("Setting X-Initiator header to agent (found assistant messages in history)")
		initiator = agentInitiator
	} else {
		slog.Debug("Setting X-Initiator header to user (no assistant messages)")
	}
	req.Header.Set(xInitiatorHeader, initiator)

	return t.roundTrip(req)
}

func (t *initiatorTransport) roundTrip(req *http.Request) (*http.Response, error) {
	if t.debug {
		return log.NewHTTPClient().Transport.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// RefreshTokenFromDisk reads the GitHub OAuth token from the disk cache.
func RefreshTokenFromDisk() (string, bool) {
	data, err := os.ReadFile(tokenFilePath())
	if err != nil {
		return "", false
	}
	var content map[string]struct {
		User        string `json:"user"`
		OAuthToken  string `json:"oauth_token"`
		GitHubAppID string `json:"githubAppId"`
	}
	if err := json.Unmarshal(data, &content); err != nil {
		return "", false
	}
	if app, ok := content["github.com:Iv1.b507a08c87ecfe98"]; ok {
		return app.OAuthToken, true
	}
	return "", false
}

func tokenFilePath() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "github-copilot/apps.json")
	default:
		return filepath.Join(os.Getenv("HOME"), ".config/github-copilot/apps.json")
	}
}