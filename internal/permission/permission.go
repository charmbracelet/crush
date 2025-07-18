package permission

import (
	"errors"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

var ErrorPermissionDenied = errors.New("permission denied")

type CreatePermissionRequest struct {
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolName    string `json:"tool_name"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Params      any    `json:"params"`
	Path        string `json:"path"`
}

type Service interface {
	pubsub.Suscriber[PermissionRequest]
	GrantPersistent(permission PermissionRequest)
	Grant(permission PermissionRequest)
	Deny(permission PermissionRequest)
	Request(opts CreatePermissionRequest) bool
	AutoApproveSession(sessionID string)
	GetNextPendingRequest() *PermissionRequest
	HasPendingRequests() bool
}

type permissionService struct {
	*pubsub.Broker[PermissionRequest]

	workingDir            string
	sessionPermissions    []PermissionRequest
	sessionPermissionsMu  sync.RWMutex
	pendingRequests       sync.Map
	autoApproveSessions   []string
	autoApproveSessionsMu sync.RWMutex
	skip                  bool
	allowedTools          []string

	// Permission queue
	requestQueue   []PermissionRequest
	requestQueueMu sync.Mutex
	activeRequest  *PermissionRequest
}

func (s *permissionService) GrantPersistent(permission PermissionRequest) {
	respCh, ok := s.pendingRequests.Load(permission.ID)
	if ok {
		respCh.(chan bool) <- true
	}

	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	// Process next in queue
	s.processNextInQueue()
}

func (s *permissionService) Grant(permission PermissionRequest) {
	respCh, ok := s.pendingRequests.Load(permission.ID)
	if ok {
		respCh.(chan bool) <- true
	}

	// Process next in queue
	s.processNextInQueue()
}

func (s *permissionService) Deny(permission PermissionRequest) {
	respCh, ok := s.pendingRequests.Load(permission.ID)
	if ok {
		respCh.(chan bool) <- false
	}

	// Process next in queue
	s.processNextInQueue()
}

func (s *permissionService) processNextInQueue() {
	s.requestQueueMu.Lock()
	defer s.requestQueueMu.Unlock()

	// Remove the current active request from queue
	if s.activeRequest != nil && len(s.requestQueue) > 0 {
		// Find and remove the active request
		for i, req := range s.requestQueue {
			if req.ID == s.activeRequest.ID {
				s.requestQueue = slices.Delete(s.requestQueue, i, i+1)
				break
			}
		}
	}

	// Get next request if available
	if len(s.requestQueue) > 0 {
		s.activeRequest = &s.requestQueue[0]
		s.Publish(pubsub.CreatedEvent, *s.activeRequest)
	} else {
		s.activeRequest = nil
	}
}

func (s *permissionService) GetNextPendingRequest() *PermissionRequest {
	s.requestQueueMu.Lock()
	defer s.requestQueueMu.Unlock()

	if len(s.requestQueue) > 0 {
		return &s.requestQueue[0]
	}
	return nil
}

func (s *permissionService) HasPendingRequests() bool {
	s.requestQueueMu.Lock()
	defer s.requestQueueMu.Unlock()

	return len(s.requestQueue) > 0
}

func (s *permissionService) Request(opts CreatePermissionRequest) bool {
	if s.skip {
		return true
	}

	// Check if the tool/action combination is in the allowlist
	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return true
	}

	s.autoApproveSessionsMu.RLock()
	autoApprove := slices.Contains(s.autoApproveSessions, opts.SessionID)
	s.autoApproveSessionsMu.RUnlock()

	if autoApprove {
		return true
	}

	dir := filepath.Dir(opts.Path)
	if dir == "." {
		dir = s.workingDir
	}
	slog.Info("Requesting permission", "session_id", opts.SessionID, "tool_name", opts.ToolName, "action", opts.Action, "path", dir)
	permission := PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}

	s.sessionPermissionsMu.RLock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			s.sessionPermissionsMu.RUnlock()
			return true
		}
	}
	s.sessionPermissionsMu.RUnlock()

	respCh := make(chan bool, 1)

	s.pendingRequests.Store(permission.ID, respCh)
	defer s.pendingRequests.Delete(permission.ID)

	// Add to queue
	s.requestQueueMu.Lock()
	s.requestQueue = append(s.requestQueue, permission)
	// If this is the first request or no active request, publish it
	if s.activeRequest == nil {
		s.activeRequest = &permission
		s.requestQueueMu.Unlock()
		s.Publish(pubsub.CreatedEvent, permission)
	} else {
		s.requestQueueMu.Unlock()
	}

	// Wait for the response indefinitely
	return <-respCh
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.autoApproveSessionsMu.Lock()
	s.autoApproveSessions = append(s.autoApproveSessions, sessionID)
	s.autoApproveSessionsMu.Unlock()
}

func NewPermissionService(workingDir string, skip bool, allowedTools []string) Service {
	return &permissionService{
		Broker:             pubsub.NewBroker[PermissionRequest](),
		workingDir:         workingDir,
		sessionPermissions: make([]PermissionRequest, 0),
		skip:               skip,
		allowedTools:       allowedTools,
		requestQueue:       make([]PermissionRequest, 0),
	}
}
