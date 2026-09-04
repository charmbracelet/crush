package oauth_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	id      string
	name    string
	choices bool
	flows   oauth.FlowType
}

func (m *mockProvider) ID() string                     { return m.id }
func (m *mockProvider) Name() string                   { return m.name }
func (m *mockProvider) HasAuthChoices() bool           { return m.choices }
func (m *mockProvider) SupportedFlows() oauth.FlowType { return m.flows }
func (m *mockProvider) RefreshToken(ctx context.Context, rt string) (*oauth.Token, error) {
	return &oauth.Token{AccessToken: "refreshed-" + rt}, nil
}
func (m *mockProvider) WrapClient(base *http.Client, token *oauth.Token, isSubAgent, debug bool) *http.Client {
	return base
}

func TestRegistry(t *testing.T) {
	mock := &mockProvider{
		id:      "test-prov",
		name:    "Test Provider",
		choices: true,
		flows:   oauth.FlowBrowser,
	}

	oauth.Register(mock)

	require.True(t, oauth.IsSupported("test-prov"))
	require.True(t, oauth.HasAuthChoices("test-prov"))

	p := oauth.Get("test-prov")
	require.NotNil(t, p)
	require.Equal(t, "test-prov", p.ID())
	require.Equal(t, "Test Provider", p.Name())

	tok, err := p.RefreshToken(context.Background(), "my-rt")
	require.NoError(t, err)
	require.Equal(t, "refreshed-my-rt", tok.AccessToken)

	require.False(t, oauth.IsSupported("non-existent"))
	require.False(t, oauth.HasAuthChoices("non-existent"))
}
