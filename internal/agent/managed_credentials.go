package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/pubsub"
)

const (
	// managedExpiryMargin is how close to its stated expiry a managed
	// credential may get before a turn asks its owner for a fresh one.
	managedExpiryMargin = 60 * time.Second
	// managedWaitTimeout bounds how long a turn waits for the owner of a
	// managed credential to push a replacement.
	managedWaitTimeout = 30 * time.Second
)

var (
	// errNoReplacementCredential is returned when the owner of a managed
	// credential did not push a replacement in time.
	errNoReplacementCredential = errors.New("no replacement arrived")
	// errReplacementRejected is returned when the provider rejected the
	// replacement too; a second 401 in one turn is terminal.
	errReplacementRejected = errors.New("the replacement was rejected too")
)

// managedCredentialError is how a turn fails when the provider rejected a
// managed credential. Its text never carries the provider's response body.
// It unwraps to the provider error so status checks keep working.
type managedCredentialError struct {
	provider string
	reason   error
	err      error
}

func (e *managedCredentialError) Error() string {
	if errors.Is(e.reason, errReplacementRejected) {
		return fmt.Sprintf("managed credential for %s was rejected, and so was its replacement", e.provider)
	}
	return fmt.Sprintf("managed credential for %s was rejected and no replacement arrived", e.provider)
}

func (e *managedCredentialError) Unwrap() error { return e.err }

// prepareManagedCredential readies a managed provider for a turn. Crush
// never refreshes a managed credential: when it is about to expire the
// owner is asked for a fresh one, and the turn waits for it for a bounded
// time before going ahead with what it has (a 401 then takes the
// unauthorized path).
func (c *coordinator) prepareManagedCredential(ctx context.Context, providerID string, cred config.ManagedCredential) error {
	if cred.ExpiresWithin(managedExpiryMargin) {
		slog.Info("Managed credential is expiring; asking its owner for a fresh one",
			"provider", providerID, "expires_at", cred.ExpiresAt)
		c.publishCredentialsRequired(providerID, notify.CredentialsReasonExpiring)
		if _, err := c.waitForManagedCredential(ctx, providerID, "", cred.Version); err != nil {
			slog.Warn("No fresh managed credential arrived; proceeding with the current one",
				"provider", providerID, "error", err)
		}
	}
	return c.syncManagedModels(ctx)
}

// prepareSubAgentTurn applies the pre-turn managed-credential checks to a
// subagent turn on model. It fails closed: models that are stale for the
// managed set (a push or a delete since they were built) and cannot be
// rebuilt are not used.
func (c *coordinator) prepareSubAgentTurn(ctx context.Context, model Model) error {
	providerID := model.ModelCfg.Provider
	if cred, ok := c.cfg.ManagedCredential(providerID); ok {
		return c.prepareManagedCredential(ctx, providerID, cred)
	}
	return c.syncManagedModels(ctx)
}

// authRefreshFor returns the OnAuthRefresh hook for one call (a turn or a
// summary) and, for managed providers, the call's managed state.
func (c *coordinator) authRefreshFor(providerCfg config.ProviderConfig) (func(context.Context, *fantasy.ProviderError) error, *managedCall) {
	if managed := c.newManagedCall(providerCfg.ID); managed != nil {
		return managed.refresh, managed
	}
	return c.makeAuthRefreshCallback(providerCfg), nil
}

// managedCall is one call's managed-credential state: the build the
// latest attempt ran on (recorded by agent.go as it selects it) and how
// the call's 401s were handled. Exactly one replacement is allowed.
type managedCall struct {
	c        *coordinator
	provider string

	mu        sync.Mutex
	attempt   *Model
	sent      []string
	refreshes int
	lastErr   error
}

// newManagedCall returns the state for a call on providerID, or nil when
// the provider is not managed.
func (c *coordinator) newManagedCall(providerID string) *managedCall {
	if !c.cfg.IsManagedProvider(providerID) {
		return nil
	}
	return &managedCall{c: c, provider: providerID}
}

// record notes the build an attempt is about to run on. Nil-safe.
func (m *managedCall) record(model Model) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.attempt = &model
	if model.Managed && !slices.Contains(m.sent, model.cred.AccessToken) {
		m.sent = append(m.sent, model.cred.AccessToken)
	}
	m.mu.Unlock()
}

// usedToken is the token the latest attempt presented. Before any attempt
// it falls back to the current pushed token.
func (m *managedCall) usedToken() string {
	m.mu.Lock()
	attempt := m.attempt
	m.mu.Unlock()
	if attempt != nil && attempt.Managed {
		return attempt.cred.AccessToken
	}
	cred, _ := m.c.cfg.ManagedCredential(m.provider)
	return cred.AccessToken
}

// refresh is the call's OnAuthRefresh hook. It judges the 401 against the
// token the failing attempt sent; a second 401 is terminal.
func (m *managedCall) refresh(ctx context.Context, _ *fantasy.ProviderError) error {
	used := m.usedToken()
	m.mu.Lock()
	m.refreshes++
	n := m.refreshes
	m.mu.Unlock()
	err := errReplacementRejected
	if n == 1 {
		err = m.c.replaceRejectedCredential(ctx, m.provider, used)
	}
	m.mu.Lock()
	m.lastErr = err
	m.mu.Unlock()
	return err
}

// mapErr turns the call's auth failure into a managedCredentialError.
// Nil-safe; other errors pass through.
func (m *managedCall) mapErr(err error) error {
	if m == nil || err == nil {
		return err
	}
	err = redactErr(err, m.secrets()...)
	if !isUnauthorized(err) {
		return err
	}
	if _, done := errors.AsType[*managedCredentialError](err); done {
		return err
	}
	m.mu.Lock()
	reason, n := m.lastErr, m.refreshes
	m.mu.Unlock()
	switch {
	case reason == nil && n > 0:
		// The replacement went out and came back 401.
		reason = errReplacementRejected
	case reason == nil:
		reason = errNoReplacementCredential
	}
	return &managedCredentialError{provider: m.provider, reason: reason, err: err}
}

// secrets lists every token this call may have presented: each attempt's,
// plus the one pushed now.
func (m *managedCall) secrets() []string {
	m.mu.Lock()
	out := slices.Clone(m.sent)
	m.mu.Unlock()
	if cred, ok := m.c.cfg.ManagedCredential(m.provider); ok {
		out = append(out, cred.AccessToken)
	}
	return out
}

// redactProviderError returns a copy of pe with the call's tokens removed,
// for logging. Nil-safe on both sides.
func (m *managedCall) redactProviderError(pe *fantasy.ProviderError) *fantasy.ProviderError {
	if m == nil || pe == nil {
		return pe
	}
	return redactProviderError(pe, m.secrets())
}

type managedCallKey struct{}

// withManagedCall attaches the call's managed state to ctx. A nil state
// shadows any outer one: a queued call run as its own turn from inside a
// running turn inherits that turn's context, and must not have its errors
// judged by the running turn's state.
func withManagedCall(ctx context.Context, m *managedCall) context.Context {
	if m == nil && managedCallFrom(ctx) == nil {
		return ctx
	}
	return context.WithValue(ctx, managedCallKey{}, m)
}

func managedCallFrom(ctx context.Context) *managedCall {
	if ctx == nil {
		return nil
	}
	m, _ := ctx.Value(managedCallKey{}).(*managedCall)
	return m
}

// mapManagedErr maps an auth failure on a managed model to the managed
// error before it is logged or persisted, using the call's state when ctx
// carries one (a turn or summary) and the model otherwise (a title).
func mapManagedErr(ctx context.Context, err error, model Model) error {
	if m := managedCallFrom(ctx); m != nil {
		return m.mapErr(err)
	}
	if err == nil || !model.Managed {
		return err
	}
	err = redactErr(err, model.cred.AccessToken)
	if !isUnauthorized(err) {
		return err
	}
	if _, done := errors.AsType[*managedCredentialError](err); done {
		return err
	}
	return &managedCredentialError{provider: model.ModelCfg.Provider, reason: errNoReplacementCredential, err: err}
}

const redacted = "[redacted]"

// redactedError is an error with pushed credentials scrubbed from its text
// and from the provider error it carries, so neither the turn's message
// nor a log line can repeat a token a provider echoed back.
type redactedError struct {
	msg   string
	inner error // sanitized *fantasy.ProviderError or *fantasy.Error, or nil
}

func (e *redactedError) Error() string { return e.msg }
func (e *redactedError) Unwrap() error { return e.inner }

// redactErr scrubs every literal occurrence of secrets from err. It
// returns err unchanged when none occurs.
func redactErr(err error, secrets ...string) error {
	secrets = slices.DeleteFunc(slices.Clone(secrets), func(s string) bool { return s == "" })
	if err == nil || len(secrets) == 0 {
		return err
	}
	scrub := func(s string) string {
		for _, secret := range secrets {
			s = strings.ReplaceAll(s, secret, redacted)
		}
		return s
	}
	var inner error
	if pe, ok := errors.AsType[*fantasy.ProviderError](err); ok {
		clean := redactProviderError(pe, secrets)
		if clean == pe && scrub(err.Error()) == err.Error() {
			return err
		}
		inner = clean
	} else if fe, ok := errors.AsType[*fantasy.Error](err); ok {
		c := *fe
		c.Title, c.Message = scrub(c.Title), scrub(c.Message)
		c.Cause = nil
		inner = &c
	}
	msg := scrub(err.Error())
	if inner == nil && msg == err.Error() {
		return err
	}
	return &redactedError{msg: msg, inner: inner}
}

// redactProviderError returns pe, or a copy of it with secrets scrubbed from
// every field that can carry one: the message, the dumped request (whose
// headers hold the credential) and response, and the cause's text.
func redactProviderError(pe *fantasy.ProviderError, secrets []string) *fantasy.ProviderError {
	contains := func(s string) bool {
		for _, secret := range secrets {
			if secret != "" && strings.Contains(s, secret) {
				return true
			}
		}
		return false
	}
	scrub := func(s string) string {
		for _, secret := range secrets {
			if secret != "" {
				s = strings.ReplaceAll(s, secret, redacted)
			}
		}
		return s
	}
	dirty := contains(pe.Message) || contains(pe.Title) || contains(string(pe.RequestBody)) ||
		contains(string(pe.ResponseBody)) || (pe.Cause != nil && contains(pe.Cause.Error()))
	for _, v := range pe.ResponseHeaders {
		dirty = dirty || contains(v)
	}
	if !dirty {
		return pe
	}
	c := *pe
	c.Message, c.Title = scrub(c.Message), scrub(c.Title)
	c.RequestBody = []byte(scrub(string(c.RequestBody)))
	c.ResponseBody = []byte(scrub(string(c.ResponseBody)))
	if c.ResponseHeaders != nil {
		c.ResponseHeaders = maps.Clone(c.ResponseHeaders)
		for k, v := range c.ResponseHeaders {
			c.ResponseHeaders[k] = scrub(v)
		}
	}
	if c.Cause != nil {
		c.Cause = errors.New(scrub(c.Cause.Error()))
	}
	return &c
}

// replaceRejectedCredential handles a 401 on a managed provider whose
// request presented rejected. A different token already on hand is used
// at once; otherwise the owner is asked and the call waits for one. It
// returns nil, with models rebuilt, when the request should be retried.
func (c *coordinator) replaceRejectedCredential(ctx context.Context, providerID, rejected string) error {
	if cur, ok := c.cfg.ManagedCredential(providerID); ok && cur.AccessToken != rejected {
		slog.Info("Managed credential was rejected; a newer one is already here",
			"provider", providerID, "version", cur.Version)
		return c.syncManagedModels(ctx)
	}
	slog.Info("Managed credential was rejected; asking its owner for a replacement", "provider", providerID)
	c.publishCredentialsRequired(providerID, notify.CredentialsReasonUnauthorized)
	if _, err := c.waitForManagedCredential(ctx, providerID, rejected, 0); err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			err = errNoReplacementCredential
		}
		slog.Warn("No replacement managed credential arrived", "provider", providerID, "error", err)
		return err
	}
	return c.syncManagedModels(ctx)
}

// waitForManagedCredential waits up to the managed wait timeout for a
// push newer than minVersion whose token differs from notToken.
func (c *coordinator) waitForManagedCredential(ctx context.Context, providerID, notToken string, minVersion uint64) (config.ManagedCredential, error) {
	timeout := c.managedWait
	if timeout <= 0 {
		timeout = managedWaitTimeout
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return c.cfg.ManagedCredentials().Wait(waitCtx, providerID, notToken, minVersion)
}

// syncManagedModels rebuilds the shared models when the managed set
// changed (a push or a delete) since they were built. The check and the
// rebuild happen under rebuildMu, so concurrent callers rebuild once.
func (c *coordinator) syncManagedModels(ctx context.Context) error {
	c.rebuildMu.Lock()
	defer c.rebuildMu.Unlock()
	if c.models.Get().managedVersion == c.cfg.ManagedCredentials().Version() {
		return nil
	}
	return c.rebuildModelsLocked(ctx)
}

// managedCredentialFor returns the live credential a build of providerCfg
// presents. The build carries it from then on.
func (c *coordinator) managedCredentialFor(providerCfg config.ProviderConfig) (config.ManagedCredential, bool) {
	return c.cfg.ManagedCredential(providerCfg.ID)
}

// publishCredentialsRequired asks the owner of a managed credential for a
// fresh one.
func (c *coordinator) publishCredentialsRequired(providerID, reason string) {
	if c.notify == nil {
		return
	}
	c.notify.Publish(pubsub.CreatedEvent, notify.Notification{
		Type:       notify.TypeCredentialsRequired,
		ProviderID: providerID,
		Reason:     reason,
	})
}
