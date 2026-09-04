package openai

import (
	"context"
	"net/http"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/oauth"
)

func init() {
	oauth.Register(&Provider{})
}

// Provider implements the generic oauth.Provider for OpenAI Codex.
type Provider struct{}

var (
	_ oauth.Provider            = (*Provider)(nil)
	_ oauth.BrowserFlowProvider = (*Provider)(nil)
)

func (p *Provider) ID() string {
	return string(catwalk.InferenceProviderOpenAI)
}

func (p *Provider) Name() string {
	return "OpenAI (ChatGPT / Codex)"
}

func (p *Provider) HasAuthChoices() bool {
	return true
}

func (p *Provider) SupportedFlows() oauth.FlowType {
	return oauth.FlowBrowser
}

func (p *Provider) RefreshToken(ctx context.Context, rt string) (*oauth.Token, error) {
	return RefreshToken(ctx, rt)
}

func (p *Provider) StartBrowserFlow(ctx context.Context) (oauth.BrowserSession, error) {
	return oauth.StartBrowserFlow(ctx, oauth.BrowserFlowConfig{
		Port:         CallbackPort,
		CallbackPath: "/auth/callback",
		Subject:      "OpenAI Codex",
		AuthURL:      AuthorizeURL,
		Exchange:     ExchangeCode,
	})
}

func (p *Provider) WrapClient(base *http.Client, token *oauth.Token, isSubAgent, debug bool) *http.Client {
	var rt http.RoundTripper
	if base != nil && base.Transport != nil {
		rt = base.Transport
	} else if debug {
		rt = log.NewHTTPClient().Transport
	} else {
		rt = http.DefaultTransport
	}

	transport := &Transport{
		Base:       rt,
		Token:      token,
		Originator: "crush",
	}

	return &http.Client{
		Transport: transport,
	}
}
