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
	// KeyInfo identifies the key being used (user ID hint or key ID).
	KeyInfo string `json:"key_info,omitempty"`
	// Kind reports whether this is a passphrase or a security key PIN.
	Kind Kind `json:"kind"`
	// RetryCount is the number of previous bad attempts for this operation.
	RetryCount int `json:"retry_count,omitempty"`
	// Error describes why the previous attempt failed, if any.
	Error string `json:"error,omitempty"`
}

// Notification is published when a prompt is resolved so that clients
// which did not answer it can dismiss their open dialogs.
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
// Only one prompt can be pending at a time (GPG handles one credential
// per operation), so responses are correlated by the pending request ID.
// The secret is delivered to the single waiting caller and is never
// published on any broker.
type Prompts struct {
	broker             *pubsub.Broker[PromptRequest]
	notificationBroker *pubsub.Broker[Notification]

	mu         sync.Mutex
	pending    chan PromptResponse
	cancelled  chan struct{}
	pendingID  string
	prompterWG sync.WaitGroup
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

	s.mu.Lock()
	if s.pending != nil {
		s.mu.Unlock()
		return "", errors.New("another pinentry prompt is already pending")
	}
	respCh := make(chan PromptResponse, 1)
	cancelCh := make(chan struct{})
	s.pending = respCh
	s.cancelled = cancelCh
	s.pendingID = req.ID
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.pending = nil
		s.cancelled = nil
		s.pendingID = ""
		s.mu.Unlock()
	}()

	// Credential prompts are terminal events; losing one to a full
	// buffer would hang the GPG operation.
	s.broker.PublishMustDeliver(context.Background(), pubsub.CreatedEvent, req)

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-cancelCh:
		return "", ErrCancelled
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
