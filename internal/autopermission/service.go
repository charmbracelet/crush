package autopermission

import (
	"context"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
)

const (
	defaultMaxConsecutiveClassifierBlocks = 3
	defaultMaxTotalClassifierBlocks       = 20
)

type sessionClassifierState struct {
	lastMode            session.CollaborationMode
	consecutiveBlocks   int
	totalBlocks         int
	suspendAutoApproval bool
}

type service struct {
	base          permission.Service
	sessions      session.Service
	classifierFn  func() permission.Classifier
	classifierMu  sync.Mutex
	sessionStates map[string]sessionClassifierState
}

func New(
	base permission.Service,
	sessions session.Service,
	classifierFn func() permission.Classifier,
) permission.Service {
	return &service{
		base:          base,
		sessions:      sessions,
		classifierFn:  classifierFn,
		sessionStates: map[string]sessionClassifierState{},
	}
}

func (s *service) Subscribe(ctx context.Context) <-chan pubsub.Event[permission.PermissionRequest] {
	return s.base.Subscribe(ctx)
}

func (s *service) GrantPersistent(p permission.PermissionRequest) {
	s.base.GrantPersistent(p)
}

func (s *service) Grant(p permission.PermissionRequest) {
	s.base.Grant(p)
}

func (s *service) Deny(p permission.PermissionRequest) {
	s.base.Deny(p)
}

func (s *service) EvaluateRequest(ctx context.Context, opts permission.CreatePermissionRequest) (permission.EvaluationResult, error) {
	return s.base.EvaluateRequest(ctx, opts)
}

func (s *service) Prompt(ctx context.Context, p permission.PermissionRequest) (bool, error) {
	return s.base.Prompt(ctx, p)
}

func (s *service) Request(ctx context.Context, opts permission.CreatePermissionRequest) (bool, error) {
	eval, err := s.base.EvaluateRequest(ctx, opts)
	if err != nil {
		return false, err
	}

	switch eval.Decision {
	case permission.EvaluationDecisionAllow:
		return true, nil
	case permission.EvaluationDecisionDeny:
		return false, nil
	}

	mode, err := s.sessionMode(ctx, eval.Permission.SessionID)
	if err != nil || mode != session.CollaborationModeAuto {
		return s.base.Prompt(ctx, eval.Permission)
	}

	if s.shouldSuspendAutoApproval(eval.Permission.SessionID, mode) {
		return s.base.Prompt(ctx, eval.Permission)
	}
	if isAlwaysManual(eval.Permission) {
		return s.base.Prompt(ctx, eval.Permission)
	}

	if isAutoAllowedReadOnly(eval.Permission) {
		s.resetClassifierBlocks(eval.Permission.SessionID)
		return true, nil
	}

	classifier := s.classifier()
	if classifier == nil {
		return s.base.Prompt(ctx, eval.Permission)
	}

	classification, err := classifier.ClassifyPermission(ctx, eval.Permission)
	if err != nil {
		return s.base.Prompt(ctx, eval.Permission)
	}
	if classification.AllowAuto {
		s.resetClassifierBlocks(eval.Permission.SessionID)
		return true, nil
	}

	s.recordClassifierBlock(eval.Permission.SessionID)
	return s.base.Prompt(ctx, eval.Permission)
}

func (s *service) AutoApproveSession(sessionID string) {
	s.base.AutoApproveSession(sessionID)
}

func (s *service) SetSessionAutoApprove(sessionID string, enabled bool) {
	s.base.SetSessionAutoApprove(sessionID, enabled)
}

func (s *service) IsSessionAutoApprove(sessionID string) bool {
	return s.base.IsSessionAutoApprove(sessionID)
}

func (s *service) SetSkipRequests(skip bool) {
	s.base.SetSkipRequests(skip)
}

func (s *service) SkipRequests() bool {
	return s.base.SkipRequests()
}

func (s *service) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return s.base.SubscribeNotifications(ctx)
}

func (s *service) classifier() permission.Classifier {
	if s.classifierFn == nil {
		return nil
	}
	return s.classifierFn()
}

func (s *service) sessionMode(ctx context.Context, sessionID string) (session.CollaborationMode, error) {
	if sessionID == "" {
		return session.CollaborationModeDefault, nil
	}
	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return session.CollaborationModeDefault, err
	}
	return sess.CollaborationMode, nil
}

func (s *service) shouldSuspendAutoApproval(sessionID string, mode session.CollaborationMode) bool {
	s.classifierMu.Lock()
	defer s.classifierMu.Unlock()

	state := s.sessionStates[sessionID]
	if mode != session.CollaborationModeAuto {
		delete(s.sessionStates, sessionID)
		return false
	}
	if state.lastMode != session.CollaborationModeAuto {
		state = sessionClassifierState{lastMode: mode}
		s.sessionStates[sessionID] = state
		return false
	}
	return state.suspendAutoApproval
}

func (s *service) resetClassifierBlocks(sessionID string) {
	s.classifierMu.Lock()
	defer s.classifierMu.Unlock()

	state := s.sessionStates[sessionID]
	state.lastMode = session.CollaborationModeAuto
	state.consecutiveBlocks = 0
	s.sessionStates[sessionID] = state
}

func (s *service) recordClassifierBlock(sessionID string) {
	s.classifierMu.Lock()
	defer s.classifierMu.Unlock()

	state := s.sessionStates[sessionID]
	state.lastMode = session.CollaborationModeAuto
	state.consecutiveBlocks++
	state.totalBlocks++
	if state.consecutiveBlocks >= defaultMaxConsecutiveClassifierBlocks || state.totalBlocks >= defaultMaxTotalClassifierBlocks {
		state.suspendAutoApproval = true
	}
	s.sessionStates[sessionID] = state
}

func isAutoAllowedReadOnly(req permission.PermissionRequest) bool {
	switch req.ToolName {
	case tools.ViewToolName, tools.ReadMCPResourceToolName:
		return req.Action == "read"
	case tools.LSToolName, tools.ListMCPResourcesToolName:
		return req.Action == "list"
	default:
		return false
	}
}

func isAlwaysManual(req permission.PermissionRequest) bool {
	switch req.ToolName {
	case tools.DownloadToolName, tools.FetchToolName, tools.AgenticFetchToolName:
		return true
	case tools.BashToolName:
		return isHighRiskBashRequest(req)
	default:
		return strings.HasPrefix(req.ToolName, "mcp_") && req.Action == "execute"
	}
}

func isHighRiskBashRequest(req permission.PermissionRequest) bool {
	params, ok := req.Params.(tools.BashPermissionsParams)
	if !ok {
		return false
	}

	command := strings.ToLower(strings.TrimSpace(params.Command))
	if command == "" {
		return false
	}

	highRiskSnippets := []string{
		"curl ",
		"wget ",
		"git push",
		"git reset --hard",
		"rm -",
		"remove-item",
		"del ",
		"sudo ",
		"kubectl ",
		"terraform apply",
		"terraform destroy",
		"npm publish",
		"docker push",
		"| sh",
		"| bash",
	}
	for _, snippet := range highRiskSnippets {
		if strings.Contains(command, snippet) {
			return true
		}
	}
	return false
}
