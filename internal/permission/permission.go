package permission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/charmbracelet/crushcl/internal/csync"
	"github.com/charmbracelet/crushcl/internal/pubsub"
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

type PermissionNotification struct {
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
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

type Service interface {
	pubsub.Subscriber[PermissionRequest]
	GrantPersistent(permission PermissionRequest)
	Grant(permission PermissionRequest)
	Deny(permission PermissionRequest)
	Request(ctx context.Context, opts CreatePermissionRequest) (bool, error)
	AutoApproveSession(sessionID string)
	SetSkipRequests(skip bool)
	SkipRequests() bool
	SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[PermissionNotification]
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	notificationBroker    *pubsub.Broker[PermissionNotification]
	workingDir            string
	sessionPermissions    []PermissionRequest
	sessionPermissionsMu  sync.RWMutex
	pendingRequests       *csync.Map[string, chan bool]
	autoApproveSessions   map[string]bool
	autoApproveSessionsMu sync.RWMutex
	skip                  bool
	allowedTools          []string

	// used to make sure we only process one request at a time
	requestMu       sync.Mutex
	activeRequest   *PermissionRequest
	activeRequestMu sync.Mutex
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) {
	s.publishGrant(permission)

	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	s.clearActiveRequest(permission.ID)
}

func (s *permissionService) Grant(permission PermissionRequest) {
	s.publishGrant(permission)
	s.clearActiveRequest(permission.ID)
}

func (s *permissionService) publishGrant(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}
}

func (s *permissionService) clearActiveRequest(id string) {
	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == id {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
}

func (s *permissionService) Deny(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    false,
		Denied:     true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- false
	}

	s.clearActiveRequest(permission.ID)
}

func (s *permissionService) Request(ctx context.Context, opts CreatePermissionRequest) (bool, error) {
	if s.skip || s.isAllowedTool(opts.ToolName, opts.Action) {
		return true, nil
	}

	s.notifyPermissionRequested(opts.ToolCallID)

	s.requestMu.Lock()
	defer s.requestMu.Unlock()

	if s.isAutoApprovedSession(opts.SessionID) {
		s.notifyPermissionGranted(opts.ToolCallID)
		return true, nil
	}

	permission := s.createPermissionRequest(opts)

	if s.hasExistingPermission(permission) {
		s.notifyPermissionGranted(opts.ToolCallID)
		return true, nil
	}

	return s.waitForPermissionDecision(ctx, permission, opts.ToolCallID)
}

func (s *permissionService) isAllowedTool(toolName, action string) bool {
	commandKey := toolName + ":" + action
	return slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, toolName)
}

func (s *permissionService) isAutoApprovedSession(sessionID string) bool {
	s.autoApproveSessionsMu.RLock()
	defer s.autoApproveSessionsMu.RUnlock()
	return s.autoApproveSessions[sessionID]
}

func (s *permissionService) createPermissionRequest(opts CreatePermissionRequest) PermissionRequest {
	dir := s.resolvePath(opts.Path)

	return PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}
}

func (s *permissionService) resolvePath(path string) string {
	if path == "" || path == "." {
		return s.workingDir
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return s.workingDir
	}

	if fileInfo.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

func (s *permissionService) hasExistingPermission(req PermissionRequest) bool {
	s.sessionPermissionsMu.RLock()
	defer s.sessionPermissionsMu.RUnlock()

	for _, p := range s.sessionPermissions {
		if p.ToolName == req.ToolName && p.Action == req.Action &&
			p.SessionID == req.SessionID && p.Path == req.Path {
			return true
		}
	}
	return false
}

func (s *permissionService) waitForPermissionDecision(ctx context.Context, permission PermissionRequest, toolCallID string) (bool, error) {
	s.activeRequestMu.Lock()
	s.activeRequest = &permission
	s.activeRequestMu.Unlock()

	respCh := make(chan bool, 1)
	s.pendingRequests.Set(permission.ID, respCh)
	defer s.pendingRequests.Del(permission.ID)

	s.Publish(pubsub.CreatedEvent, permission)

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case granted := <-respCh:
		return granted, nil
	}
}

func (s *permissionService) notifyPermissionRequested(toolCallID string) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: toolCallID,
	})
}

func (s *permissionService) notifyPermissionGranted(toolCallID string) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: toolCallID,
		Granted:    true,
	})
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
	s.skip = skip
}

func (s *permissionService) SkipRequests() bool {
	return s.skip
}

func NewPermissionService(workingDir string, skip bool, allowedTools []string) Service {
	return &permissionService{
		Broker:              pubsub.NewBroker[PermissionRequest](),
		notificationBroker:  pubsub.NewBroker[PermissionNotification](),
		workingDir:          workingDir,
		sessionPermissions:  make([]PermissionRequest, 0),
		autoApproveSessions: make(map[string]bool),
		skip:                skip,
		allowedTools:        allowedTools,
		pendingRequests:     csync.NewMap[string, chan bool](),
	}
}
