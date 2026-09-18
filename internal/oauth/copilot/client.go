// Package copilot provides GitHub Copilot integration.
package copilot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/oauth"
)

var (
	assistantRolePattern = regexp.MustCompile(`"role"\s*:\s*"assistant"`)
	autoModelPattern     = regexp.MustCompile(`"model"\s*:\s*"` + AutoModelID + `"`)
)

// NewClient creates a new HTTP client with a custom transport that adds the
// X-Initiator header based on message history in the request body. The
// token getter feeds auto mode: requests whose model is the "auto"
// pseudo-model are rewritten to a model the session grants and authorized
// with the session token. A nil getter disables auto mode.
func NewClient(isSubAgent, debug bool, token func() *oauth.Token) *http.Client {
	t := &initiatorTransport{debug: debug, isSubAgent: isSubAgent}
	if token != nil {
		t.auto = newAutoResolver(token)
	}
	return &http.Client{Transport: t}
}

type initiatorTransport struct {
	debug      bool
	isSubAgent bool
	auto       *autoResolver
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
	if req.Body == nil || req.Body == http.NoBody {
		// No body to inspect; default to user. A nil Body is valid for
		// bodyless requests (e.g. GET), and is distinct from http.NoBody,
		// so both must be handled before reading below.
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

	// The "auto" pseudo-model is not servable as-is: resolve it to a
	// model the auto session grants and authorize the request with the
	// session token, mirroring the official VS Code extension.
	if t.auto != nil && autoModelPattern.Match(bodyBytes) {
		bodyBytes, err = t.resolveAuto(req, bodyBytes)
		if err != nil {
			return nil, err
		}
	}

	// Restore the original body using the preserved bytes.
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	req.ContentLength = int64(len(bodyBytes))

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

// resolveAuto rewrites the request body's model field from "auto" to the
// session-granted model and sets the session token header.
func (t *initiatorTransport) resolveAuto(req *http.Request, body []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse request body for auto mode: %w", err)
	}
	if payload["model"] != AutoModelID {
		return body, nil
	}

	model, sessionToken, err := t.auto.resolve(req.Context())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Copilot auto model: %w", err)
	}
	slog.Debug("Resolved Copilot auto model", "model", model)

	payload["model"] = model
	req.Header.Set("Copilot-Session-Token", sessionToken)

	rewritten, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to rewrite request body for auto mode: %w", err)
	}
	return rewritten, nil
}

func (t *initiatorTransport) roundTrip(req *http.Request) (*http.Response, error) {
	if t.debug {
		return log.NewHTTPClient().Transport.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}
