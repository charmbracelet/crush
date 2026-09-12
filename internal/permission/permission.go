package permission

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// hookApprovalKey is the unexported context key used to mark a tool call as
// pre-approved by a PreToolUse hook. The value is the tool call ID so an
// approval can't be reused across calls that happen to share a context.
type hookApprovalKey struct{}

// WithHookApproval returns a context that marks the given tool call ID as
// pre-approved by a hook. When the permission service sees a matching
// request it short-circuits the normal prompt and grants immediately.
func WithHookApproval(ctx context.Context, toolCallID string) context.Context {
	return context.WithValue(ctx, hookApprovalKey{}, toolCallID)
}

// hookApproved reports whether the context carries a hook approval for the
// given tool call ID.
func hookApproved(ctx context.Context, toolCallID string) bool {
	if toolCallID == "" {
		return false
	}
	v, _ := ctx.Value(hookApprovalKey{}).(string)
	return v == toolCallID
}

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type PermissionNotification struct {
	SessionID  string `json:"session_id"`
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
	// Cancelled is set when the request was abandoned before a decision
	// (the turn was cancelled while the prompt was open).
	Cancelled bool `json:"cancelled"`
	// Reason optionally explains a denial (e.g. a PrePermission hook
	// verdict). Empty when the denial came from the user or from an
	// automatic path without a reason.
	Reason string `json:"reason,omitempty"`
}

// Outcome is how a permission prompt the user saw was resolved.
type Outcome string

const (
	OutcomeGranted   Outcome = "granted"
	OutcomeDenied    Outcome = "denied"
	OutcomeCancelled Outcome = "cancelled"
)

// PromptObserver hears about every prompt the user is shown and how it
// was resolved, in that order. Auto-approved calls show no prompt and
// are not reported. Calls run off the requesting goroutine, so a slow
// observer never delays the prompt or the tool.
type PromptObserver interface {
	PromptShown(ctx context.Context, req PermissionRequest)
	PromptResolved(ctx context.Context, req PermissionRequest, outcome Outcome)
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

// HookDecision is the outcome of a permission-hook evaluation.
type HookDecision int

const (
	// HookDecisionNone means the hook expressed no opinion; the normal
	// prompt proceeds.
	HookDecisionNone HookDecision = iota
	// HookDecisionAllow means the hook approved the call; the prompt is
	// skipped and the call is granted.
	HookDecisionAllow
	// HookDecisionDeny means the hook blocked the call; the prompt is
	// skipped and the call is denied with Reason surfacing to the UI.
	HookDecisionDeny
)

// PreHookResult is the outcome of a PrePermission hook evaluation.
type PreHookResult struct {
	Decision HookDecision
	Reason   string
}

// PermissionHooks lets the agent layer run external policy hooks around
// the permission prompt. Implementations are optional; the service
// behaves unchanged when none is set.
//
// PrePermission runs only when a request is about to prompt the user
// (after allowlists, session auto-approvals, and prior grants have been
// exhausted). Returning allow or deny short-circuits the prompt.
// PermissionDenied is a fire-and-forget notification that a request was
// denied; it never blocks the denial.
type PermissionHooks interface {
	PrePermission(ctx context.Context, req PermissionRequest) PreHookResult
	PermissionDenied(ctx context.Context, req PermissionRequest)
}

// AutoModeToggler is implemented by hooks that can be enabled/disabled
// at runtime (the native auto mode). SetAutoMode forwards to any
// installed hooks implementing this interface.
type AutoModeToggler interface {
	SetAutoModeEnabled(enabled bool)
}

type Service interface {
	pubsub.Subscriber[PermissionRequest]
	// GrantPersistent grants a permission request and remembers the grant
	// for the session. It returns true if this call actually resolved the
	// pending request; false if the request had already been resolved
	// (e.g., by another concurrent caller) or is unknown.
	GrantPersistent(permission PermissionRequest) bool
	// Grant grants a permission request. It returns true if this call
	// actually resolved the pending request; false if the request had
	// already been resolved or is unknown.
	Grant(permission PermissionRequest) bool
	// Deny denies a permission request. It returns true if this call
	// actually resolved the pending request; false if the request had
	// already been resolved or is unknown.
	Deny(permission PermissionRequest) bool
	Request(ctx context.Context, opts CreatePermissionRequest) (bool, error)
	// ActiveRequest returns the request that is currently awaiting a
	// decision, or false when no request is pending. A request is only
	// reported while it can still be resolved; a request whose waiter
	// was cancelled is not returned.
	ActiveRequest() (PermissionRequest, bool)
	AutoApproveSession(sessionID string)
	SetSkipRequests(skip bool)
	SkipRequests() bool
	// SetPermissionHooks installs optional external policy hooks. It may
	// be called once at startup; calls after the service is in use should
	// be avoided.
	SetPermissionHooks(hooks PermissionHooks)
	// DenialReason returns and clears the reason recorded for a denied
	// tool call (e.g. by a PrePermission hook or the native auto mode),
	// so tools can surface it to the model. Empty when the denial had no
	// recorded reason.
	DenialReason(toolCallID string) string
	// SetAutoMode sets the runtime auto-mode state and forwards it to any
	// installed hooks that support runtime toggling (the native auto
	// mode). AutoMode reports the current runtime state.
	SetAutoMode(enabled bool)
	AutoMode() bool
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
}

// PermissionKey is a composite key for session permission lookups.
type PermissionKey struct {
	SessionID string
	ToolName  string
	Action    string
	Path      string
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	notificationBroker    *pubsub.Broker[PermissionNotification]
	workingDir            string
	sessionPermissions    *csync.Map[PermissionKey, bool]
	pendingRequests       *csync.Map[string, chan bool]
	autoApproveSessions   map[string]bool
	autoApproveSessionsMu sync.RWMutex
	skip                  atomic.Bool
	allowedTools          []string
	observer              PromptObserver
	hooksMu               sync.RWMutex
	hooks                 PermissionHooks
	hooksWG               sync.WaitGroup
	denialReasons         *csync.Map[string, string]
	autoMode              atomic.Bool

	// used to make sure we only process one request at a time
	requestMu       sync.Mutex
	activeRequest   *PermissionRequest
	activeRequestMu sync.Mutex
}

// resolve atomically removes the pending request entry for the given
// permission and, if it was still pending, publishes exactly one
// PermissionNotification and forwards the outcome to the waiter on
// respCh. It returns true if this call resolved the request, false if
// it had already been resolved (e.g., by another concurrent caller) or
// the request ID is unknown.
//
// If onResolve is non-nil it runs after the pending entry has been
// taken but before the notification is published or the waiter is
// unblocked. This lets GrantPersistent record the session permission
// only when it actually wins the race, so a losing GrantPersistent
// that lost to a Deny does not leak an auto-approve entry.
//
// All three public resolution methods (Grant, GrantPersistent, Deny)
// route through this helper so multi-subscriber UIs can race safely:
// the first caller wins, the rest become no-ops.
func (s *permissionService) resolve(permission PermissionRequest, granted, denied bool, onResolve func()) bool {
	respCh, ok := s.pendingRequests.Take(permission.ID)
	if !ok {
		return false
	}

	if onResolve != nil {
		onResolve()
	}

	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  permission.SessionID,
		ToolCallID: permission.ToolCallID,
		Granted:    granted,
		Denied:     denied,
	})

	if denied {
		s.dispatchPermissionDenied(permission)
	}

	// respCh is buffered (cap 1) and only ever has at most one sender
	// per request because Take removes the entry under the map lock,
	// so this send never blocks.
	respCh <- granted

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
	return true
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) bool {
	// Record the persistent grant only if this call wins the
	// pending-request race. Otherwise a losing GrantPersistent that
	// lost to a Deny would still leave an auto-approve entry behind,
	// silently flipping later denied calls to allowed.
	return s.resolve(permission, true, false, func() {
		s.sessionPermissions.Set(PermissionKey{
			SessionID: permission.SessionID,
			ToolName:  permission.ToolName,
			Action:    permission.Action,
			Path:      permission.Path,
		}, true)
	})
}

func (s *permissionService) Grant(permission PermissionRequest) bool {
	return s.resolve(permission, true, false, nil)
}

func (s *permissionService) Deny(permission PermissionRequest) bool {
	return s.resolve(permission, false, true, nil)
}

func (s *permissionService) Request(ctx context.Context, opts CreatePermissionRequest) (bool, error) {
	if s.skip.Load() {
		return true, nil
	}

	// Check if the tool/action combination is in the allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true, nil
	}

	// A PreToolUse hook that returned decision=allow stamps the context
	// with the tool call ID. Treat that as a pre-approval and skip the
	// prompt entirely. We still publish a granted notification so the UI
	// and audit subscribers see the outcome.
	if hookApproved(ctx, opts.ToolCallID) {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}

	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	// tell the UI that a permission was requested
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		SessionID:  opts.SessionID,
		ToolCallID: opts.ToolCallID,
	})

	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()

	if autoApprove {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}

	fileInfo, err := os.Stat(opts.Path)
	dir := opts.Path
	if err == nil {
		if fileInfo.IsDir() {
			dir = opts.Path
		} else {
			dir = filepath.Dir(opts.Path)
		}
	}

	if dir == "." {
		dir = s.workingDir
	}
	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	if _, ok := s.sessionPermissions.Get(PermissionKey{
		SessionID: permission.SessionID,
		ToolName:  permission.ToolName,
		Action:    permission.Action,
		Path:      permission.Path,
	}); ok {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}

	// PrePermission hooks decide only when a prompt would actually
	// happen: allowlists, hook approvals, session auto-approvals, and
	// previously granted permissions have all been exhausted at this
	// point. They run while holding requestMu, so a slow hook delays
	// other permission requests; hooks that classify should keep a short
	// timeout (see the docs for the default and how to tune it).
	if h := s.currentHooks(); h != nil {
		switch res := h.PrePermission(ctx, permission); res.Decision {
		case HookDecisionAllow:
			s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
				ToolCallID: opts.ToolCallID,
				Granted:    true,
			})
			return true, nil
		case HookDecisionDeny:
			s.denialReasons.Set(opts.ToolCallID, res.Reason)
			s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
				ToolCallID: opts.ToolCallID,
				Denied:     true,
				Reason:     res.Reason,
			})
			return false, nil
		}
	}

	s.activeRequestMu.Lock()
	s.activeRequest = &permission
	s.activeRequestMu.Unlock()

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)
	resolved := s.observe(ctx, permission)

	select {
	case <-ctx.Done():
		// The prompt is abandoned without a decision; tell subscribers
		// so the request they saw is not left open.
		if _, ok := s.pendingRequests.Take(permission.ID); ok {
			s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
				SessionID:  opts.SessionID,
				ToolCallID: opts.ToolCallID,
				Cancelled:  true,
			})
			resolved(OutcomeCancelled)
			return false, ctx.Err()
		}
		// A decision won the race; resolve always delivers it.
		resolved(outcomeOf(<-respCh))
		return false, ctx.Err()
	case granted := <-respCh:
		resolved(outcomeOf(granted))
		return granted, nil
	}
}

// observe reports the prompt to the observer and returns the function
// that reports its outcome. Both run on one goroutine so the observer
// always hears them in order, and outlive ctx so a cancelled turn still
// reports the cancellation.
func (s *permissionService) observe(ctx context.Context, req PermissionRequest) func(Outcome) {
	if s.observer == nil {
		return func(Outcome) {}
	}
	ctx = context.WithoutCancel(ctx)
	outcome := make(chan Outcome, 1)
	go func() {
		s.observer.PromptShown(ctx, req)
		s.observer.PromptResolved(ctx, req, <-outcome)
	}()
	return func(o Outcome) { outcome <- o }
}

func outcomeOf(granted bool) Outcome {
	if granted {
		return OutcomeGranted
	}
	return OutcomeDenied
}

// ActiveRequest returns the request that is currently awaiting a
// decision. The active pointer can outlive the waiter that published
// it (Request leaves it behind when its context is cancelled), so a
// request is only reported while its ID is still registered in
// pendingRequests, i.e. while Grant/Deny can actually resolve it.
func (s *permissionService) ActiveRequest() (PermissionRequest, bool) {
	s.activeRequestMu.Lock()
	active := s.activeRequest
	s.activeRequestMu.Unlock()
	if active == nil {
		return PermissionRequest{}, false
	}
	if _, ok := s.pendingRequests.Get(active.ID); !ok {
		return PermissionRequest{}, false
	}
	return *active, true
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions[sessionID] = true
	s.autoApproveSessionsMu.Unlock()
}

// SetPermissionHooks installs the external policy hooks, replacing any
// previously installed implementation.
func (s *permissionService) SetPermissionHooks(hooks PermissionHooks) {
	s.hooksMu.Lock()
	s.hooks = hooks
	s.hooksMu.Unlock()
}

// DenialReason returns and clears the recorded denial reason for a tool
// call, so the tool can surface it to the model. It is consumed exactly
// once, keeping the map bounded.
func (s *permissionService) DenialReason(toolCallID string) string {
	if toolCallID == "" {
		return ""
	}
	reason, ok := s.denialReasons.Take(toolCallID)
	if !ok {
		return ""
	}
	return reason
}

// SetAutoMode sets the runtime auto-mode state and forwards it to any
// installed hooks that support runtime toggling.
func (s *permissionService) SetAutoMode(enabled bool) {
	s.autoMode.Store(enabled)
	if h := s.currentHooks(); h != nil {
		if toggler, ok := h.(AutoModeToggler); ok {
			toggler.SetAutoModeEnabled(enabled)
		}
	}
}

// AutoMode reports the runtime auto-mode state.
func (s *permissionService) AutoMode() bool {
	return s.autoMode.Load()
}

func (s *permissionService) currentHooks() PermissionHooks {
	s.hooksMu.RLock()
	h := s.hooks
	s.hooksMu.RUnlock()
	return h
}

// dispatchPermissionDenied fires the PermissionDenied hook asynchronously
// with a background context so denials are never blocked by hook
// execution, and so the hook does not inherit a cancelled/timed-out
// request context. In-flight dispatches are tracked so tests can wait
// for completion via waitForHookDispatches.
func (s *permissionService) dispatchPermissionDenied(req PermissionRequest) {
	h := s.currentHooks()
	if h == nil {
		return
	}
	s.hooksWG.Add(1)
	go func() {
		defer s.hooksWG.Done()
		h.PermissionDenied(context.Background(), req)
	}()
}

// waitForHookDispatches blocks until all in-flight PermissionDenied hook
// dispatches have completed. Test-only helper.
func (s *permissionService) waitForHookDispatches() {
	s.hooksWG.Wait()
}

func (s *permissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification] {
	return s.notificationBroker.Subscribe(ctx)
}

func (s *permissionService) SetSkipRequests(skip bool) {
	s.skip.Store(skip)
}

func (s *permissionService) SkipRequests() bool {
	return s.skip.Load()
}

// NewPermissionService creates the permission service. observer may be
// nil.
func NewPermissionService(workingDir string, skip bool, allowedTools []string, observer PromptObserver) Service {
	svc := &permissionService{
		Broker:              pubsub.NewBroker[PermissionRequest](),
		notificationBroker:  pubsub.NewBroker[PermissionNotification](),
		workingDir:          workingDir,
		sessionPermissions:  csync.NewMap[PermissionKey, bool](),
		autoApproveSessions: make(map[string]bool),
		allowedTools:        allowedTools,
		pendingRequests:     csync.NewMap[string, chan bool](),
		observer:            observer,
		denialReasons:       csync.NewMap[string, string](),
	}
	svc.skip.Store(skip)
	return svc
}
