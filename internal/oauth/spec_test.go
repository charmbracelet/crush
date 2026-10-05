package oauth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthSpecKindAndFlow(t *testing.T) {
	t.Parallel()

	// A spec with no kind is an OAuth spec: the block's presence is the
	// signal that the provider signs in rather than takes a key.
	require.True(t, (&AuthSpec{}).UsesOAuth())
	require.True(t, (&AuthSpec{Kind: "OAuth"}).UsesOAuth())
	require.False(t, (&AuthSpec{Kind: AuthKindAPIKey}).UsesOAuth())
	require.False(t, (*AuthSpec)(nil).UsesOAuth())

	require.Equal(t, AuthFlowAuto, (&AuthSpec{}).FlowMode())
	require.Equal(t, AuthFlowBrowser, (&AuthSpec{Flow: "authorization_code"}).FlowMode())
	require.Equal(t, AuthFlowDevice, (&AuthSpec{Flow: "device_code"}).FlowMode())
	require.Equal(t, AuthFlowAuto, (*AuthSpec)(nil).FlowMode())
}

func TestAuthSpecEndpoints(t *testing.T) {
	t.Parallel()

	// Explicit endpoints are enough even without an issuer.
	require.True(t, (&AuthSpec{TokenURL: "https://auth.example.com/token"}).Usable())
	// So is an issuer to discover them from.
	require.True(t, (&AuthSpec{Issuer: "https://auth.example.com"}).Usable())

	// A client ID alone names no authorization server, so no flow can run.
	require.False(t, (&AuthSpec{ClientID: "crush"}).Usable())
	require.False(t, (*AuthSpec)(nil).Usable())
}
