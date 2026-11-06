package agent

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
)

// hyperCreditsModel wraps a Hyper language model so that every request
// also refreshes the remaining hypercredit balance from the /v1/credits
// endpoint. Hyper completions no longer carry the balance, so it has to
// be fetched separately. Refreshes run in the background so they never
// add latency to a turn, and only one runs at a time: refreshes requested
// while one is in flight are coalesced into a single follow-up fetch, so
// the balance still reflects every request that finished meanwhile.
type hyperCreditsModel struct {
	fantasy.LanguageModel
	apiKey func() string
	fetch  func(context.Context, string) (*int, error)

	mu       sync.Mutex
	inflight bool
	pending  bool
}

// newHyperCreditsModel wraps m so every request refreshes the
// hypercredit balance once it finishes. apiKey resolves the Hyper API key
// at fetch time. A nil model or resolver returns m unchanged.
func newHyperCreditsModel(m fantasy.LanguageModel, apiKey func() string) fantasy.LanguageModel {
	if m == nil || apiKey == nil {
		return m
	}
	return &hyperCreditsModel{
		LanguageModel: m,
		apiKey:        apiKey,
		fetch:         hyper.FetchCredits,
	}
}

// Generate implements [fantasy.LanguageModel].
func (m *hyperCreditsModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	resp, err := m.LanguageModel.Generate(ctx, call)
	if err == nil {
		m.refreshBalance()
	}
	return resp, err
}

// Stream implements [fantasy.LanguageModel]. The stream is consumed after
// Stream returns, so the refresh has to wait for the iteration to end:
// the balance changes only once the request has been served.
func (m *hyperCreditsModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	stream, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		defer m.refreshBalance()
		stream(yield)
	}, nil
}

// refreshBalance starts a background fetch of the hypercredit balance, or
// marks one pending when a fetch is already running.
func (m *hyperCreditsModel) refreshBalance() {
	m.mu.Lock()
	if m.inflight {
		m.pending = true
		m.mu.Unlock()
		return
	}
	m.inflight = true
	m.mu.Unlock()

	go m.fetchBalance()
}

// fetchBalance fetches the balance on a detached context so a cancelled
// or finished run still lands its final refresh. Fetch failures are
// logged and otherwise ignored: a stale balance beats no balance.
func (m *hyperCreditsModel) fetchBalance() {
	if apiKey := m.apiKey(); apiKey != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if _, err := m.fetch(ctx, apiKey); err != nil {
			slog.Warn("Failed to fetch Hyper credits", "error", err)
		}
		cancel()
	}

	m.mu.Lock()
	m.inflight = false
	pending := m.pending
	m.pending = false
	m.mu.Unlock()

	if pending {
		m.refreshBalance()
	}
}
