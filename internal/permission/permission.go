package permission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/enum"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

var ErrorPermissionDenied = errors.New("user denied permission")

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

// PermissionRequest represents a permission request for tool execution
// This eliminates split-brain states like {granted: true, denied: true}
type PermissionRequest struct {
	ID PermissionRequestId `json:"id"`
	CreatePermissionRequest
}

type PermissionEvent struct {
	ToolCallID string          `json:"tool_call_id"`
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
	s.publishUnsafe(permission, enum.ToolCallStatePermissionApproved)
	s.sessionPermissions.Append(permission)
	s.noLongerActiveRequest(permission)
}

func (s *permissionService) Grant(permission PermissionRequest) {
	s.publishUnsafe(permission, enum.ToolCallStatePermissionApproved)
	s.noLongerActiveRequest(permission)
}

func (s *permissionService) Deny(permission PermissionRequest) {
	s.publishUnsafe(permission, enum.ToolCallStatePermissionDenied)
	s.noLongerActiveRequest(permission)
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

func (s *permissionService) noLongerActiveRequest(permission PermissionRequest) {
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
}

func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	if s.skip {
		return true
	}

	// tell the UI that a permission was requested
	s.uiBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: opts.ToolCallID,
		Status:     enum.ToolCallStatePermissionPending,
	})
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	// Check if the tool/action combination is in the allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true
	}

	autoApprove, _ := s.autoApproveSessions.Get(opts.SessionID)

	if autoApprove {
		return true
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
		ID:                      uuid.New().String(),
		CreatePermissionRequest: opts,
	}
	permission.CreatePermissionRequest.Path = dir

	for request := range s.sessionPermissions.Seq() {
		if request.ToolName == permission.ToolName &&
			request.Action == permission.Action &&
			request.SessionID == permission.SessionID &&
			request.Path == permission.Path {
			return true
		}
	}

	s.activeRequest = &permission

	respCh := make(chan enum.ToolCallState, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)

	return <-respCh == enum.ToolCallStatePermissionApproved
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


