// Package sshaskpass renders OpenSSH credential and confirmation
// prompts as native Crush dialogs. Commands Crush spawns (ssh, scp,
// sftp, ssh-add, or anything that shells out to OpenSSH such as
// git-over-ssh or rsync) run with SSH_ASKPASS pointed at a hidden Crush
// subcommand that forwards the prompt over a Unix socket to the Crush
// process that owns the terminal; the UI shows a masked password dialog
// or, for confirmations (security key touches, host key acceptance), a
// confirm dialog.
package sshaskpass

import (
	"context"
	"errors"
	"sync"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Kind identifies the kind of askpass question.
type Kind string

const (
	// KindPassword is an SSH authentication password or private key
	// passphrase, collected in a masked input.
	KindPassword Kind = "password"
	// KindConfirm is a decision question that must not be answered
	// automatically: a host key acceptance or an ssh-add key use
	// confirmation. The dialog submits "yes" to confirm and cancels to
	// decline.
	KindConfirm Kind = "confirm"
	// KindTouch is a passive security key touch (FIDO user presence, a
	// "touch your device" instruction). The physical touch is the
	// approval, so the UI just surfaces a warning and confirms
	// automatically; no input is collected.
	KindTouch Kind = "touch"
)

// PromptRequest describes a credential or confirmation the user must
// provide. It is published over pubsub and rendered by the UI as a
// masked-input or confirm dialog, or auto-confirmed as a touch warning.
// The resolved secret is never carried on this type.
type PromptRequest struct {
	// ID identifies the request; used to correlate the response.
	ID string `json:"id"`
	// Prompt is the phrasing OpenSSH used (e.g. "user@host's
	// password: " or "Confirm user presence for key ...").
	Prompt string `json:"prompt"`
	// KeyInfo is a short hint identifying the account or key path, when
	// it can be parsed from the prompt.
	KeyInfo string `json:"key_info,omitempty"`
	// Kind reports whether this is a password or a confirmation.
	Kind Kind `json:"kind"`
}

// Notification is published when a prompt is resolved so that clients
// which did not answer it can dismiss their open dialogs.
type Notification struct {
	RequestID string `json:"request_id"`
}

// ErrCancelled is returned when the user cancelled the prompt.
var ErrCancelled = errors.New("ssh prompt cancelled by user")

// ErrNoPrompter is returned when no one can answer prompts (no
// subscribers), so callers should abort cleanly instead of hanging.
var ErrNoPrompter = errors.New("no ssh prompt listener available")

// Prompts is the request/response credential prompt service. It mirrors
// the permission and question service patterns: publish a request over
// pubsub, block on a channel, and resolve when the UI responds.
//
// Only one prompt can be pending at a time (OpenSSH asks for one
// credential per operation), so responses are correlated by the pending
// request ID. The secret is delivered to the single waiting caller and
// is never published on any broker.
type Prompts struct {
	broker             *pubsub.Broker[PromptRequest]
	notificationBroker *pubsub.Broker[Notification]

	mu        sync.Mutex
	pending   chan PromptResponse
	cancelled chan struct{}
	pendingID string
}

// PromptResponse resolves a pending prompt. Exactly one of Secret or
// Cancelled is meaningful.
type PromptResponse struct {
	// ID of the request being resolved.
	ID string `json:"id"`
	// Secret is the user-provided credential, or "yes" for confirmations.
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

// DefaultPrompts returns the process-wide prompt service. The ssh
// askpass subprocess uses it over the socket, and the app forwards its
// events to the UI.
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
	// callers use ErrNoPrompter to abort cleanly instead.
	if s.broker.GetSubscriberCount() == 0 {
		return "", ErrNoPrompter
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Kind == "" {
		req.Kind = KindPassword
	}

	s.mu.Lock()
	if s.pending != nil {
		s.mu.Unlock()
		return "", errors.New("another ssh prompt is already pending")
	}
	respCh := make(chan PromptResponse, 1)
	cancelCh := make(chan struct{})
	s.pending = respCh
	s.cancelled = cancelCh
	s.pendingID = req.ID
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		// Only clear our own slot; a follow-up prompt (e.g. a retry)
		// may already occupy it by the time this caller unwinds.
		if s.pendingID == req.ID {
			s.pending = nil
			s.cancelled = nil
			s.pendingID = ""
		}
		s.mu.Unlock()
	}()

	// Credential prompts are terminal events; losing one to a full
	// buffer would hang the ssh command.
	s.broker.PublishMustDeliver(context.Background(), pubsub.CreatedEvent, req)

	select {
	case resp := <-respCh:
		if resp.Cancelled {
			return "", ErrCancelled
		}
		return resp.Secret, nil
	case <-cancelCh:
		return "", ErrCancelled
	case <-ctx.Done():
		s.notify(req.ID)
		return "", ctx.Err()
	}
}

// notify publishes a resolution so other subscribers close their
// dialogs. Safe to call from Respond/Cancel under s.mu because the
// notification broker is independent.
func (s *Prompts) notify(id string) {
	s.notificationBroker.Publish(pubsub.UpdatedEvent, Notification{RequestID: id})
}

// Respond resolves the pending prompt with the user's secret. Returns
// false when no matching prompt is pending.
func (s *Prompts) Respond(id, secret string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil || s.pendingID != id {
		return false
	}
	s.pending <- PromptResponse{ID: id, Secret: secret}
	s.notify(id)
	return true
}

// Cancel dismisses the pending prompt. Returns false when no matching
// prompt is pending.
func (s *Prompts) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelled == nil || s.pendingID != id {
		return false
	}
	close(s.cancelled)
	s.notify(id)
	return true
}
