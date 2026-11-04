package generic

import (
	"context"
	"errors"
	"log/slog"

	"github.com/charmbracelet/crush/internal/oauth"
)

// errNoSpec is returned when a flow is asked to run for a provider that
// carries no OAuth configuration.
var errNoSpec = errors.New("provider has no OAuth configuration")

// Flow is the interactive authorization surface that the login command and
// the in-app dialogs drive. It has the same shape as the built-in
// providers' flows, so spec-driven providers need no special handling
// downstream.
type Flow interface {
	Start(ctx context.Context) (url string, userCode string, err error)
	Wait(ctx context.Context) (*oauth.Token, error)
	Close()
}

var (
	_ Flow = (*BrowserFlow)(nil)
	_ Flow = (*DeviceFlow)(nil)
)

// StartFlow begins the flow the spec selects. In auto mode the browser
// flow is preferred; when its loopback listener cannot be opened, which is
// typical on a remote machine without port forwarding, the device flow
// takes over instead.
func StartFlow(spec *oauth.AuthSpec, subject string) (Flow, error) {
	if spec == nil {
		return nil, errNoSpec
	}

	switch spec.FlowMode() {
	case oauth.AuthFlowDevice:
		return StartDevice(spec), nil
	case oauth.AuthFlowBrowser:
		return StartBrowser(spec, subject)
	default:
		browser, err := StartBrowser(spec, subject)
		if err == nil {
			return browser, nil
		}
		slog.Info(
			"Falling back to the device flow because the browser callback listener could not start",
			"error", err,
		)
		return StartDevice(spec), nil
	}
}
