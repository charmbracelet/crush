package ratelimit

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"charm.land/fantasy/providers/openai"
	"github.com/stretchr/testify/require"
)

func TestCaptureKeepsEveryProvidersSpelling(t *testing.T) {
	// The three families seen in the wild. Matching a list of names would have
	// kept the first and dropped the rest.
	header := http.Header{}
	header.Set("X-RateLimit-Remaining-Requests", "499")
	header.Set("X-RateLimit-Reset-Tokens", "254ms")
	header.Set("anthropic-ratelimit-unified-5h-utilization", "0.42")
	header.Set("anthropic-ratelimit-unified-5h-reset", "2026-09-17T09:13:32Z")
	header.Set("Retry-After", "17")
	header.Set("some-gateway-rate-limit-left", "80%")
	header.Set("Content-Type", "text/event-stream")
	header.Set("Date", "Thu, 17 Sep 2026 06:11:23 GMT")

	require.Equal(t, map[string]string{
		"x-ratelimit-remaining-requests":             "499",
		"x-ratelimit-reset-tokens":                   "254ms",
		"anthropic-ratelimit-unified-5h-utilization": "0.42",
		"anthropic-ratelimit-unified-5h-reset":       "2026-09-17T09:13:32Z",
		"retry-after":                                "17",
		"some-gateway-rate-limit-left":               "80%",
	}, Capture(header))
}

func TestCaptureReportsNothingRatherThanAnEmptySet(t *testing.T) {
	// An empty map and "the provider said nothing" must not read the same: a
	// caller drawing a meter from the first would show a full account.
	require.Nil(t, Capture(nil))

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	require.Nil(t, Capture(header))
}

func TestHeaderFuncStoresAndTheAgentReadsItBack(t *testing.T) {
	header := http.Header{}
	header.Set("x-ratelimit-remaining-requests", "499")

	metadata := &openai.ProviderMetadata{}
	HeaderFunc(header, metadata)

	snapshot, ok := Decode(metadata.ExtraFields[Field])
	require.True(t, ok)
	require.Equal(t, "499", snapshot.Headers["x-ratelimit-remaining-requests"])
	require.False(t, snapshot.ObservedAt.IsZero(), "the reading needs its own time; a stale one must be visible as stale")
}

func TestHeaderFuncLeavesMetadataAloneWhenNothingWasReported(t *testing.T) {
	header := http.Header{}
	header.Set("content-type", "application/json")

	metadata := &openai.ProviderMetadata{}
	HeaderFunc(header, metadata)

	require.Empty(t, metadata.ExtraFields, "an unreported limit must not create a field for something to draw")
}

func TestHeaderFuncKeepsFieldsAnotherHookAlreadyWrote(t *testing.T) {
	// Hyper's router capture and this one run against the same metadata.
	metadata := &openai.ProviderMetadata{}
	metadata.ExtraFields = map[string]json.RawMessage{"x-prism-model-name": json.RawMessage(`"some-model"`)}

	header := http.Header{}
	header.Set("x-ratelimit-remaining-requests", "499")
	HeaderFunc(header, metadata)

	require.Contains(t, metadata.ExtraFields, "x-prism-model-name")
	require.Contains(t, metadata.ExtraFields, Field)
}

func TestLastKnownIsPerProvider(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	now := time.Now()
	Record("anthropic", Snapshot{Headers: map[string]string{"anthropic-ratelimit-unified-5h-utilization": "0.42"}, ObservedAt: now})
	Record("openai", Snapshot{Headers: map[string]string{"x-ratelimit-remaining-requests": "499"}, ObservedAt: now})

	anthropic, ok := LastKnown("anthropic")
	require.True(t, ok)
	require.Equal(t, "0.42", anthropic.Headers["anthropic-ratelimit-unified-5h-utilization"])

	openaiSnapshot, ok := LastKnown("openai")
	require.True(t, ok)
	require.Equal(t, "499", openaiSnapshot.Headers["x-ratelimit-remaining-requests"])

	_, ok = LastKnown("never-called")
	require.False(t, ok, "a provider that has not answered has no reading, which is not the same as a zeroed one")
}

func TestRecordIgnoresAnEmptyReading(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	Record("openai", Snapshot{Headers: map[string]string{"x-ratelimit-remaining-requests": "499"}, ObservedAt: time.Now()})
	// A later call that reported nothing must not erase what is known.
	Record("openai", Snapshot{})

	snapshot, ok := LastKnown("openai")
	require.True(t, ok)
	require.Equal(t, "499", snapshot.Headers["x-ratelimit-remaining-requests"])
}
