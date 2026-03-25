package permission

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
)

type PermissionErrorKind string

const (
	PermissionErrorKindUserDenied   PermissionErrorKind = "user_denied"
	PermissionErrorKindPolicyDenied PermissionErrorKind = "policy_denied"
)

type PermissionError struct {
	Kind    PermissionErrorKind
	Message string
	Details string
}

func (e *PermissionError) Error() string {
	if e == nil {
		return "permission error"
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if e.Kind == PermissionErrorKindPolicyDenied {
		return "permission blocked by safety policy"
	}
	return "user denied permission"
}

func (e *PermissionError) Is(target error) bool {
	t, ok := target.(*PermissionError)
	if !ok {
		return false
	}
	return e.Kind == t.Kind
}

var (
	ErrorPermissionDenied  = &PermissionError{Kind: PermissionErrorKindUserDenied, Message: "user denied permission"}
	ErrorPermissionBlocked = &PermissionError{Kind: PermissionErrorKindPolicyDenied, Message: "permission blocked by safety policy"}
)

func NewPermissionBlockedError(message, details string) error {
	if strings.TrimSpace(message) == "" {
		message = ErrorPermissionBlocked.Error()
	}
	return &PermissionError{
		Kind:    PermissionErrorKindPolicyDenied,
		Message: strings.TrimSpace(message),
		Details: strings.TrimSpace(details),
	}
}

func AsPermissionError(err error) (*PermissionError, bool) {
	var permissionErr *PermissionError
	if errors.As(err, &permissionErr) {
		return permissionErr, true
	}
	return nil, false
}

func IsPermissionError(err error) bool {
	return errors.Is(err, ErrorPermissionDenied) || errors.Is(err, ErrorPermissionBlocked)
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
	ToolCallID string `json:"tool_call_id"`
	Granted    bool   `json:"granted"`
	Denied     bool   `json:"denied"`
}

type PermissionRequest struct {
	ID          string      `json:"id"`
	SessionID   string      `json:"session_id"`
	ToolCallID  string      `json:"tool_call_id"`
	ToolName    string      `json:"tool_name"`
	Description string      `json:"description"`
	Action      string      `json:"action"`
	Params      any         `json:"params"`
	Path        string      `json:"path"`
	AutoReview  *AutoReview `json:"auto_review,omitempty"`
}

type EvaluationDecision string

const (
	EvaluationDecisionAllow EvaluationDecision = "allow"
	EvaluationDecisionAsk   EvaluationDecision = "ask"
	EvaluationDecisionDeny  EvaluationDecision = "deny"
)

type EvaluationResult struct {
	Decision   EvaluationDecision `json:"decision"`
	Permission PermissionRequest  `json:"permission"`
}

type AutoApprovalConfidence string

const (
	AutoApprovalConfidenceLow    AutoApprovalConfidence = "low"
	AutoApprovalConfidenceMedium AutoApprovalConfidence = "medium"
	AutoApprovalConfidenceHigh   AutoApprovalConfidence = "high"
)

type AutoClassification struct {
	AllowAuto  bool                   `json:"allow_auto"`
	Reason     string                 `json:"reason"`
	Confidence AutoApprovalConfidence `json:"confidence"`
}

type AutoReviewTrigger string

const (
	AutoReviewTriggerClassifierBlock       AutoReviewTrigger = "classifier_block"
	AutoReviewTriggerAlwaysManual          AutoReviewTrigger = "always_manual"
	AutoReviewTriggerClassifierUnavailable AutoReviewTrigger = "classifier_unavailable"
	AutoReviewTriggerClassifierFailed      AutoReviewTrigger = "classifier_failed"
	AutoReviewTriggerClassifierSuspended   AutoReviewTrigger = "classifier_suspended"
)

type AutoReview struct {
	Trigger    AutoReviewTrigger      `json:"trigger,omitempty"`
	Reason     string                 `json:"reason,omitempty"`
	Confidence AutoApprovalConfidence `json:"confidence,omitempty"`
}

type Classifier interface {
	ClassifyPermission(ctx context.Context, req PermissionRequest) (AutoClassification, error)
}

type Service interface {
	pubsub.Subscriber[PermissionRequest]
	GrantPersistent(permission PermissionRequest)
	Grant(permission PermissionRequest)
	Deny(permission PermissionRequest)
	EvaluateRequest(ctx context.Context, opts CreatePermissionRequest) (EvaluationResult, error)
	Prompt(ctx context.Context, permission PermissionRequest) (bool, error)
	Request(ctx context.Context, opts CreatePermissionRequest) (bool, error)
	AutoApproveSession(sessionID string)
	SetSessionAutoApprove(sessionID string, enabled bool)
	IsSessionAutoApprove(sessionID string) bool
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
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.sessionPermissionsMu.Lock()
	s.sessionPermissions = append(s.sessionPermissions, permission)
	s.sessionPermissionsMu.Unlock()

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
}

func (s *permissionService) Grant(permission PermissionRequest) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
		Granted:    true,
	})
	respCh, ok := s.pendingRequests.Get(permission.ID)
	if ok {
		respCh <- true
	}

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
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

	s.activeRequestMu.Lock()
	if s.activeRequest != nil && s.activeRequest.ID == permission.ID {
		s.activeRequest = nil
	}
	s.activeRequestMu.Unlock()
}

func (s *permissionService) Request(ctx context.Context, opts CreatePermissionRequest) (bool, error) {
	eval, err := s.EvaluateRequest(ctx, opts)
	if err != nil {
		return false, err
	}

	switch eval.Decision {
	case EvaluationDecisionAllow:
		return true, nil
	case EvaluationDecisionDeny:
		return false, ErrorPermissionBlocked
	default:
		return s.Prompt(ctx, eval.Permission)
	}
}

func (s *permissionService) EvaluateRequest(_ context.Context, opts CreatePermissionRequest) (EvaluationResult, error) {
	permission, err := s.buildPermissionRequest(opts)
	if err != nil {
		return EvaluationResult{}, err
	}

	if s.skip {
		return EvaluationResult{Decision: EvaluationDecisionAllow, Permission: permission}, nil
	}

	commandKey := opts.ToolName + ":" + opts.Action
	if slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, opts.ToolName) {
		return EvaluationResult{Decision: EvaluationDecisionAllow, Permission: permission}, nil
	}

	s.autoApproveSessionsMu.RLock()
	autoApprove := s.autoApproveSessions[opts.SessionID]
	s.autoApproveSessionsMu.RUnlock()
	if autoApprove {
		return EvaluationResult{Decision: EvaluationDecisionAllow, Permission: permission}, nil
	}

	hookDecision := plugin.TriggerPermissionAsk(plugin.PermissionAskInput{
		Permission: plugin.PermissionRequest{
			ID:          permission.ID,
			SessionID:   permission.SessionID,
			ToolCallID:  permission.ToolCallID,
			ToolName:    permission.ToolName,
			Description: permission.Description,
			Action:      permission.Action,
			Params:      permission.Params,
			Path:        permission.Path,
		},
	})
	if hookDecision.Action == plugin.PermissionAllow {
		return EvaluationResult{Decision: EvaluationDecisionAllow, Permission: permission}, nil
	}
	if hookDecision.Action == plugin.PermissionDeny {
		return EvaluationResult{Decision: EvaluationDecisionDeny, Permission: permission}, nil
	}

	s.sessionPermissionsMu.RLock()
	defer s.sessionPermissionsMu.RUnlock()
	for _, p := range s.sessionPermissions {
		if p.ToolName == permission.ToolName && p.Action == permission.Action && p.SessionID == permission.SessionID && p.Path == permission.Path {
			return EvaluationResult{Decision: EvaluationDecisionAllow, Permission: permission}, nil
		}
	}

	return EvaluationResult{Decision: EvaluationDecisionAsk, Permission: permission}, nil
}

func (s *permissionService) Prompt(ctx context.Context, permission PermissionRequest) (bool, error) {
	s.notificationBroker.Publish(pubsub.CreatedEvent, PermissionNotification{
		ToolCallID: permission.ToolCallID,
	})
	s.requestMu.Lock()
	defer s.requestMu.Unlock()

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

func (s *permissionService) buildPermissionRequest(opts CreatePermissionRequest) (PermissionRequest, error) {
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

	return PermissionRequest{
		ID:          uuid.New().String(),
		Path:        dir,
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Description: opts.Description,
		Action:      opts.Action,
		Params:      opts.Params,
	}, nil
}

func (s *permissionService) AutoApproveSession(sessionID string) {
	s.SetSessionAutoApprove(sessionID, true)
}

func (s *permissionService) SetSessionAutoApprove(sessionID string, enabled bool) {
	s.autoApproveSessionsMu.Lock()
	if enabled {
		s.autoApproveSessions[sessionID] = true
	} else {
		delete(s.autoApproveSessions, sessionID)
	}
	s.autoApproveSessionsMu.Unlock()
}

func (s *permissionService) IsSessionAutoApprove(sessionID string) bool {
	s.autoApproveSessionsMu.RLock()
	enabled := s.autoApproveSessions[sessionID]
	s.autoApproveSessionsMu.RUnlock()
	return enabled
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
