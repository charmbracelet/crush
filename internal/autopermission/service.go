package autopermission

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
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
	workingDir    string
	classifierMu  sync.Mutex
	sessionStates map[string]sessionClassifierState
}

func New(
	base permission.Service,
	sessions session.Service,
	classifierFn func() permission.Classifier,
	workingDir string,
) permission.Service {
	return &service{
		base:          base,
		sessions:      sessions,
		classifierFn:  classifierFn,
		workingDir:    workingDir,
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
	if isAlwaysManual(eval.Permission, s.workingDir) {
		return s.base.Prompt(ctx, eval.Permission)
	}

	if isAutoAllowedFastPath(eval.Permission, s.workingDir) {
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
	if state.totalBlocks < defaultMaxTotalClassifierBlocks {
		state.suspendAutoApproval = false
	}
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

func isAutoAllowedFastPath(req permission.PermissionRequest, workingDir string) bool {
	return isAutoAllowedReadOnly(req) ||
		isSafeReadOnlyBashRequest(req) ||
		isSafeWorkspaceWrite(req, workingDir)
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

func isSafeReadOnlyBashRequest(req permission.PermissionRequest) bool {
	if req.ToolName != tools.BashToolName || req.Action != "execute" {
		return false
	}

	params, ok := req.Params.(tools.BashPermissionsParams)
	if !ok || params.RunInBackground {
		return false
	}

	command := strings.TrimSpace(params.Command)
	if command == "" {
		return false
	}
	if strings.ContainsAny(command, "\r\n;<>") ||
		strings.Contains(command, "&&") ||
		strings.Contains(command, "||") ||
		strings.Contains(command, "|") ||
		strings.Contains(command, "$(") ||
		strings.Contains(command, "`") {
		return false
	}

	fields := strings.Fields(strings.ToLower(command))
	if len(fields) == 0 {
		return false
	}

	switch fields[0] {
	case "pwd", "ls", "dir", "tree", "cat", "type", "head", "tail", "wc", "rg", "grep", "which", "where", "stat":
		return true
	case "find":
		return isSafeFindCommand(fields)
	case "get-location", "get-childitem", "get-content", "select-string", "get-item", "get-command":
		return true
	case "git":
		return isSafeReadOnlyGitCommand(fields[1:])
	default:
		return false
	}
}

func isSafeReadOnlyGitCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	switch args[0] {
	case "status", "diff", "log", "show", "rev-parse", "ls-files", "grep", "symbolic-ref":
		return true
	case "branch":
		return len(args) == 1
	case "remote":
		return len(args) > 1 && args[1] == "-v"
	default:
		return false
	}
}

func isSafeFindCommand(args []string) bool {
	for _, arg := range args[1:] {
		if arg == "-delete" || arg == "-exec" || arg == "-execdir" || arg == "-ok" || arg == "-okdir" {
			return false
		}
		if arg == "-fprint" || arg == "-fprintf" || arg == "-fls" {
			return false
		}
		if strings.HasPrefix(arg, "-exec") || strings.HasPrefix(arg, "-ok") || strings.HasPrefix(arg, "-fprint") || strings.HasPrefix(arg, "-fls") {
			return false
		}
	}
	return true
}

func isSafeWorkspaceWrite(req permission.PermissionRequest, workingDir string) bool {
	if workingDir == "" {
		return false
	}

	switch req.ToolName {
	case tools.EditToolName, tools.WriteToolName, tools.MultiEditToolName:
	default:
		return false
	}

	if !fsext.HasPrefix(req.Path, workingDir) {
		return false
	}

	filePath, ok := permissionRequestFilePath(req)
	if !ok || filePath == "" || !fsext.HasPrefix(filePath, workingDir) {
		return false
	}

	return !isSensitiveWorkspacePath(filePath, workingDir)
}

func permissionRequestFilePath(req permission.PermissionRequest) (string, bool) {
	switch params := req.Params.(type) {
	case tools.EditPermissionsParams:
		return params.FilePath, true
	case tools.WritePermissionsParams:
		return params.FilePath, true
	case tools.MultiEditPermissionsParams:
		return params.FilePath, true
	default:
		return "", false
	}
}

func isSensitiveWorkspacePath(path, workingDir string) bool {
	rel, err := filepath.Rel(workingDir, path)
	if err != nil {
		return true
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || strings.HasPrefix(rel, "../") {
		return true
	}

	lowerRel := strings.ToLower(rel)
	lowerBase := strings.ToLower(filepath.Base(path))

	switch {
	case lowerRel == ".cursorrules":
		return true
	case lowerRel == ".github/copilot-instructions.md":
		return true
	case strings.HasPrefix(lowerRel, ".cursor/rules/"):
		return true
	case strings.HasPrefix(lowerRel, ".git/"):
		return true
	case strings.HasPrefix(lowerRel, ".crush/"):
		return true
	case strings.HasPrefix(lowerBase, ".env"):
		return true
	}

	switch lowerBase {
	case "agents.md", "agents.local.md",
		"claude.md", "claude.local.md",
		"gemini.md", "gemini.local.md",
		"crush.md", "crush.local.md",
		"crush.json", ".crush.json":
		return true
	default:
		return false
	}
}

func isAlwaysManual(req permission.PermissionRequest, workingDir string) bool {
	switch req.ToolName {
	case tools.DownloadToolName, tools.FetchToolName, tools.AgenticFetchToolName:
		return true
	case tools.BashToolName:
		return isHighRiskBashRequest(req)
	case tools.EditToolName, tools.WriteToolName, tools.MultiEditToolName:
		filePath, ok := permissionRequestFilePath(req)
		return ok && isSensitiveWorkspacePath(filePath, workingDir)
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
