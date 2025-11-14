package permission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

var ErrorPermissionDenied = errors.New("user denied permission")
var ErrorPermissionStatusUnknown = errors.New("unknown state: regarding tool call permission status")

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

// PermissionStatus represents the state of a permission request
// This eliminates split-brain states like {granted: true, denied: true}
type PermissionStatus string

const (
	PermissionPending  PermissionStatus = "pending"
	PermissionApproved PermissionStatus = "approved"
	PermissionDenied   PermissionStatus = "denied"
)

type PermissionEvent struct {
	ToolCallID string           `json:"tool_call_id"`
	Status     PermissionStatus `json:"status"`
}

type PermissionRequest struct {
	ID string `json:"id"`
	CreatePermissionRequest
}

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

	EventBroker           *pubsub.Broker[PermissionEvent]
	workingDir            string
	sessionPermissions    []PermissionRequest
	sessionPermissionsMu  sync.RWMutex
	pendingRequests       *csync.Map[string, chan bool]
	autoApproveSessions   map[string]bool
	autoApproveSessionsMu sync.RWMutex
	skip                  bool
	allowedTools          []string

	// used to make sure we only process one request at a time
	requestMu     sync.Mutex
	activeRequest *PermissionRequest
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) {
	s.EventBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: permission.ToolCallID,
		Status:     PermissionApproved,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	s.noLongerActiveRequest(permission)
}

func (s *permissionService) Grant(permission PermissionRequest) {
	s.EventBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: permission.ToolCallID,
		Status:     PermissionApproved,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.noLongerActiveRequest(permission)
}

func (s *permissionService) Deny(permission PermissionRequest) {
	s.EventBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: permission.ToolCallID,
		Status:     PermissionDenied,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- false
	}

	s.noLongerActiveRequest(permission)
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
	s.EventBroker.Publish(pubsub.CreatedEvent, PermissionEvent{
		ToolCallID: opts.ToolCallID,
		Status:     PermissionPending,
	})
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	// Check if the tool/action combination is in the allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true
	}

	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()

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

	s.sessionPermissionsMu.RLock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			s.sessionPermissionsMu.RUnlock()
			return true
		}
	}
	s.sessionPermissionsMu.RUnlock()

	s.sessionPermissionsMu.RLock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			s.sessionPermissionsMu.RUnlock()
			return true
		}
	}
	s.sessionPermissionsMu.RUnlock()

	s.activeRequest = &permission

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	// Publish the request
	s.Publish(pubsub.CreatedEvent, permission)

	return <-respCh
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions[sessionID] = true
	s.autoApproveSessionsMu.Unlock()
}

func (s *permissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionEvent] {
	return s.EventBroker.Subscribe(ctx)
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
		EventBroker:         pubsub.NewBroker[PermissionEvent](),
		workingDir:          workingDir,
		sessionPermissions:  make([]PermissionRequest, 0),
		autoApproveSessions: make(map[string]bool),
		skip:                skip,
		allowedTools:        allowedTools,
		pendingRequests:     csync.NewMap[string, chan bool](),
	}
}

func (status PermissionStatus) ToMessage() (string, error) {
	switch status {
	case PermissionPending:
		return "Requesting permission...", nil
	case PermissionApproved:
		return "Permission approved. Executing command...", nil
	case PermissionDenied:
		return "Permission denied.", nil
	default:
		return "", ErrorPermissionStatusUnknown
	}
}

func (status PermissionStatus) ToMessageColored(baseStyle lipgloss.Style) (string, error) {
	message, err := status.ToMessage()
	if err != nil {
		return "", err
	}
	return baseStyle.Render(message), nil
}
