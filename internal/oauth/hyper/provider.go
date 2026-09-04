package hyper

import (
	"context"
	"errors"
	"net/http"

	"github.com/charmbracelet/crush/internal/oauth"
)

func init() {
	oauth.Register(&Provider{})
}

// Provider implements oauth.DeviceFlowProvider for Charm Hyper.
type Provider struct{}

var (
	_ oauth.Provider           = (*Provider)(nil)
	_ oauth.DeviceFlowProvider = (*Provider)(nil)
)

func (p *Provider) ID() string {
	return "hyper"
}

func (p *Provider) Name() string {
	return "Hyper"
}

func (p *Provider) HasAuthChoices() bool {
	return false
}

func (p *Provider) SupportedFlows() oauth.FlowType {
	return oauth.FlowDevice
}

func (p *Provider) RefreshToken(ctx context.Context, rt string) (*oauth.Token, error) {
	return ExchangeToken(ctx, rt)
}

func (p *Provider) RequestDeviceCode(ctx context.Context) (*oauth.DeviceCode, error) {
	resp, err := InitiateDeviceAuth(ctx)
	if err != nil {
		return nil, err
	}
	return &oauth.DeviceCode{
		DeviceCode:      resp.DeviceCode,
		UserCode:        resp.UserCode,
		VerificationURI: resp.VerificationURL,
		ExpiresIn:       resp.ExpiresIn,
	}, nil
}

func (p *Provider) PollForToken(ctx context.Context, code *oauth.DeviceCode) (*oauth.Token, error) {
	rt, err := PollForToken(ctx, code.DeviceCode, code.ExpiresIn)
	if err != nil {
		return nil, err
	}
	token, err := ExchangeToken(ctx, rt)
	if err != nil {
		return nil, err
	}
	introspect, err := IntrospectToken(ctx, token.AccessToken)
	if err != nil {
		return nil, err
	}
	if !introspect.Active {
		return nil, errors.New("access token is not active")
	}
	return token, nil
}

func (p *Provider) WrapClient(base *http.Client, token *oauth.Token, isSubAgent, debug bool) *http.Client {
	if base != nil {
		return base
	}
	return http.DefaultClient
}
