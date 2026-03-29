package autopermission

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/plugin"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"mvdan.cc/sh/v3/syntax"
)

const (
	defaultMaxConsecutiveClassifierBlocks = 3
	defaultMaxTotalClassifierBlocks       = 20
)

var safeNullRedirectPattern = regexp.MustCompile(`(?i)(^|\s)(?:\d?>)\s*(?:/dev/null|nul|\$null)`)

var highRiskBashDirectCommands = map[string]struct{}{
	"curl":        {},
	"wget":        {},
	"sudo":        {},
	"kubectl":     {},
	"remove-item": {},
	"del":         {},
}

var highRiskBashPipelineTargets = map[string]struct{}{
	"sh":   {},
	"bash": {},
}

var highRiskGitFlagsWithValue = map[string]bool{
	"-C":                  true,
	"-c":                  true,
	"--git-dir":           true,
	"--work-tree":         true,
	"--namespace":         true,
	"--exec-path":         true,
	"--no-pager":          false,
	"--no-optional-locks": false,
}

var highRiskTerraformFlagsWithValue = map[string]bool{
	"-chdir": true,
}

var highRiskDockerFlagsWithValue = map[string]bool{
	"-c":          true,
	"-h":          true,
	"-l":          true,
	"--config":    true,
	"--context":   true,
	"--host":      true,
	"--log-level": true,
}

var highRiskNPMFlagsWithValue = map[string]bool{
	"-c":           true,
	"--cache":      true,
	"--loglevel":   true,
	"--prefix":     true,
	"--userconfig": true,
	"-w":           true,
	"--workspace":  true,
}

type sessionClassifierState struct {
	lastMode            session.PermissionMode
	consecutiveBlocks   int
	totalBlocks         int
	suspendAutoApproval bool
}

type service struct {
	base                        permission.Service
	sessions                    session.Service
	classifierFn                func() permission.Classifier
	workingDir                  string
	failClosedOnClassifierError bool
	allowedTools                []string
	classifierMu                sync.Mutex
	sessionStates               map[string]sessionClassifierState
}

func New(
	base permission.Service,
	sessions session.Service,
	classifierFn func() permission.Classifier,
	workingDir string,
	failClosedOnClassifierError bool,
	allowedTools []string,
) permission.Service {
	return &service{
		base:                        base,
		sessions:                    sessions,
		classifierFn:                classifierFn,
		workingDir:                  workingDir,
		failClosedOnClassifierError: failClosedOnClassifierError,
		allowedTools:                slices.Clone(allowedTools),
		sessionStates:               map[string]sessionClassifierState{},
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

func (s *service) HasPersistentPermission(p permission.PermissionRequest) bool {
	return s.base.HasPersistentPermission(p)
}

func (s *service) ClearPersistentPermissions(sessionID string) {
	s.base.ClearPersistentPermissions(sessionID)
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
		return false, permission.ErrorPermissionBlocked
	}

	mode, err := s.sessionPermissionMode(ctx, eval.Permission.SessionID)
	if err != nil {
		return s.base.Prompt(ctx, eval.Permission)
	}

	if mode == session.PermissionModeYolo {
		return true, nil
	}

	if s.base.HasPersistentPermission(eval.Permission) {
		slog.Debug("Permission allowed via session grant",
			"session_id", eval.Permission.SessionID,
			"tool", eval.Permission.ToolName,
			"action", eval.Permission.Action,
		)
		if mode == session.PermissionModeAuto {
			s.resetClassifierBlocks(eval.Permission.SessionID)
		}
		return true, nil
	}

	if s.isExplicitlyAllowed(opts, eval.Permission) {
		if mode == session.PermissionModeAuto && isAlwaysManual(eval.Permission, s.workingDir) {
			slog.Debug("Auto Mode explicit allowlist still requires manual confirmation",
				"session_id", eval.Permission.SessionID,
				"tool", eval.Permission.ToolName,
				"action", eval.Permission.Action,
			)
		} else {
			slog.Debug("Permission explicitly allowed",
				"session_id", eval.Permission.SessionID,
				"tool", eval.Permission.ToolName,
				"action", eval.Permission.Action,
			)
			if mode == session.PermissionModeAuto {
				s.resetClassifierBlocks(eval.Permission.SessionID)
			}
			return true, nil
		}
	}

	switch classifyPluginDecision(eval.Permission) {
	case permission.EvaluationDecisionAllow:
		return true, nil
	case permission.EvaluationDecisionDeny:
		return false, permission.ErrorPermissionBlocked
	}

	if mode == session.PermissionModeDefault {
		return s.base.Prompt(ctx, eval.Permission)
	}

	if mode != session.PermissionModeAuto {
		return s.base.Prompt(ctx, eval.Permission)
	}

	if s.shouldSuspendAutoApproval(eval.Permission.SessionID, mode) {
		slog.Debug("Auto Mode permission auto-approval suspended",
			"session_id", eval.Permission.SessionID,
			"tool", eval.Permission.ToolName,
			"action", eval.Permission.Action,
		)
		return s.base.Prompt(ctx, withAutoReview(eval.Permission, permission.AutoReview{
			Trigger: permission.AutoReviewTriggerClassifierSuspended,
			Reason:  "Auto approval is paused after repeated classifier blocks.",
		}))
	}

	if isAlwaysManual(eval.Permission, s.workingDir) {
		slog.Debug("Auto Mode permission requires manual confirmation",
			"session_id", eval.Permission.SessionID,
			"tool", eval.Permission.ToolName,
			"action", eval.Permission.Action,
		)
		return s.base.Prompt(ctx, withAutoReview(eval.Permission, permission.AutoReview{
			Trigger: permission.AutoReviewTriggerAlwaysManual,
			Reason:  "This action always requires manual confirmation in Auto Mode.",
		}))
	}

	if isAcceptEditsEquivalentRequest(eval.Permission, s.workingDir) {
		slog.Debug("Auto Mode permission allowed via accept-edits equivalent request",
			"session_id", eval.Permission.SessionID,
			"tool", eval.Permission.ToolName,
			"action", eval.Permission.Action,
		)
		s.resetClassifierBlocks(eval.Permission.SessionID)
		return true, nil
	}

	if isAutoModeAllowlistedRequest(eval.Permission) {
		slog.Debug("Auto Mode permission allowed via allowlist",
			"session_id", eval.Permission.SessionID,
			"tool", eval.Permission.ToolName,
			"action", eval.Permission.Action,
		)
		s.resetClassifierBlocks(eval.Permission.SessionID)
		return true, nil
	}

	if isSafeReadOnlyBashRequest(eval.Permission) {
		slog.Debug("Auto Mode permission allowed via safe read-only bash",
			"session_id", eval.Permission.SessionID,
			"tool", eval.Permission.ToolName,
			"action", eval.Permission.Action,
		)
		s.resetClassifierBlocks(eval.Permission.SessionID)
		return true, nil
	}

	classifier := s.classifier()
	if classifier == nil {
		return s.handleClassifierUnavailable(ctx, eval.Permission, "Auto Mode permission classification is unavailable.")
	}

	classification, err := classifier.ClassifyPermission(ctx, eval.Permission)
	if err != nil {
		return s.handleClassifierFailure(ctx, eval.Permission, err)
	}
	if classification.AllowAuto {
		slog.Debug("Auto Mode permission allowed by classifier",
			"session_id", eval.Permission.SessionID,
			"tool", eval.Permission.ToolName,
			"action", eval.Permission.Action,
			"reason", strings.TrimSpace(classification.Reason),
			"confidence", classification.Confidence,
		)
		s.resetClassifierBlocks(eval.Permission.SessionID)
		return true, nil
	}

	slog.Debug("Auto Mode permission blocked by classifier",
		"session_id", eval.Permission.SessionID,
		"tool", eval.Permission.ToolName,
		"action", eval.Permission.Action,
		"reason", strings.TrimSpace(classification.Reason),
		"confidence", classification.Confidence,
	)

	s.recordClassifierBlock(eval.Permission.SessionID)
	return s.base.Prompt(ctx, withAutoReview(eval.Permission, permission.AutoReview{
		Trigger:    permission.AutoReviewTriggerClassifierBlock,
		Reason:     firstNonEmpty(classification.Reason, "The classifier could not confirm this action is safe to auto-approve."),
		Confidence: classification.Confidence,
	}))
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

func (s *service) handleClassifierUnavailable(ctx context.Context, req permission.PermissionRequest, message string) (bool, error) {
	slog.Debug("Auto Mode classifier unavailable",
		"session_id", req.SessionID,
		"tool", req.ToolName,
		"action", req.Action,
		"fail_closed", s.failClosedOnClassifierError,
	)
	if s.failClosedOnClassifierError {
		return false, permission.NewPermissionBlockedError(message, "Set permissions.fail_closed_on_classifier_error=false to fall back to manual confirmation.")
	}
	return s.base.Prompt(ctx, withAutoReview(req, permission.AutoReview{
		Trigger: permission.AutoReviewTriggerClassifierUnavailable,
		Reason:  message,
	}))
}

func (s *service) handleClassifierFailure(ctx context.Context, req permission.PermissionRequest, err error) (bool, error) {
	reason := fmt.Sprintf("Auto Mode permission classification failed: %v", err)
	slog.Warn("Auto Mode permission classification failed",
		"session_id", req.SessionID,
		"tool", req.ToolName,
		"action", req.Action,
		"err", err,
	)
	if s.failClosedOnClassifierError {
		slog.Debug("Auto Mode classifier failure blocks request (fail closed)",
			"session_id", req.SessionID,
			"tool", req.ToolName,
			"action", req.Action,
		)
		return false, permission.NewPermissionBlockedError(
			"Auto Mode permission classification failed.",
			reason,
		)
	}
	slog.Debug("Auto Mode classifier failure falls back to manual review",
		"session_id", req.SessionID,
		"tool", req.ToolName,
		"action", req.Action,
	)
	return s.base.Prompt(ctx, withAutoReview(req, permission.AutoReview{
		Trigger: permission.AutoReviewTriggerClassifierFailed,
		Reason:  reason,
	}))
}

func withAutoReview(req permission.PermissionRequest, review permission.AutoReview) permission.PermissionRequest {
	req.AutoReview = &review
	return req
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *service) sessionPermissionMode(ctx context.Context, sessionID string) (session.PermissionMode, error) {
	if sessionID == "" {
		return session.PermissionModeDefault, nil
	}
	sess, err := s.sessions.Get(ctx, sessionID)
	if err != nil {
		return session.PermissionModeDefault, err
	}
	return sess.PermissionMode, nil
}

func (s *service) shouldSuspendAutoApproval(sessionID string, mode session.PermissionMode) bool {
	s.classifierMu.Lock()
	defer s.classifierMu.Unlock()

	state := s.sessionStates[sessionID]
	if mode != session.PermissionModeAuto {
		if _, ok := s.sessionStates[sessionID]; ok {
			slog.Debug("Auto Mode classifier state cleared for non-auto mode",
				"session_id", sessionID,
				"mode", mode,
			)
		}
		delete(s.sessionStates, sessionID)
		return false
	}
	if state.lastMode != session.PermissionModeAuto {
		state = sessionClassifierState{lastMode: mode}
		s.sessionStates[sessionID] = state
		slog.Debug("Auto Mode classifier state initialized",
			"session_id", sessionID,
		)
		return false
	}
	return state.suspendAutoApproval
}

func (s *service) resetClassifierBlocks(sessionID string) {
	s.classifierMu.Lock()
	defer s.classifierMu.Unlock()

	state := s.sessionStates[sessionID]
	prevConsecutive := state.consecutiveBlocks
	prevSuspended := state.suspendAutoApproval
	state.lastMode = session.PermissionModeAuto
	state.consecutiveBlocks = 0
	if state.totalBlocks < defaultMaxTotalClassifierBlocks {
		state.suspendAutoApproval = false
	}
	s.sessionStates[sessionID] = state
	if prevConsecutive > 0 || prevSuspended {
		slog.Debug("Auto Mode classifier block counters reset",
			"session_id", sessionID,
			"previous_consecutive_blocks", prevConsecutive,
			"total_blocks", state.totalBlocks,
			"previous_suspended", prevSuspended,
			"suspended", state.suspendAutoApproval,
		)
	}
}

func (s *service) recordClassifierBlock(sessionID string) {
	s.classifierMu.Lock()
	defer s.classifierMu.Unlock()

	state := s.sessionStates[sessionID]
	prevSuspended := state.suspendAutoApproval
	state.lastMode = session.PermissionModeAuto
	state.consecutiveBlocks++
	state.totalBlocks++
	if state.consecutiveBlocks >= defaultMaxConsecutiveClassifierBlocks || state.totalBlocks >= defaultMaxTotalClassifierBlocks {
		state.suspendAutoApproval = true
	}
	s.sessionStates[sessionID] = state
	slog.Debug("Auto Mode classifier block recorded",
		"session_id", sessionID,
		"consecutive_blocks", state.consecutiveBlocks,
		"total_blocks", state.totalBlocks,
		"suspended", state.suspendAutoApproval,
	)
	if !prevSuspended && state.suspendAutoApproval {
		slog.Debug("Auto Mode auto-approval suspended due to classifier blocks",
			"session_id", sessionID,
			"consecutive_blocks", state.consecutiveBlocks,
			"total_blocks", state.totalBlocks,
		)
	}
}

func (s *service) isExplicitlyAllowed(opts permission.CreatePermissionRequest, req permission.PermissionRequest) bool {
	commandKey := opts.ToolName + ":" + opts.Action
	return slices.Contains(s.allowedTools, commandKey) || slices.Contains(s.allowedTools, req.ToolName)
}

func isAutoModeAllowlistedRequest(req permission.PermissionRequest) bool {
	switch req.ToolName {
	case tools.ViewToolName, tools.ReadMCPResourceToolName:
		return req.Action == "read"
	case tools.LSToolName, tools.ListMCPResourcesToolName:
		return req.Action == "list"
	default:
		return false
	}
}

func isAcceptEditsEquivalentRequest(req permission.PermissionRequest, workingDir string) bool {
	return isSafeWorkspaceWrite(req, workingDir)
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

	sanitizedCommand := strings.TrimSpace(safeNullRedirectPattern.ReplaceAllString(command, " "))
	if strings.ContainsAny(sanitizedCommand, "\r\n;<>") ||
		strings.Contains(sanitizedCommand, "&&") ||
		strings.Contains(sanitizedCommand, "||") ||
		strings.Contains(sanitizedCommand, "|&") ||
		strings.Contains(sanitizedCommand, "$(") ||
		strings.Contains(sanitizedCommand, "`") {
		return false
	}

	segments := strings.Split(sanitizedCommand, "|")
	for _, segment := range segments {
		fields := strings.Fields(strings.ToLower(strings.TrimSpace(segment)))
		if len(fields) == 0 || !isSafeReadOnlyBashSegment(fields) {
			return false
		}
	}
	return true
}

func isSafeReadOnlyBashSegment(fields []string) bool {
	switch fields[0] {
	case "pwd", "ls", "dir", "tree", "cat", "type", "head", "tail", "wc", "rg", "grep", "which", "where", "stat":
		return true
	case "cut", "sort", "uniq":
		return true
	case "find":
		return isSafeFindCommand(fields)
	case "get-location", "get-childitem", "get-content", "select-string", "get-item", "get-command":
		return true
	case "select-object", "sort-object", "measure-object", "format-table", "out-string":
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
	case "stash":
		return len(args) > 1 && args[1] == "list"
	case "tag":
		return len(args) == 1 || slices.Contains(args[1:], "--list")
	case "config":
		return len(args) > 1 && args[1] == "--get"
	case "branch":
		return len(args) == 1 || slices.Contains(args[1:], "--show-current")
	case "remote":
		return len(args) > 1 && (args[1] == "-v" || (args[1] == "get-url" && len(args) <= 3))
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
		return false
	}
}

func classifyPluginDecision(req permission.PermissionRequest) permission.EvaluationDecision {
	hookDecision := plugin.TriggerPermissionAsk(plugin.PermissionAskInput{
		Permission: plugin.PermissionRequest{
			ID:          req.ID,
			SessionID:   req.SessionID,
			ToolCallID:  req.ToolCallID,
			ToolName:    req.ToolName,
			Description: req.Description,
			Action:      req.Action,
			Params:      req.Params,
			Path:        req.Path,
		},
	})
	switch hookDecision.Action {
	case plugin.PermissionAllow:
		return permission.EvaluationDecisionAllow
	case plugin.PermissionDeny:
		return permission.EvaluationDecisionDeny
	default:
		return permission.EvaluationDecisionAsk
	}
}

func isHighRiskBashRequest(req permission.PermissionRequest) bool {
	params, ok := req.Params.(tools.BashPermissionsParams)
	if !ok {
		return false
	}

	command := strings.TrimSpace(params.Command)
	if command == "" {
		return false
	}

	if highRisk, ok := isHighRiskShellCommand(command); ok {
		return highRisk
	}

	return isHighRiskBashTextFallback(strings.ToLower(command))
}

func isHighRiskShellCommand(command string) (bool, bool) {
	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return false, false
	}

	highRisk := false
	syntax.Walk(file, func(node syntax.Node) bool {
		if highRisk {
			return false
		}

		switch x := node.(type) {
		case *syntax.CallExpr:
			if isHighRiskCallExpr(x) {
				highRisk = true
				return false
			}
		case *syntax.BinaryCmd:
			if isHighRiskPipeline(x) {
				highRisk = true
				return false
			}
		}
		return true
	})

	return highRisk, true
}

func isHighRiskCallExpr(call *syntax.CallExpr) bool {
	args := shellCallArgs(call)
	if len(args) == 0 || !args[0].literal {
		return false
	}

	cmd := normalizeShellCommandName(args[0].value)
	if _, ok := highRiskBashDirectCommands[cmd]; ok {
		return true
	}

	switch cmd {
	case "rm":
		for _, arg := range args[1:] {
			if arg.literal && strings.HasPrefix(arg.value, "-") {
				return true
			}
		}
	case "git":
		subcommand, ok := firstShellSubcommand(args[1:], highRiskGitFlagsWithValue)
		if !ok {
			return false
		}
		return subcommand == "push" || (subcommand == "reset" && containsLiteralShellArg(args[1:], "--hard"))
	case "terraform":
		subcommand, ok := firstShellSubcommand(args[1:], highRiskTerraformFlagsWithValue)
		return ok && (subcommand == "apply" || subcommand == "destroy")
	case "docker":
		subcommand, ok := firstShellSubcommand(args[1:], highRiskDockerFlagsWithValue)
		return ok && subcommand == "push"
	case "npm":
		subcommand, ok := firstShellSubcommand(args[1:], highRiskNPMFlagsWithValue)
		return ok && subcommand == "publish"
	}

	return false
}

func isHighRiskPipeline(cmd *syntax.BinaryCmd) bool {
	if cmd == nil {
		return false
	}
	if op := cmd.Op.String(); op != "|" && op != "|&" {
		return false
	}
	return stmtInvokesHighRiskPipelineTarget(cmd.Y)
}

func stmtInvokesHighRiskPipelineTarget(stmt *syntax.Stmt) bool {
	if stmt == nil {
		return false
	}

	invokesShell := false
	syntax.Walk(stmt, func(node syntax.Node) bool {
		if invokesShell {
			return false
		}
		call, ok := node.(*syntax.CallExpr)
		if !ok {
			return true
		}
		args := shellCallArgs(call)
		if len(args) == 0 || !args[0].literal {
			return true
		}
		_, invokesShell = highRiskBashPipelineTargets[normalizeShellCommandName(args[0].value)]
		return !invokesShell
	})
	return invokesShell
}

type shellCallArg struct {
	value   string
	literal bool
}

func shellCallArgs(call *syntax.CallExpr) []shellCallArg {
	if call == nil || len(call.Args) == 0 {
		return nil
	}

	args := make([]shellCallArg, 0, len(call.Args))
	for _, word := range call.Args {
		arg, ok := literalWord(word)
		if ok {
			args = append(args, shellCallArg{
				value:   strings.ToLower(strings.TrimSpace(arg)),
				literal: true,
			})
			continue
		}
		args = append(args, shellCallArg{})
	}
	return args
}

func literalWord(word *syntax.Word) (string, bool) {
	if word == nil {
		return "", false
	}
	return literalWordParts(word.Parts)
}

func literalWordParts(parts []syntax.WordPart) (string, bool) {
	var b strings.Builder
	for _, part := range parts {
		switch x := part.(type) {
		case *syntax.Lit:
			b.WriteString(x.Value)
		case *syntax.SglQuoted:
			b.WriteString(x.Value)
		case *syntax.DblQuoted:
			value, ok := literalWordParts(x.Parts)
			if !ok {
				return "", false
			}
			b.WriteString(value)
		default:
			return "", false
		}
	}
	return b.String(), true
}

func normalizeShellCommandName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.ToLower(path.Base(filepath.ToSlash(raw)))
}

func firstShellSubcommand(args []shellCallArg, flagsWithValue map[string]bool) (string, bool) {
	for i := 0; i < len(args); i++ {
		if !args[i].literal {
			return "", false
		}

		arg := strings.TrimSpace(args[i].value)
		if arg == "" {
			continue
		}
		if arg == "--" {
			if i+1 >= len(args) || !args[i+1].literal {
				return "", false
			}
			return strings.ToLower(strings.TrimSpace(args[i+1].value)), true
		}
		if !strings.HasPrefix(arg, "-") {
			return strings.ToLower(arg), true
		}

		flag, _, hasInlineValue := strings.Cut(arg, "=")
		if flagsWithValue[flag] && !hasInlineValue {
			i++
			if i >= len(args) {
				return "", false
			}
		}
	}
	return "", false
}

func containsLiteralShellArg(args []shellCallArg, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	return slices.ContainsFunc(args, func(arg shellCallArg) bool {
		return arg.literal && strings.TrimSpace(arg.value) == target
	})
}

func isHighRiskBashTextFallback(command string) bool {
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
