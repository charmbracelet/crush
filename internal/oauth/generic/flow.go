package generic

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/crush/internal/oauth"
)

// errNoSpec is returned when a flow is asked to run for a provider that
// carries no OAuth configuration.
var errNoSpec = errors.New("provider has no OAuth configuration")

// Flow is a started authorization the login command and the in-app dialogs
// wait on. Its prompt comes from [StartFlow], so callers never need to know
// which grant is running.
type Flow interface {
	Wait(ctx context.Context) (*oauth.Token, error)
	Close()
}

// CodeCompleter is implemented by flows that can also finish with a code
// pasted back from the authorization page, which only the browser flow can.
type CodeCompleter interface {
	CompleteWithCode(ctx context.Context, input string) (*oauth.Token, error)
}

var (
	_ Flow          = (*BrowserFlow)(nil)
	_ Flow          = (*DeviceFlow)(nil)
	_ CodeCompleter = (*BrowserFlow)(nil)
)

// Prompt is what the user needs to start authorizing. UserCode and
// DeviceCode are set only for the device grant.
type Prompt struct {
	URL        string
	UserCode   string
	DeviceCode string
	ExpiresIn  int
	Interval   int
}

// StartFlow begins the flow the spec selects and returns it with the prompt
// to show. In auto mode the endpoints are resolved once and the browser flow
// is preferred; the device flow takes over, reusing them, when the server
// offers no authorization endpoint or the loopback listener cannot be opened,
// as on a remote machine. A browser flow that started is never replaced:
// a failed or denied authorization surfaces from Wait.
func StartFlow(ctx context.Context, spec *oauth.AuthSpec, subject string) (Flow, Prompt, error) {
	if spec == nil {
		return nil, Prompt{}, errNoSpec
	}

	switch spec.FlowMode() {
	case oauth.AuthFlowDevice:
		return startDevice(ctx, spec, Endpoints{})
	case oauth.AuthFlowBrowser:
		return startBrowser(ctx, spec, subject, Endpoints{})
	}

	endpoints, err := Resolve(ctx, spec, GrantAuto)
	if err != nil {
		return nil, Prompt{}, err
	}
	var browserErr error
	if endpoints.AuthorizeURL != "" {
		flow, prompt, err := startBrowser(ctx, spec, subject, endpoints)
		if err == nil {
			return flow, prompt, nil
		}
		browserErr = err
	}
	if endpoints.DeviceAuthURL == "" {
		return nil, Prompt{}, errors.Join(browserErr, errors.New("the device grant is unavailable: no device_auth_url"))
	}
	flow, prompt, err := startDevice(ctx, spec, endpoints)
	if err != nil {
		return nil, Prompt{}, errors.Join(browserErr, err)
	}
	if browserErr != nil {
		slog.Info("Fell back to the device flow", "error", browserErr)
	}
	return flow, prompt, nil
}

// startBrowser opens the callback listener and returns the started flow.
func startBrowser(ctx context.Context, spec *oauth.AuthSpec, subject string, endpoints Endpoints) (Flow, Prompt, error) {
	flow, err := newBrowserFlow(spec, subject, endpoints)
	if err != nil {
		return nil, Prompt{}, err
	}
	prompt, err := flow.Start(ctx)
	if err != nil {
		flow.Close()
		return nil, Prompt{}, fmt.Errorf("start browser flow: %w", err)
	}
	return flow, prompt, nil
}

// startDevice requests a device authorization and returns the started flow.
func startDevice(ctx context.Context, spec *oauth.AuthSpec, endpoints Endpoints) (Flow, Prompt, error) {
	flow := &DeviceFlow{spec: spec, endpoints: endpoints}
	prompt, err := flow.Start(ctx)
	if err != nil {
		return nil, Prompt{}, err
	}
	return flow, prompt, nil
}
