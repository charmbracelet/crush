package pinentry

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Kind identifies the credential a prompt is asking for.
type Kind string

const (
	// KindPassphrase is a secret key passphrase.
	KindPassphrase Kind = "passphrase"
	// KindPIN is a security key (smartcard) user PIN.
	KindPIN Kind = "pin"
)

// Key describes the key a credential prompt concerns, parsed from GPG's
// status output. Every field is optional: GPG reports only what it knows
// for the operation at hand.
type Key struct {
	// CardNumber is the smartcard serial number, when GPG reported one.
	CardNumber string `json:"card_number,omitempty"`
	// KeyID is the long key ID in hex, when GPG reported one.
	KeyID string `json:"key_id,omitempty"`
	// Fingerprint is the key fingerprint, when GPG reported one.
	Fingerprint string `json:"fingerprint,omitempty"`
	// Holder is the user ID (name and email), when GPG reported one.
	Holder string `json:"holder,omitempty"`
}

// Lines returns the key details as labeled rows, mirroring the card
// block a terminal pinentry shows. Empty when GPG reported nothing.
func (k Key) Lines() []string {
	var lines []string
	if k.CardNumber != "" {
		lines = append(lines, "Number: "+k.CardNumber)
	}
	if id := k.KeyID; id != "" {
		lines = append(lines, "Key:    "+id)
	} else if k.Fingerprint != "" {
		lines = append(lines, "Key:    "+k.Fingerprint)
	}
	if k.Holder != "" {
		lines = append(lines, "Holder: "+k.Holder)
	}
	return lines
}

// PromptRequest describes a GPG credential the user must provide. It is
// published over pubsub and rendered by the UI as a masked-input dialog.
// The resolved secret is never carried on this type.
type PromptRequest struct {
	// ID identifies the request; used to correlate the response.
	ID string `json:"id"`
	// SessionID is the session that triggered the GPG operation, when known.
	SessionID string `json:"session_id,omitempty"`
	// Prompt is the human-readable description of what is being unlocked.
	Prompt string `json:"prompt"`
	// Key identifies the key being unlocked, as far as GPG reported it.
	Key Key `json:"key,omitempty"`
	// Kind reports whether this is a passphrase or a security key PIN.
	Kind Kind `json:"kind"`
	// RetryCount is the number of previous bad attempts for this operation.
	RetryCount int `json:"retry_count,omitempty"`
	// Error describes why the previous attempt failed, if any.
	Error string `json:"error,omitempty"`
}

// Notification is published when a prompt is resolved (or abandoned by
// its GPG command) so that clients can dismiss the matching dialog.
type Notification struct {
	RequestID string `json:"request_id"`
}

// ErrCancelled is returned when the user cancelled the prompt.
var ErrCancelled = errors.New("pinentry prompt cancelled by user")

// ErrNoPrompter is returned when no one can answer prompts (no
// subscribers), so callers should fall back to plain GPG execution.
var ErrNoPrompter = errors.New("no pinentry prompt listener available")

// Prompts is the request/response credential prompt service. It mirrors
// the permission and question service patterns: publish a request over
// pubsub, block on a channel, and resolve when the UI responds.
//
// Only one prompt is pending at a time (the UI shows a single dialog);
// concurrent callers queue for their turn. Responses are correlated by
// the pending request ID. The secret is delivered to the single waiting
// caller and is never published on any broker.
type Prompts struct {
	broker             *pubsub.Broker[PromptRequest]
	notificationBroker *pubsub.Broker[Notification]

	// turn holds a token while a prompt is pending.
	turn chan struct{}

	mu        sync.Mutex
	pending   chan PromptResponse
	pendingID string
}

// PromptResponse resolves a pending prompt. Exactly one of Secret or
// Cancelled is meaningful.
type PromptResponse struct {
	// ID of the request being resolved.
	ID string `json:"id"`
	// Secret is the user-provided credential.
	Secret string `json:"secret,omitempty"`
	// Cancelled reports the user dismissed the prompt.
	Cancelled bool `json:"cancelled,omitempty"`
}

// NewPrompts creates a new prompt service.
func NewPrompts() *Prompts {
	return &Prompts{
		broker:             pubsub.NewBroker[PromptRequest](),
		notificationBroker: pubsub.NewBroker[Notification](),
		turn:               make(chan struct{}, 1),
	}
}

var defaultPrompts = NewPrompts()

// DefaultPrompts returns the process-wide prompt service. The shell
// exec layer uses it to intercept GPG commands, and the app forwards
// its events to the UI.
func DefaultPrompts() *Prompts {
	return defaultPrompts
}

// Subscribe returns a channel for prompt requests.
func (s *Prompts) Subscribe(ctx context.Context) <-chan pubsub.Event[PromptRequest] {
	return s.broker.Subscribe(ctx)
}

// SubscribeNotifications returns a channel for prompt resolution
// notifications.
func (s *Prompts) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[Notification] {
	return s.notificationBroker.Subscribe(ctx)
}

// Prompt publishes a credential request and blocks until the user
// answers, cancels, or the context is cancelled.
func (s *Prompts) Prompt(ctx context.Context, req PromptRequest) (string, error) {
	// Without a listener the prompt would hang until the context dies;
	// callers use ErrNoPrompter to fall back to plain GPG execution.
	if s.broker.GetSubscriberCount() == 0 {
		return "", ErrNoPrompter
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Kind == "" {
		req.Kind = KindPassphrase
	}

	// Concurrent GPG operations wait their turn instead of failing.
	select {
	case s.turn <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-s.turn }()

	respCh := make(chan PromptResponse, 1)
	s.mu.Lock()
	s.pending = respCh
	s.pendingID = req.ID
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.pending = nil
		s.pendingID = ""
		s.mu.Unlock()
	}()

	// Credential prompts are terminal events; losing one to a full
	// buffer would hang the GPG operation. A retry request carries the
	// reason the previous attempt failed in its Error field.
	s.broker.PublishMustDeliver(context.Background(), pubsub.CreatedEvent, req)

	select {
	case <-ctx.Done():
		// The GPG command went away unanswered (cancelled or timed
		// out): let clients dismiss the now-stale dialog.
		s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{RequestID: req.ID})
		return "", ctx.Err()
	case resp := <-respCh:
		if resp.Cancelled {
			return "", ErrCancelled
		}
		return resp.Secret, nil
	}
}

// Respond resolves the pending prompt with a secret. Returns false if
// no matching prompt is pending.
func (s *Prompts) Respond(id, secret string) bool {
	return s.resolve(PromptResponse{ID: id, Secret: secret})
}

// Cancel dismisses the pending prompt. Returns false if no matching
// prompt is pending.
func (s *Prompts) Cancel(id string) bool {
	return s.resolve(PromptResponse{ID: id, Cancelled: true})
}

// resolve delivers a response to the pending prompt and publishes a
// notification so non-answering clients can dismiss their dialogs.
func (s *Prompts) resolve(resp PromptResponse) bool {
	s.mu.Lock()
	pendingID := s.pendingID
	ch := s.pending
	s.mu.Unlock()

	if ch == nil || pendingID != resp.ID {
		return false
	}

	select {
	case ch <- resp:
	default:
		return false
	}

	s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{
		RequestID: resp.ID,
	})
	return true
}

// passphraseCache is an optional in-memory credential cache, opt-in via
// the pinentry.cache_timeout config option. Entries are zeroed and
// dropped when they expire. It is keyed by key info so a cached
// credential is never reused for a different key.
type passphraseCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

type cacheEntry struct {
	secret  []byte
	expires time.Time
}

func newPassphraseCache(ttl time.Duration) *passphraseCache {
	return &passphraseCache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

func (c *passphraseCache) get(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return "", false
	}
	if time.Now().After(e.expires) {
		delete(c.entries, key)
		zeroBytes(e.secret)
		return "", false
	}
	return string(e.secret), true
}

func (c *passphraseCache) set(key, secret string) {
	if c == nil || key == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.entries[key]; ok {
		zeroBytes(old.secret)
	}
	c.entries[key] = cacheEntry{
		secret:  []byte(secret),
		expires: time.Now().Add(c.ttl),
	}
}

// forget zeroes and drops the entry for key, if any.
func (c *passphraseCache) forget(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		zeroBytes(e.secret)
		delete(c.entries, key)
	}
}

func (c *passphraseCache) clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.entries {
		zeroBytes(e.secret)
		delete(c.entries, k)
	}
}

// zeroBytes best-effort zeroes a secret buffer.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
