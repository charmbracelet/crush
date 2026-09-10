package keyring

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	zkeyring "github.com/zalando/go-keyring"
)

func TestRefAndParseRef(t *testing.T) {
	require.Equal(t, "keychain://openai", Ref("openai"))

	id, ok := ParseRef("keychain://openai")
	require.True(t, ok)
	require.Equal(t, "openai", id)

	_, ok = ParseRef("sk-literal")
	require.False(t, ok)

	_, ok = ParseRef("$OPENAI_API_KEY")
	require.False(t, ok)

	_, ok = ParseRef("keychain://")
	require.False(t, ok)

	require.True(t, IsRef("keychain://openai"))
	require.False(t, IsRef("see keychain://openai for details"))
}

func TestSetGetDeleteRoundTrip(t *testing.T) {
	MockInit()
	ResetAvailableCache()

	require.NoError(t, Set("testprovider", "secret-value"))

	got, err := Get("testprovider")
	require.NoError(t, err)
	require.Equal(t, "secret-value", got)

	require.NoError(t, Delete("testprovider"))

	_, err = Get("testprovider")
	require.ErrorIs(t, err, zkeyring.ErrNotFound)

	require.NoError(t, Delete("testprovider"), "deleting a missing entry must not fail")
}

func TestAvailableProbesBackend(t *testing.T) {
	MockInit()
	ResetAvailableCache()
	require.True(t, Available())

	MockInitWithError(zkeyring.ErrUnsupportedPlatform)
	ResetAvailableCache()
	require.False(t, Available())

	MockInitWithError(errors.New("dbus connection refused"))
	ResetAvailableCache()
	require.False(t, Available())
}

func TestUnavailableErrorsAreClassified(t *testing.T) {
	require.True(t, IsUnavailable(ErrUnavailable))
	require.True(t, IsUnavailable(zkeyring.ErrUnsupportedPlatform))
	wrapped := fmt.Errorf("read failed: %w", ErrUnavailable)
	require.True(t, IsUnavailable(wrapped))
	require.False(t, IsUnavailable(zkeyring.ErrNotFound))
}

func TestCallTimesOutOnWedgedBackend(t *testing.T) {
	originalTimeout := opTimeout
	opTimeout = 10 * time.Millisecond
	defer func() { opTimeout = originalTimeout }()

	_, err := call(func() (string, error) {
		time.Sleep(200 * time.Millisecond)
		return "late secret", nil
	})
	require.ErrorIs(t, err, ErrUnavailable)
}
