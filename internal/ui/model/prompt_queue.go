package model

import "slices"

// promptQueue is what the queue pill shows: the last fetched server queue
// plus prompts submitted behind a busy turn that no fetch has reported yet.
// Equal-text prompts match one to one, so a pending prompt that repeats one
// already queued may briefly under-count until the next fetch.
type promptQueue struct {
	fetched []string
	pending []pendingPrompt
	nextID  uint64
}

// pendingPrompt is a submitted prompt the server queue may not hold yet.
type pendingPrompt struct {
	id      uint64
	prompt  string
	settled bool
}

// Items returns the fetched queue followed by the unreported pending prompts.
func (q *promptQueue) Items() []string {
	if len(q.pending) == 0 {
		return q.fetched
	}
	items := slices.Clone(q.fetched)
	for _, entry := range q.pending {
		items = append(items, entry.prompt)
	}
	return items
}

// Shown returns the number of prompts the pill shows.
func (q *promptQueue) Shown() int {
	return len(q.fetched) + len(q.pending)
}

// ServerQueued reports whether the server queue holds any prompts. Pending
// submissions do not count: there is nothing on the server to clear yet.
func (q *promptQueue) ServerQueued() bool {
	return len(q.fetched) > 0
}

// SetFetched stores a fetched queue. Settled pending prompts are dropped,
// as is any pending prompt the fetch reports, since the server now owns it.
func (q *promptQueue) SetFetched(fetched []string) {
	q.fetched = fetched
	unmatched := make(map[string]int, len(fetched))
	for _, prompt := range fetched {
		unmatched[prompt]++
	}
	q.pending = slices.DeleteFunc(q.pending, func(entry pendingPrompt) bool {
		if unmatched[entry.prompt] > 0 {
			unmatched[entry.prompt]--
			return true
		}
		return entry.settled
	})
}

// ClearServer empties the server side after the queue was cleared, along
// with settled submissions.
func (q *promptQueue) ClearServer() {
	q.SetFetched(nil)
}

// Add records a prompt being submitted behind a busy turn and returns its id.
func (q *promptQueue) Add(prompt string) uint64 {
	q.nextID++
	q.pending = append(q.pending, pendingPrompt{id: q.nextID, prompt: prompt})
	return q.nextID
}

// Settle marks a submission as finished. It keeps showing until the next
// fetch, which reports it if the server queued it.
func (q *promptQueue) Settle(id uint64) {
	for idx := range q.pending {
		if q.pending[idx].id == id {
			q.pending[idx].settled = true
		}
	}
}

// Reset empties the queue, pending submissions included.
func (q *promptQueue) Reset() {
	q.fetched = nil
	q.pending = nil
}
