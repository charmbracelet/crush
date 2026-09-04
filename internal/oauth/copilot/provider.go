package copilot

import (
	"context"
	"net/http"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/oauth"
)

func init() {
	oauth.Register(&Provider{})
}

// Provider implements oauth.DeviceFlowProvider for GitHub Copilot.
type Provider struct{}

var (
	_ oauth.Provider           = (*Provider)(nil)
	_ oauth.DeviceFlowProvider = (*Provider)(nil)
)

func (p *Provider) ID() string {
	return string(catwalk.InferenceProviderCopilot)
}

func (p *Provider) Name() string {
	return "GitHub Copilot"
}

func (p *Provider) HasAuthChoices() bool {
	return false
}

func (p *Provider) SupportedFlows() oauth.FlowType {
	return oauth.FlowDevice
}

func (p *Provider) RefreshToken(ctx context.Context, rt string) (*oauth.Token, error) {
	return RefreshToken(ctx, rt)
}

func (p *Provider) RequestDeviceCode(ctx context.Context) (*oauth.DeviceCode, error) {
	dc, err := RequestDeviceCode(ctx)
	if err != nil {
		return nil, err
	}
	return &oauth.DeviceCode{
		DeviceCode:      dc.DeviceCode,
		UserCode:        dc.UserCode,
		VerificationURI: dc.VerificationURI,
		ExpiresIn:       dc.ExpiresIn,
		Interval:        dc.Interval,
	}, nil
}

func (p *Provider) PollForToken(ctx context.Context, code *oauth.DeviceCode) (*oauth.Token, error) {
	dc := &DeviceCode{
		DeviceCode:      code.DeviceCode,
		UserCode:        code.UserCode,
		VerificationURI: code.VerificationURI,
		ExpiresIn:       code.ExpiresIn,
		Interval:        code.Interval,
	}
	return PollForToken(ctx, dc)
}

func (p *Provider) WrapClient(base *http.Client, token *oauth.Token, isSubAgent, debug bool) *http.Client {
	return NewClient(isSubAgent, debug)
}
