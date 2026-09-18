package config

import (
	"testing"

	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/keyring"
	"github.com/stretchr/testify/require"
)

func TestShellVariableResolver_KeychainRef(t *testing.T) {
	keyring.MockInit()
	keyring.ResetAvailableCache()
	require.NoError(t, keyring.Set("testprovider", "secret-value"))

	r := NewShellVariableResolver(env.New())

	got, err := r.ResolveValue(keyring.Ref("testprovider"))
	require.NoError(t, err)
	require.Equal(t, "secret-value", got)
}

func TestShellVariableResolver_KeychainRefMissingEntry(t *testing.T) {
	keyring.MockInit()
	keyring.ResetAvailableCache()

	r := NewShellVariableResolver(env.New())

	_, err := r.ResolveValue(keyring.Ref("testprovider"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "keychain://testprovider")
	require.NotContains(t, err.Error(), "secret-value", "the missing secret must not be leaked")
}

func TestShellVariableResolver_KeychainRefInsideLargerStringIsLiteral(t *testing.T) {
	keyring.MockInit()
	keyring.ResetAvailableCache()
	require.NoError(t, keyring.Set("testprovider", "secret-value"))

	r := NewShellVariableResolver(env.New())

	got, err := r.ResolveValue("service=keychain://testprovider and more")
	require.NoError(t, err)
	require.Equal(t, "service=keychain://testprovider and more", got)
}
