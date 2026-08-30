package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegisterFactory(t *testing.T) {
	name := "test-provider"
	factory := func(json.RawMessage) (Client, error) { return nil, nil }
	Register(name, factory)
	got, ok := FactoryFor(name)
	require.True(t, ok)
	require.NotNil(t, got)
}

func TestProviderError(t *testing.T) {
	err := &ProviderError{Message: "rate limited", StatusCode: 429, Retryable: true}
	require.EqualError(t, err, "rate limited")
}
