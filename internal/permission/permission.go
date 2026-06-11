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
	SessionID   string   `json:"session_id"`
	ToolCallID  string   `json:"tool_call_id"`
	ToolName    string   `json:"tool_name"`
	Description string   `json:"description"`
	Action      string   `json:"action"`
	Params      any      `json:"params"`
	Path        string   `json:"path"`
	Contexts    []string `json:"contexts,omitempty"`
}

type PermissionNotification struct {
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
}

type PermissionRequest struct {
	ID          string   `json:"id"`
	SessionID   string   `json:"session_id"`
	ToolCallID  string   `json:"tool_call_id"`
	ToolName    string   `json:"tool_name"`
	Description string   `json:"description"`
	Action      string   `json:"action"`
	Params      any      `json:"params"`
	Path        string   `json:"path"`
	Contexts    []string `json:"contexts,omitempty"`
	// PendingContexts is the subset of Contexts that are not yet satisfied by
	// config-level allowances or prior session grants. The dialog uses this to
	// show only the tokens the user is actually being asked to approve.
	PendingContexts []string `json:"pending_contexts,omitempty"`
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
	AutoApproveSession(sessionID string)
	SetSkipRequests(skip bool)
	SkipRequests() bool
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
}

// PermissionKey is a composite key for session permission lookups.
type PermissionKey struct {
	SessionID string
	ToolName  string
	Action    string
	Path      string
	Context   string // generic context token; empty for legacy tool grants
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
	allowedContexts       []string

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
		ToolCallID: permission.ToolCallID,
		Granted:    granted,
		Denied:     denied,
	})

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
		// If Contexts is non-empty, record one key per context token.
		// The Path field is intentionally omitted from contextual grant
		// keys — location semantics are captured by path: tokens, not
		// the working directory at approval time.
		if len(permission.Contexts) > 0 {
			for _, ctx := range permission.Contexts {
				s.sessionPermissions.Set(PermissionKey{
					SessionID: permission.SessionID,
					ToolName:  permission.ToolName,
					Action:    permission.Action,
					Context:   ctx,
				}, true)
			}
			return
		}

		// Otherwise record the legacy key for backward compatibility
		// with tools that don't use contextual approval (edit, write, view, etc.)
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
	// 1. Skip mode → allow
	if s.skip.Load() {
		return true, nil
	}

	// 2. Tool / tool:action allowlist → allow
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true, nil
	}

	// 3. Hook approval (context-stamped) → allow
	if hookApproved(ctx, opts.ToolCallID) {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
			ToolCallID: opts.ToolCallID,
			Granted:    true,
		})
		return true, nil
	}

	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	// tell the UI that a permission was requested
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: opts.ToolCallID,
	})

	// 4. Auto-approve session → allow
	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()

	if autoApprove {
		s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
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
		Contexts:    opts.Contexts,
	}

	// 5. Contextual approval: if len(Contexts) > 0 and every context is
	// satisfied → allow. Matching uses structured token prefix semantics
	// (see tokenSatisfies in context.go).
	//
	// Also compute PendingContexts — the subset that is NOT yet satisfied —
	// so the UI can display only what the user is actually being asked to
	// approve, omitting tokens already covered by prior grants.
	if len(permission.Contexts) > 0 {
		allApproved := true
		for _, ctx := range permission.Contexts {
			if !s.contextSatisfied(permission.SessionID, permission.ToolName, permission.Action, ctx) {
				allApproved = false
				permission.PendingContexts = append(permission.PendingContexts, ctx)
			}
		}
		if allApproved {
			s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
				ToolCallID: opts.ToolCallID,
				Granted:    true,
			})
			return true, nil
		}
	}

	// 6. Legacy session approval: if len(Contexts) == 0 → allow
	if len(permission.Contexts) == 0 {
		if _, ok := s.sessionPermissions.Get(PermissionKey{
			SessionID: permission.SessionID,
			ToolName:  permission.ToolName,
			Action:    permission.Action,
			Path:      permission.Path,
		}); ok {
			s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
				ToolCallID: opts.ToolCallID,
				Granted:    true,
			})
			return true, nil
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

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case granted := <-respCh:
		return granted, nil
	}
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions[sessionID] = true
	s.autoApproveSessionsMu.Unlock()
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

// contextSatisfied reports whether the given context token is satisfied by a
// config-level allowed token or by a previously stored session grant. Config
// tokens are checked first (fast path), followed by an exact key lookup, and
// finally a structured prefix scan over all session grants.
func (s *permissionService) contextSatisfied(sessionID, toolName, action, candidate string) bool {
	// Config-level allowed contexts (e.g. from allowed_commands): check
	// whether any configured token is broad enough to satisfy the candidate.
	for _, allowed := range s.allowedContexts {
		if tokenSatisfies(allowed, candidate) {
			return true
		}
	}

	// Fast path: exact match on stored contextual grant key.
	if _, ok := s.sessionPermissions.Get(PermissionKey{
		SessionID: sessionID,
		ToolName:  toolName,
		Action:    action,
		Context:   candidate,
	}); ok {
		return true
	}

	// Structured prefix match: iterate all contextual grants for this
	// session/tool/action and check whether any stored token is broad
	// enough to satisfy the candidate (e.g. command:go satisfies
	// command:go test; path:/tmp satisfies path:/tmp/subdir).
	for key := range s.sessionPermissions.Seq2() {
		if key.SessionID != sessionID || key.ToolName != toolName || key.Action != action || key.Context == "" {
			continue
		}
		if tokenSatisfies(key.Context, candidate) {
			return true
		}
	}
	return false
}

func NewPermissionService(workingDir string, skip bool, allowedTools []string, allowedContexts []string) Service {
	svc := &permissionService{
		Broker:              pubsub.NewBroker[PermissionRequest](),
		notificationBroker:  pubsub.NewBroker[PermissionNotification](),
		workingDir:          workingDir,
		sessionPermissions:  csync.NewMap[PermissionKey, bool](),
		autoApproveSessions: make(map[string]bool),
		allowedTools:        allowedTools,
		allowedContexts:     allowedContexts,
		pendingRequests:     csync.NewMap[string, chan bool](),
	}
	svc.skip.Store(skip)
	return svc
}
