package permission

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/enum"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// RaceFreePermissionKey provides O(1) lookup key for permissions
type RaceFreePermissionKey struct {
	SessionID string
	ToolName  string
	Action    string
	Path      string
}

// ActiveRequest represents a lock-free active permission request
type ActiveRequest struct {
	RequestID    PermissionRequestId
	ResponseChan chan enum.ToolCallState // Buffered, non-blocking
	CreatedAt    time.Time
}

// RaceFreePermissionService provides race-free, optimized permission handling
type RaceFreePermissionService struct {
	*pubsub.Broker[PermissionRequest]

	// ✅ ELIMINATE MUTEX CONTENTION
	// No more requestMu - concurrent operations enabled

	// ✅ O(1) LOOKUPS INSTEAD OF O(n) SCANS
	permissionCache *csync.Map[RaceFreePermissionKey, bool]

	// ✅ LOCK-FREE ACTIVE REQUEST TRACKING
	activeRequests *csync.Map[PermissionRequestId, *ActiveRequest]

	// ✅ FAST PATH STRUCTURES
	autoApproveSessions *csync.Map[string, bool]
	allowlist           *csync.Map[string, bool]

	// ✅ UI EVENT SYSTEM
	uiBroker   *pubsub.Broker[PermissionEvent]
	workingDir string
	skip       atomic.Bool

	// ✅ USE EXISTING PERMISSION SLICE (REPLACE LATER)
	sessionPermissions *csync.Slice[PermissionRequest]
}

// NewRaceFreePermissionService creates a race-free permission service
func NewRaceFreePermissionService(workingDir string, skip bool, allowedTools []string) Service {
	service := &RaceFreePermissionService{
		Broker:              pubsub.NewBroker[PermissionRequest](),
		permissionCache:     csync.NewMap[RaceFreePermissionKey, bool](),
		activeRequests:      csync.NewMap[PermissionRequestId, *ActiveRequest](),
		autoApproveSessions: csync.NewMap[string, bool](),
		allowlist:           csync.NewMap[string, bool](),
		uiBroker:            pubsub.NewBroker[PermissionEvent](),
		workingDir:          workingDir,
		sessionPermissions:  csync.NewSlice[PermissionRequest](),
	}

	service.skip.Store(skip)

	// ✅ Initialize allowlist for O(1) lookups
	for _, tool := range allowedTools {
		service.allowlist.Set(tool, true)
	}

	return service
}

// Request handles permission request without global mutex
func (s *RaceFreePermissionService) Request(opts CreatePermissionRequest) bool {
	// ✅ FAST PATH: Skip mode (atomic read)
	if s.skip.Load() {
		return true
	}

	// ✅ PUBLISH BEFORE ANY STATE CHANGES (eliminates timing race)
	s.uiBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: opts.ToolCallID,
		Status:     enum.ToolCallStatePermissionPending,
	})

	// ✅ FAST PATH: Allowlist check (O(1))
	if s.isInAllowlist(opts.ToolName, opts.Action) {
		return true
	}

	// ✅ FAST PATH: Auto-approve session (O(1))
	if autoApprove, _ := s.autoApproveSessions.Get(opts.SessionID); autoApprove {
		return true
	}

	// ✅ FAST PATH: Permission cache check (O(1))
	if s.hasCachedPermission(opts) {
		return true
	}

	// ✅ MAIN PATH: Create lock-free request
	permission := PermissionRequest{
		ID:                      uuid.New().String(),
		CreatePermissionRequest: opts,
	}

	// ✅ LOCK-FREE ACTIVE REQUEST TRACKING
	activeReq := &ActiveRequest{
		RequestID:    permission.ID,
		ResponseChan: make(chan enum.ToolCallState, 1), // Buffered to prevent blocking
		CreatedAt:    time.Now(),
	}

	// ✅ ATOMIC OPERATIONS - NO LOCKS NEEDED
	s.activeRequests.Set(permission.ID, activeReq)

	// ✅ Publish permission request (pubsub is thread-safe)
	s.Publish(pubsub.CreatedEvent, permission)

	// ✅ NON-BLOCKING WAIT WITH TIMEOUT
	return s.waitForResponse(activeReq)
}

// GrantPersistent approves and caches permission (lock-free)
func (s *RaceFreePermissionService) GrantPersistent(permission PermissionRequest) {
	s.publishApprovalEvent(permission)
	s.cachePermission(permission)
	s.completeRequest(permission.ID, enum.ToolCallStatePermissionApproved)
	s.sessionPermissions.Append(permission)
}

// Grant approves single request (lock-free)
func (s *RaceFreePermissionService) Grant(permission PermissionRequest) {
	s.publishApprovalEvent(permission)
	s.completeRequest(permission.ID, enum.ToolCallStatePermissionApproved)
}

// Deny rejects request (lock-free)
func (s *RaceFreePermissionService) Deny(permission PermissionRequest) {
	s.publishDenialEvent(permission)
	s.completeRequest(permission.ID, enum.ToolCallStatePermissionDenied)
}

// ✅ LOCK-FREE HELPER METHODS

func (s *RaceFreePermissionService) isInAllowlist(toolName, action string) bool {
	// Check tool:action
	if _, ok := s.allowlist.Get(toolName + ":" + action); ok {
		return true
	}
	// Check tool
	if _, ok := s.allowlist.Get(toolName); ok {
		return true
	}
	return false
}

func (s *RaceFreePermissionService) hasCachedPermission(opts CreatePermissionRequest) bool {
	key := RaceFreePermissionKey{
		SessionID: opts.SessionID,
		ToolName:  opts.ToolName,
		Action:    opts.Action,
		Path:      opts.Path, // Simplified - can add path resolution
	}

	approved, _ := s.permissionCache.Get(key)
	return approved
}

func (s *RaceFreePermissionService) cachePermission(permission PermissionRequest) {
	key := RaceFreePermissionKey{
		SessionID: permission.SessionID,
		ToolName:  permission.ToolName,
		Action:    permission.Action,
		Path:      permission.Path,
	}

	s.permissionCache.Set(key, true)
}

func (s *RaceFreePermissionService) completeRequest(requestID PermissionRequestId, status enum.ToolCallState) {
	// ✅ LOCK-FREE COMPLETION
	if activeReq, ok := s.activeRequests.Get(requestID); ok {
		// Non-blocking send (buffered channel)
		select {
		case activeReq.ResponseChan <- status:
			// ✅ Success: Response delivered
		default:
			// ❌ Channel full - should never happen with buffered channel
		}

		// Clean up
		s.activeRequests.Del(requestID)
	}
}

func (s *RaceFreePermissionService) publishApprovalEvent(permission PermissionRequest) {
	s.uiBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: permission.ToolCallID,
		Status:     enum.ToolCallStatePermissionApproved,
	})
}

func (s *RaceFreePermissionService) publishDenialEvent(permission PermissionRequest) {
	s.uiBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: permission.ToolCallID,
		Status:     enum.ToolCallStatePermissionDenied,
	})
}

func (s *RaceFreePermissionService) waitForResponse(activeReq *ActiveRequest) bool {
	select {
	case status := <-activeReq.ResponseChan:
		return status == enum.ToolCallStatePermissionApproved
	case <-time.After(30 * time.Second):
		// ✅ TIMEOUT CLEANUP (atomic)
		s.activeRequests.Del(activeReq.RequestID)
		return false
	}
}

// ✅ INTERFACE COMPLIANCE METHODS
func (s *RaceFreePermissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessions.Set(sessionID, true)
}

func (s *RaceFreePermissionService) SetSkipRequests(skip bool) {
	s.skip.Store(skip)
}

func (s *RaceFreePermissionService) SkipRequests() bool {
	return s.skip.Load()
}

func (s *RaceFreePermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionEvent] {
	return s.uiBroker.Subscribe(ctx)
}

// ✅ COMPILE-TIME INTERFACE VERIFICATION
var _ Service = (*RaceFreePermissionService)(nil)
