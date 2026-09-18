// Package ratelimit captures the rate-limit position providers report on every
// response.
//
// Providers say where the account stands on each successful call and Crush
// discards it, so the only way to learn a window is nearly spent is to hit it
// mid-turn. This is the capture half of #420: the numbers exist here now, and a
// surface can read them from LastKnown without another request.
//
// It follows the Prism router capture in internal/agent/hyper: an openai
// language-model header func copies the values into provider metadata, and the
// agent lifts them back out when a turn finishes.
package ratelimit

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy/providers/openai"
)

// Field is where a captured header lands in the openai provider metadata. One
// field holds the whole set, JSON-encoded, because the names differ per
// provider and reserving a metadata key per header would mean guessing them.
const Field = "x-crush-rate-limit"

// Snapshot is what a provider last said about the account.
//
// Values stay strings on purpose. OpenAI reports a remaining count, Anthropic's
// unified windows report a spent fraction, and resets arrive variously as
// seconds, as durations like "120ms", and as RFC3339 instants. Parsing them
// into one shape here would have to invent a meaning for each; a reader that
// knows its provider can parse what it needs.
type Snapshot struct {
	// Headers is every rate-limit header the provider sent, lower-cased.
	Headers map[string]string `json:"headers"`
	// ObservedAt is when the response carrying them arrived.
	ObservedAt time.Time `json:"observedAt"`
}

// Reported says whether the provider volunteered anything at all. An empty
// snapshot is not a full account -- it is an unasked question, and the
// difference matters to whatever draws it.
func (s Snapshot) Reported() bool { return len(s.Headers) > 0 }

// interesting decides which headers are worth keeping. It matches on meaning
// rather than on a list of names: OpenAI sends x-ratelimit-remaining-requests,
// Anthropic sends anthropic-ratelimit-unified-5h-utilization, and a gateway in
// between sends its own. Naming today's headers would silently drop tomorrow's.
func interesting(name string) bool {
	lowered := strings.ToLower(name)
	return strings.Contains(lowered, "ratelimit") ||
		strings.Contains(lowered, "rate-limit") ||
		lowered == "retry-after" ||
		lowered == "retry-after-ms"
}

// Capture returns the rate-limit headers from one response.
func Capture(header http.Header) map[string]string {
	if len(header) == 0 {
		return nil
	}
	found := map[string]string{}
	for name := range header {
		if !interesting(name) {
			continue
		}
		if value := header.Get(name); value != "" {
			found[strings.ToLower(name)] = value
		}
	}
	if len(found) == 0 {
		return nil
	}
	return found
}

var (
	mu       sync.RWMutex
	lastSeen = map[string]Snapshot{}
)

// Record stores what a provider last reported. Keyed by provider so two
// accounts in one session do not overwrite each other's reading.
func Record(providerID string, snapshot Snapshot) {
	if !snapshot.Reported() {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	lastSeen[providerID] = snapshot
}

// LastKnown returns the most recent reading for a provider.
func LastKnown(providerID string) (Snapshot, bool) {
	mu.RLock()
	defer mu.RUnlock()
	snapshot, ok := lastSeen[providerID]
	return snapshot, ok
}

// Reset drops every reading. For tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	lastSeen = map[string]Snapshot{}
}

// Encode renders a capture for storage in provider metadata.
func Encode(headers map[string]string, observedAt time.Time) (json.RawMessage, bool) {
	if len(headers) == 0 {
		return nil, false
	}
	raw, err := json.Marshal(Snapshot{Headers: headers, ObservedAt: observedAt})
	if err != nil {
		return nil, false
	}
	return raw, true
}

// Decode reads back what Encode wrote.
func Decode(raw json.RawMessage) (Snapshot, bool) {
	var snapshot Snapshot
	if len(raw) == 0 || json.Unmarshal(raw, &snapshot) != nil {
		return Snapshot{}, false
	}
	return snapshot, snapshot.Reported()
}

// HeaderFunc is the openai language-model header hook. It stores the captured
// headers under Field, following copyHeaderField in internal/agent/hyper.
func HeaderFunc(header http.Header, metadata *openai.ProviderMetadata) {
	raw, ok := Encode(Capture(header), time.Now())
	if !ok {
		return
	}
	if metadata.ExtraFields == nil {
		metadata.ExtraFields = make(map[string]json.RawMessage)
	}
	metadata.ExtraFields[Field] = raw
}
