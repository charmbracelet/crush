package permission

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/enum"
	"github.com/charmbracelet/crush/internal/errors"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

// Re-export centralized error for backward compatibility and convenient access
var ErrorPermissionDenied = errors.ErrUserDenied

type CreatePermissionRequest struct {
	SessionID   string             `json:"session_id"`
	ToolCallID  message.ToolCallID `json:"tool_call_id"`
	ToolName    string             `json:"tool_name"`
	Description string             `json:"description"`
	Action      string             `json:"action"`
	Params      any                `json:"params"`
	Path        string             `json:"path"`
}

// PermissionRequest represents a permission request for tool execution
// This eliminates split-brain states like {granted: true, denied: true}
type PermissionRequest struct {
	ID PermissionRequestId `json:"id"`
	CreatePermissionRequest
}

type PermissionEvent struct {
	ToolCallID message.ToolCallID `json:"tool_call_id"`
	Status     enum.ToolCallState `json:"status"`
}

type PermissionRequestId = string

type Service interface {
	pubsub.Subscriber[PermissionRequest]
	GrantPersistent(permission PermissionRequest)
	Grant(permission PermissionRequest)
	Deny(permission PermissionRequest)
	Request(opts CreatePermissionRequest) bool
	AutoApproveSession(sessionID string)
	SetSkipRequests(skip bool)
	SkipRequests() bool
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionEvent]
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	uiBroker            *pubsub.Broker[PermissionEvent]
	workingDir          string
	sessionPermissions  *csync.Slice[PermissionRequest]
	pendingRequests     *csync.Map[PermissionRequestId, chan enum.ToolCallState]
	autoApproveSessions *csync.Map[string, bool]
	skip                bool
	allowedTools        []string

	// used to make sure we only process one request at a time
	requestMu     sync.Mutex
	activeRequest *PermissionRequest
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	s.publishUnsafe(permission, enum.ToolCallStatePermissionApproved)
	s.sessionPermissions.Append(permission)
	s.noLongerActiveRequestUnsafe(permission)
}

func (s *permissionService) Grant(permission PermissionRequest) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	s.publishUnsafe(permission, enum.ToolCallStatePermissionApproved)
	s.noLongerActiveRequestUnsafe(permission)
}

func (s *permissionService) Deny(permission PermissionRequest) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	s.publishUnsafe(permission, enum.ToolCallStatePermissionDenied)
	s.noLongerActiveRequestUnsafe(permission)
}

// noLongerActiveRequestUnsafe clears active request (MUST be called under lock)
func (s *permissionService) noLongerActiveRequestUnsafe(permission PermissionRequest) {
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
}

func (s *permissionService) publishUnsafe(permission PermissionRequest, status enum.ToolCallState) {
	s.uiBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: permission.ToolCallID,
		Status:     status,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- status
	}
}



func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	if s.skip {
		return true
	}

	// Tell the UI that a permission was requested (thread-safe pubsub)
	s.uiBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: opts.ToolCallID,
		Status:     enum.ToolCallStatePermissionPending,
	})

	// Fast path checks (no lock needed - read-only or thread-safe)
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true
	}

	autoApprove, _ := s.autoApproveSessions.Get(opts.SessionID)
	if autoApprove {
		return true
	}

	// Build permission request
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
		ID:                      uuid.New().String(),
		CreatePermissionRequest: opts,
	}
	permission.Path = dir

	// Check session permissions (thread-safe iteration)
	for request := range s.sessionPermissions.Seq() {
		if request.ToolName == permission.ToolName &&
			request.Action == permission.Action &&
			request.SessionID == permission.SessionID &&
			request.Path == permission.Path {
			return true
		}
	}

	// Setup response channel and active request under lock
	respCh := make(chan enum.ToolCallState, 1)

	s.requestMu.Lock()
	s.activeRequest = &permission
	s.pendingRequests.Set(permission.ID, respCh)
	s.requestMu.Unlock()

	// Publish the request (thread-safe pubsub)
	s.Publish(pubsub.CreatedEvent, permission)

	// Wait for response WITHOUT holding the lock (prevents deadlock)
	result := <-respCh == enum.ToolCallStatePermissionApproved

	// Cleanup
	s.pendingRequests.Del(permission.ID)

	return result
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessions.Set(sessionID, true)
}

func (s *permissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionEvent] {
	return s.uiBroker.Subscribe(ctx)
}

func (s *permissionService) SetSkipRequests(skip bool) {
	s.skip = skip
}

func (s *permissionService) SkipRequests() bool {
	return s.skip
}

func NewPermissionService(workingDir string, skip bool, allowedTools []string) Service {
	return &permissionService{
		Broker:              pubsub.NewBroker[PermissionRequest](),
		uiBroker:            pubsub.NewBroker[PermissionEvent](),
		workingDir:          workingDir,
		sessionPermissions:  csync.NewSlice[PermissionRequest](),
		autoApproveSessions: csync.NewMap[string, bool](),
		skip:                skip,
		allowedTools:        allowedTools,
		pendingRequests:     csync.NewMap[PermissionRequestId, chan enum.ToolCallState](),
	}
}
