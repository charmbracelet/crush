package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
)

const (
	TmuxToolName = "tmux"

	defaultTmuxCaptureLines = 120
	maxTmuxCaptureLines     = 2000
	defaultTmuxDelayMillis  = 250
	maxTmuxDelayMillis      = 5000
	tmuxCommandTimeout      = 10 * time.Second
)

//go:embed tmux.md
var tmuxDescription string

var tmuxSessionNameRE = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)

type TmuxParams struct {
	Action      string `json:"action" description:"Action to run: start, send, capture, list, or kill"`
	Session     string `json:"session,omitempty" description:"Tmux session name. Required for send, capture, and kill. Generated for start when omitted."`
	Command     string `json:"command,omitempty" description:"Command to run when starting a session. If omitted, starts an interactive shell."`
	Input       string `json:"input,omitempty" description:"Literal text to send to a session for the send action"`
	Enter       *bool  `json:"enter,omitempty" description:"For send, press Enter after input. Defaults to true."`
	WorkingDir  string `json:"working_dir,omitempty" description:"Working directory for start. Defaults to the current workspace."`
	Lines       int    `json:"lines,omitempty" description:"Number of recent pane lines to capture. Defaults to 120, max 2000."`
	DelayMillis int    `json:"delay_millis,omitempty" description:"Milliseconds to wait before capturing after start/send. Defaults to 250, max 5000."`
	Description string `json:"description,omitempty" description:"Brief description of the tmux session or action"`
}

type TmuxPermissionsParams struct {
	Action      string `json:"action"`
	Session     string `json:"session,omitempty"`
	Command     string `json:"command,omitempty"`
	Input       string `json:"input,omitempty"`
	Enter       bool   `json:"enter"`
	WorkingDir  string `json:"working_dir,omitempty"`
	Lines       int    `json:"lines,omitempty"`
	DelayMillis int    `json:"delay_millis,omitempty"`
	Description string `json:"description,omitempty"`
}

type TmuxResponseMetadata struct {
	Action           string `json:"action"`
	Session          string `json:"session,omitempty"`
	Status           string `json:"status"`
	Output           string `json:"output,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
	Lines            int    `json:"lines,omitempty"`
}

func NewTmuxTool(permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TmuxToolName,
		tmuxDescription,
		func(ctx context.Context, params TmuxParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			action := strings.ToLower(strings.TrimSpace(params.Action))
			switch action {
			case "start":
				return runTmuxStart(ctx, permissions, workingDir, params, call)
			case "send":
				return runTmuxSend(ctx, permissions, workingDir, params, call)
			case "capture":
				return runTmuxCapture(ctx, params)
			case "list":
				return runTmuxList(ctx)
			case "kill":
				return runTmuxKill(ctx, permissions, workingDir, params, call)
			case "":
				return fantasy.NewTextErrorResponse("missing action"), nil
			default:
				return fantasy.NewTextErrorResponse("action must be one of: start, send, capture, list, kill"), nil
			}
		},
	)
}

func runTmuxStart(ctx context.Context, permissions permission.Service, workingDir string, params TmuxParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if err := requireTmux(); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	session := params.Session
	if session == "" {
		session = fmt.Sprintf("crush-%d", time.Now().UnixMilli())
	}
	if err := validateTmuxSession(session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	execWorkingDir, err := tmuxWorkingDir(workingDir, params.WorkingDir)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	if exists, err := tmuxSessionExists(ctx, session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	} else if exists {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("tmux session already exists: %s", session)), nil
	}

	if permissions != nil {
		granted, err := requestTmuxPermission(ctx, permissions, workingDir, call, params, session, "start", tmuxPermissionResource(params, session))
		if err != nil {
			return fantasy.ToolResponse{}, err
		}
		if !granted {
			return NewPermissionDeniedResponse(), nil
		}
	}

	args := []string{"new-session", "-d", "-s", session, "-c", execWorkingDir}
	if params.Command != "" {
		// Use interactive Bash so version managers, aliases, and prompt-time
		// shell setup are available in the tmux pane when the user needs them.
		args = append(args, "bash", "-ic", params.Command)
	}
	if output, err := runTmux(ctx, args...); err != nil {
		return fantasy.NewTextErrorResponse(formatTmuxError("start", output, err)), nil
	}

	time.Sleep(tmuxDelay(params.DelayMillis))
	output, _ := captureTmux(ctx, session, tmuxLines(params.Lines))
	statusLine := fmt.Sprintf("Tmux session started: %s", session)
	if strings.TrimSpace(output) != "" {
		statusLine += "\n\n" + output
	}
	metadata := TmuxResponseMetadata{
		Action:           "start",
		Session:          session,
		Status:           "running",
		Output:           output,
		WorkingDirectory: execWorkingDir,
		Lines:            tmuxLines(params.Lines),
	}
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(statusLine), metadata), nil
}

func runTmuxSend(ctx context.Context, permissions permission.Service, workingDir string, params TmuxParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if err := requireTmux(); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if params.Session == "" {
		return fantasy.NewTextErrorResponse("session is required for send"), nil
	}
	if err := validateTmuxSession(params.Session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if params.Input == "" && !tmuxEnter(params) {
		return fantasy.NewTextErrorResponse("input is required when enter is false"), nil
	}
	if exists, err := tmuxSessionExists(ctx, params.Session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	} else if !exists {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("tmux session not found: %s", params.Session)), nil
	}

	if permissions != nil {
		granted, err := requestTmuxPermission(ctx, permissions, workingDir, call, params, params.Session, "send", tmuxPermissionResource(params, params.Session))
		if err != nil {
			return fantasy.ToolResponse{}, err
		}
		if !granted {
			return NewPermissionDeniedResponse(), nil
		}
	}

	if params.Input != "" {
		if output, err := runTmux(ctx, "send-keys", "-t", params.Session, "-l", params.Input); err != nil {
			return fantasy.NewTextErrorResponse(formatTmuxError("send input", output, err)), nil
		}
	}
	if tmuxEnter(params) {
		if output, err := runTmux(ctx, "send-keys", "-t", params.Session, "Enter"); err != nil {
			return fantasy.NewTextErrorResponse(formatTmuxError("send enter", output, err)), nil
		}
	}

	time.Sleep(tmuxDelay(params.DelayMillis))
	output, err := captureTmux(ctx, params.Session, tmuxLines(params.Lines))
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	result := fmt.Sprintf("Sent input to tmux session: %s", params.Session)
	if strings.TrimSpace(output) != "" {
		result += "\n\n" + output
	}
	metadata := TmuxResponseMetadata{
		Action:  "send",
		Session: params.Session,
		Status:  "running",
		Output:  output,
		Lines:   tmuxLines(params.Lines),
	}
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
}

func runTmuxCapture(ctx context.Context, params TmuxParams) (fantasy.ToolResponse, error) {
	if err := requireTmux(); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if params.Session == "" {
		return fantasy.NewTextErrorResponse("session is required for capture"), nil
	}
	if err := validateTmuxSession(params.Session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if exists, err := tmuxSessionExists(ctx, params.Session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	} else if !exists {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("tmux session not found: %s", params.Session)), nil
	}

	lines := tmuxLines(params.Lines)
	output, err := captureTmux(ctx, params.Session, lines)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if output == "" {
		output = BashNoOutput
	}
	metadata := TmuxResponseMetadata{
		Action:  "capture",
		Session: params.Session,
		Status:  "running",
		Output:  output,
		Lines:   lines,
	}
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(output), metadata), nil
}

func runTmuxList(ctx context.Context) (fantasy.ToolResponse, error) {
	if err := requireTmux(); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	output, err := runTmux(ctx, "list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}")
	trimmed := strings.TrimSpace(output)
	if err != nil {
		if strings.Contains(trimmed, "no server running") || strings.Contains(trimmed, "failed to connect") || strings.Contains(trimmed, "error connecting") {
			metadata := TmuxResponseMetadata{Action: "list", Status: "empty"}
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse("No tmux sessions."), metadata), nil
		}
		return fantasy.NewTextErrorResponse(formatTmuxError("list", output, err)), nil
	}
	if trimmed == "" {
		metadata := TmuxResponseMetadata{Action: "list", Status: "empty"}
		return fantasy.WithResponseMetadata(fantasy.NewTextResponse("No tmux sessions."), metadata), nil
	}

	var lines []string
	lines = append(lines, "Tmux sessions:")
	for _, line := range strings.Split(trimmed, "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			lines = append(lines, "- "+line)
			continue
		}
		attached := "detached"
		if parts[2] != "0" {
			attached = "attached"
		}
		lines = append(lines, fmt.Sprintf("- %s (%s window(s), %s)", parts[0], parts[1], attached))
	}
	result := strings.Join(lines, "\n")
	metadata := TmuxResponseMetadata{Action: "list", Status: "running", Output: result}
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
}

func runTmuxKill(ctx context.Context, permissions permission.Service, workingDir string, params TmuxParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if err := requireTmux(); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if params.Session == "" {
		return fantasy.NewTextErrorResponse("session is required for kill"), nil
	}
	if err := validateTmuxSession(params.Session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	if exists, err := tmuxSessionExists(ctx, params.Session); err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	} else if !exists {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("tmux session not found: %s", params.Session)), nil
	}

	if permissions != nil {
		granted, err := requestTmuxPermission(ctx, permissions, workingDir, call, params, params.Session, "kill", params.Session)
		if err != nil {
			return fantasy.ToolResponse{}, err
		}
		if !granted {
			return NewPermissionDeniedResponse(), nil
		}
	}

	if output, err := runTmux(ctx, "kill-session", "-t", params.Session); err != nil {
		return fantasy.NewTextErrorResponse(formatTmuxError("kill", output, err)), nil
	}
	result := fmt.Sprintf("Tmux session terminated: %s", params.Session)
	metadata := TmuxResponseMetadata{Action: "kill", Session: params.Session, Status: "terminated"}
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
}

func requestTmuxPermission(ctx context.Context, permissions permission.Service, workingDir string, call fantasy.ToolCall, params TmuxParams, session, action, resource string) (bool, error) {
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return false, fmt.Errorf("session ID is required for tmux %s", action)
	}
	return permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        workingDir,
		Resource:    resource,
		ToolCallID:  call.ID,
		ToolName:    TmuxToolName,
		Action:      action,
		Description: tmuxPermissionDescription(action, session, params),
		Params:      tmuxPermissionParams(params, session, action),
	})
}

func tmuxPermissionDescription(action, session string, params TmuxParams) string {
	switch action {
	case "start":
		if params.Command != "" {
			return fmt.Sprintf("Start tmux session %s with command: %s", session, params.Command)
		}
		return fmt.Sprintf("Start tmux session: %s", session)
	case "send":
		return fmt.Sprintf("Send input to tmux session: %s", session)
	case "kill":
		return fmt.Sprintf("Terminate tmux session: %s", session)
	default:
		return fmt.Sprintf("Run tmux %s on session: %s", action, session)
	}
}

func tmuxPermissionParams(params TmuxParams, session, action string) TmuxPermissionsParams {
	return TmuxPermissionsParams{
		Action:      action,
		Session:     session,
		Command:     params.Command,
		Input:       params.Input,
		Enter:       tmuxEnter(params),
		WorkingDir:  params.WorkingDir,
		Lines:       params.Lines,
		DelayMillis: params.DelayMillis,
		Description: params.Description,
	}
}

func tmuxPermissionResource(params TmuxParams, session string) string {
	if params.Command != "" {
		return params.Command
	}
	if params.Input != "" {
		return params.Input
	}
	return session
}

func validateTmuxSession(session string) error {
	if !tmuxSessionNameRE.MatchString(session) {
		return fmt.Errorf("tmux session must match %s", tmuxSessionNameRE.String())
	}
	return nil
}

func tmuxWorkingDir(defaultWorkingDir, requested string) (string, error) {
	workingDir := defaultWorkingDir
	if requested != "" {
		if filepath.IsAbs(requested) {
			workingDir = requested
		} else {
			workingDir = filepath.Join(defaultWorkingDir, requested)
		}
	}
	abs, err := filepath.Abs(workingDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve working directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("failed to access working directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("working directory is not a directory: %s", abs)
	}
	return abs, nil
}

func requireTmux() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return errors.New("tmux executable not found on PATH")
	}
	return nil
}

func tmuxSessionExists(ctx context.Context, session string) (bool, error) {
	output, err := runTmux(ctx, "has-session", "-t", session)
	if err == nil {
		return true, nil
	}
	trimmed := strings.TrimSpace(output)
	if strings.Contains(trimmed, "can't find session") || strings.Contains(trimmed, "no server running") || strings.Contains(trimmed, "failed to connect") || strings.Contains(trimmed, "error connecting") {
		return false, nil
	}
	return false, errors.New(formatTmuxError("check session", output, err))
}

func captureTmux(ctx context.Context, session string, lines int) (string, error) {
	output, err := runTmux(ctx, "capture-pane", "-p", "-t", session, "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		return "", errors.New(formatTmuxError("capture", output, err))
	}
	return TruncateOutput(strings.TrimRight(output, "\r\n")), nil
}

func runTmux(ctx context.Context, args ...string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, tmuxCommandTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "tmux", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func formatTmuxError(action, output string, err error) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Sprintf("tmux %s failed: %v", action, err)
	}
	return fmt.Sprintf("tmux %s failed: %v\n%s", action, err, output)
}

func tmuxEnter(params TmuxParams) bool {
	if params.Enter == nil {
		return true
	}
	return *params.Enter
}

func tmuxLines(lines int) int {
	if lines <= 0 {
		return defaultTmuxCaptureLines
	}
	if lines > maxTmuxCaptureLines {
		return maxTmuxCaptureLines
	}
	return lines
}

func tmuxDelay(delayMillis int) time.Duration {
	if delayMillis <= 0 {
		delayMillis = defaultTmuxDelayMillis
	}
	if delayMillis > maxTmuxDelayMillis {
		delayMillis = maxTmuxDelayMillis
	}
	return time.Duration(delayMillis) * time.Millisecond
}
