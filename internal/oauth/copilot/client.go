// Package copilot provides GitHub Copilot integration.
package copilot

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/charmbracelet/crush/internal/log"
)

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

	// Priority 1: Check context value (allows explicit control).
	if initiator, ok := contextInitiator(req.Context()); ok {
		req.Header.Set(xInitiatorHeader, initiator)
		if t.debug {
			slog.Debug("Setting X-Initiator header from context", "value", initiator)
		}
		return t.roundTrip(req)
	}

	// Priority 2: Check isSubAgent flag (deprecated but kept for compatibility).
	if t.isSubAgent {
		req.Header.Set(xInitiatorHeader, agentInitiator)
		if t.debug {
			slog.Debug("Setting X-Initiator header to agent (isSubAgent flag)")
		}
		return t.roundTrip(req)
	}

	if req.Body == nil || req.Body == http.NoBody {
		// No body to inspect; default to user.
		req.Header.Set(xInitiatorHeader, userInitiator)
		if t.debug {
			slog.Debug("Setting X-Initiator header to user (no request body)")
		}
		return t.roundTrip(req)
	}

	bodyBytes, err := readAndRestoreRequestBody(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	initiator := detectInitiatorFromBody(bodyBytes)
	if initiator == "" {
		initiator = userInitiator
	}
	req.Header.Set(xInitiatorHeader, initiator)

	if t.debug {
		slog.Debug("Setting X-Initiator header from request body", "value", initiator)
	}

	return t.roundTrip(req)
}

func (t *initiatorTransport) roundTrip(req *http.Request) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)
	if t.debug {
		resp, err = log.NewHTTPClient().Transport.RoundTrip(req)
	} else {
		resp, err = http.DefaultTransport.RoundTrip(req)
	}
	if err != nil {
		return resp, err
	}
	return wrapReasoningTransform(resp), nil
}
