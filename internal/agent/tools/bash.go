package tools

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/toolruntime"
)

type BashParams struct {
	Description         string `json:"description" description:"A brief description of what the command does, try to keep it under 30 characters or so"`
	Command             string `json:"command" description:"The command to execute"`
	WorkingDir          string `json:"working_dir,omitempty" description:"The working directory to execute the command in (defaults to current directory)"`
	RunInBackground     bool   `json:"run_in_background,omitempty" description:"Set to true to run this command in the background. Use job_output for snapshots, job_wait to block until it finishes, and job_kill to terminate it."`
	TimeoutSeconds      *int   `json:"timeout_seconds,omitempty" description:"Maximum time to allow the command to run in the foreground before it is terminated. Defaults to 120 seconds. Set to 0 to disable timeout."`
	AutoBackgroundAfter int    `json:"auto_background_after,omitempty" description:"Deprecated compatibility field. If provided and timeout_seconds is omitted, its value is interpreted as timeout_seconds."`
}

type BashPermissionsParams struct {
	Description         string `json:"description"`
	Command             string `json:"command"`
	WorkingDir          string `json:"working_dir"`
	RunInBackground     bool   `json:"run_in_background"`
	TimeoutSeconds      *int   `json:"timeout_seconds"`
	AutoBackgroundAfter int    `json:"auto_background_after"`
}

type BashResponseMetadata struct {
	StartTime        int64    `json:"start_time"`
	EndTime          int64    `json:"end_time"`
	Output           string   `json:"output"`
	Description      string   `json:"description"`
	WorkingDirectory string   `json:"working_directory"`
	Background       bool     `json:"background,omitempty"`
	ShellID          string   `json:"shell_id,omitempty"`
	TimedOut         bool     `json:"timed_out,omitempty"`
	TimeoutSeconds   int      `json:"timeout_seconds,omitempty"`
	DeprecationNotes []string `json:"deprecation_notes,omitempty"`
}

const (
	BashToolName    = "bash"
	MaxOutputLength = 30000
	BashNoOutput    = "no output"
)

//go:embed bash.tpl
var bashDescriptionTmpl []byte

var bashDescriptionTpl = template.Must(
	template.New("bashDescription").
		Parse(string(bashDescriptionTmpl)),
)

type bashDescriptionData struct {
	BannedCommands  string
	MaxOutputLength int
	Attribution     config.Attribution
	ModelName       string
}

var bannedCommands = []string{
	// Network/Download tools
	"alias",
	"aria2c",
	"axel",
	"chrome",
	"curl",
	"curlie",
	"firefox",
	"http-prompt",
	"httpie",
	"links",
	"lynx",
	"nc",
	"safari",
	"scp",
	"ssh",
	"telnet",
	"w3m",
	"wget",
	"xh",

	// System administration
	"doas",
	"su",
	"sudo",

	// Package managers
	"apk",
	"apt",
	"apt-cache",
	"apt-get",
	"dnf",
	"dpkg",
	"emerge",
	"home-manager",
	"makepkg",
	"opkg",
	"pacman",
	"paru",
	"pkg",
	"pkg_add",
	"pkg_delete",
	"portage",
	"rpm",
	"yay",
	"yum",
	"zypper",

	// System modification
	"at",
	"batch",
	"chkconfig",
	"crontab",
	"fdisk",
	"mkfs",
	"mount",
	"parted",
	"service",
	"systemctl",
	"umount",

	// Network configuration
	"firewall-cmd",
	"ifconfig",
	"ip",
	"iptables",
	"netstat",
	"pfctl",
	"route",
	"ufw",
}

func bashDescription(attribution *config.Attribution, modelName string) string {
	bannedCommandsStr := strings.Join(bannedCommands, ", ")
	var out bytes.Buffer
	if err := bashDescriptionTpl.Execute(&out, bashDescriptionData{
		BannedCommands:  bannedCommandsStr,
		MaxOutputLength: MaxOutputLength,
		Attribution:     *attribution,
		ModelName:       modelName,
	}); err != nil {
		panic("failed to execute bash description template: " + err.Error())
	}
	return out.String()
}

func blockFuncs() []shell.BlockFunc {
	return []shell.BlockFunc{
		shell.CommandsBlocker(bannedCommands),
		shell.ArgumentsBlocker("apk", []string{"add"}, nil),
		shell.ArgumentsBlocker("apt", []string{"install"}, nil),
		shell.ArgumentsBlocker("apt-get", []string{"install"}, nil),
		shell.ArgumentsBlocker("dnf", []string{"install"}, nil),
		shell.ArgumentsBlocker("pacman", nil, []string{"-S"}),
		shell.ArgumentsBlocker("pkg", []string{"install"}, nil),
		shell.ArgumentsBlocker("yum", []string{"install"}, nil),
		shell.ArgumentsBlocker("zypper", []string{"install"}, nil),
		shell.ArgumentsBlocker("brew", []string{"install"}, nil),
		shell.ArgumentsBlocker("cargo", []string{"install"}, nil),
		shell.ArgumentsBlocker("gem", []string{"install"}, nil),
		shell.ArgumentsBlocker("go", []string{"install"}, nil),
		shell.ArgumentsBlocker("npm", []string{"install"}, []string{"--global"}),
		shell.ArgumentsBlocker("npm", []string{"install"}, []string{"-g"}),
		shell.ArgumentsBlocker("pip", []string{"install"}, []string{"--user"}),
		shell.ArgumentsBlocker("pip3", []string{"install"}, []string{"--user"}),
		shell.ArgumentsBlocker("pnpm", []string{"add"}, []string{"--global"}),
		shell.ArgumentsBlocker("pnpm", []string{"add"}, []string{"-g"}),
		shell.ArgumentsBlocker("yarn", []string{"global", "add"}, nil),
		shell.ArgumentsBlocker("go", []string{"test"}, []string{"-exec"}),
	}
}

func NewBashTool(permissions permission.Service, workingDir string, attribution *config.Attribution, modelName string, hookMgr *hooks.Manager, opts ...BashToolOptions) fantasy.AgentTool {
	var toolOpts BashToolOptions
	if len(opts) > 0 {
		toolOpts = opts[0]
	}

	description := bashDescription(attribution, modelName)
	if toolOpts.DescriptionOverride != "" {
		description = toolOpts.DescriptionOverride
	}

	execBlockFuncs := blockFuncs()
	if toolOpts.RestrictedToGitReadOnly {
		execBlockFuncs = append(execBlockFuncs, restrictedGitBlockFunc())
	}

	return fantasy.NewAgentTool(
		BashToolName,
		string(description),
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Command == "" {
				return fantasy.NewTextErrorResponse("missing command"), nil
			}

			execWorkingDir := cmp.Or(GetWorkingDirFromContext(ctx), params.WorkingDir, workingDir)
			fallbackCommand := ""
			sessionID := GetSessionFromContext(ctx)

			if hookMgr != nil {
				hookResult, hookErr := hookMgr.RunPreToolUse(ctx, BashToolName, map[string]any{
					"command":     params.Command,
					"working_dir": execWorkingDir,
				}, sessionID)
				if hookErr == nil && hookResult != nil {
					switch hookResult.Decision {
					case hooks.DecisionDeny:
						reason := hookResult.Reason
						if reason == "" {
							reason = "command blocked by hook"
						}
						return fantasy.NewTextErrorResponse(reason), nil
					case hooks.DecisionModify:
						if cmd, ok := hookResult.ModifiedInput["command"].(string); ok && cmd != "" {
							params.Command = cmd
						}
						if wd, ok := hookResult.ModifiedInput["working_dir"].(string); ok && wd != "" {
							execWorkingDir = wd
						}
						if hookResult.FallbackOnError {
							if cmd, ok := hookResult.FallbackInput["command"].(string); ok && cmd != "" && cmd != params.Command {
								fallbackCommand = cmd
							}
						}
					}
				}
			}

			if toolOpts.RestrictedToGitReadOnly {
				if err := validateRestrictedGitCommand(params.Command); err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				if fallbackCommand != "" {
					if err := validateRestrictedGitCommand(fallbackCommand); err != nil {
						return fantasy.NewTextErrorResponse(err.Error()), nil
					}
				}
			}

			if toolOpts.DisableBackground && params.RunInBackground {
				return fantasy.NewTextErrorResponse("background execution is disabled for this bash tool"), nil
			}

			isSafeReadOnly := false
			if toolOpts.RestrictedToGitReadOnly {
				isSafeReadOnly = true
			} else {
				cmdLower := strings.ToLower(params.Command)
				for _, safe := range safeCommands {
					if strings.HasPrefix(cmdLower, safe) {
						if len(cmdLower) == len(safe) || cmdLower[len(safe)] == ' ' || cmdLower[len(safe)] == '-' {
							isSafeReadOnly = true
							break
						}
					}
				}
			}

			fallbackNeedsPermission := false
			if fallbackCommand != "" && !toolOpts.RestrictedToGitReadOnly {
				fallbackLower := strings.ToLower(fallbackCommand)
				fallbackIsSafe := false
				for _, safe := range safeCommands {
					if strings.HasPrefix(fallbackLower, safe) {
						if len(fallbackLower) == len(safe) || fallbackLower[len(safe)] == ' ' || fallbackLower[len(safe)] == '-' {
							fallbackIsSafe = true
							break
						}
					}
				}
				fallbackNeedsPermission = !fallbackIsSafe
			}

			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for executing shell command")
			}

			timeoutSeconds, deprecationNotes := effectiveBashTimeout(params)
			if !isSafeReadOnly {
				if err := requestBashPermission(ctx, permissions, sessionID, call.ID, execWorkingDir, params); err != nil {
					return fantasy.ToolResponse{}, err
				}
			}

			if params.RunInBackground {
				return runBackgroundBash(ctx, call, params, execWorkingDir, execBlockFuncs, timeoutSeconds, deprecationNotes)
			}

			startTime := time.Now()
			commandToRun := params.Command
			attemptedFallback := false
			for {
				output, execErr, timedOut, runErr := runForegroundBashCommand(ctx, call.ID, execWorkingDir, execBlockFuncs, commandToRun, timeoutSeconds)
				if runErr != nil {
					return fantasy.ToolResponse{}, runErr
				}

				if shouldRetryOriginalCommand(execErr, commandToRun, fallbackCommand, attemptedFallback) {
					if fallbackNeedsPermission && !attemptedFallback {
						fallbackParams := BashParams{Command: fallbackCommand}
						if err := requestBashPermission(ctx, permissions, sessionID, call.ID, execWorkingDir, fallbackParams); err != nil {
							return fantasy.ToolResponse{}, err
						}
					}
					commandToRun = fallbackCommand
					attemptedFallback = true
					continue
				}

				metadata := BashResponseMetadata{
					StartTime:        startTime.UnixMilli(),
					EndTime:          time.Now().UnixMilli(),
					Output:           output,
					Description:      params.Description,
					WorkingDirectory: execWorkingDir,
					TimeoutSeconds:   timeoutSeconds,
					TimedOut:         timedOut,
				}
				appendDeprecationNotes(&metadata, deprecationNotes)

				responseText := buildBashResponseText(output, execWorkingDir)
				if timedOut {
					timeoutNote := fmt.Sprintf("Command timed out after %d seconds", timeoutSeconds)
					if output == "" {
						metadata.Output = timeoutNote
						responseText = buildBashResponseText(timeoutNote, execWorkingDir)
					} else {
						metadata.Output = output + "\n" + timeoutNote
						responseText = buildBashResponseText(metadata.Output, execWorkingDir)
					}
				}
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(responseText), metadata), nil
			}
		})
}

func requestBashPermission(ctx context.Context, permissions permission.Service, sessionID, toolCallID, workingDir string, params BashParams) error {
	p, err := permissions.Request(ctx,
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        workingDir,
			ToolCallID:  toolCallID,
			ToolName:    BashToolName,
			Action:      "execute",
			Description: fmt.Sprintf("Execute command: %s", params.Command),
			Params: BashPermissionsParams{
				Description:         params.Description,
				Command:             params.Command,
				WorkingDir:          params.WorkingDir,
				RunInBackground:     params.RunInBackground,
				TimeoutSeconds:      params.TimeoutSeconds,
				AutoBackgroundAfter: params.AutoBackgroundAfter,
			},
		},
	)
	if err != nil {
		return err
	}
	if !p {
		return permission.ErrorPermissionDenied
	}
	return nil
}

func runBackgroundBash(ctx context.Context, call fantasy.ToolCall, params BashParams, execWorkingDir string, execBlockFuncs []shell.BlockFunc, timeoutSeconds int, deprecationNotes []string) (fantasy.ToolResponse, error) {
	startTime := time.Now()
	bgManager := shell.GetBackgroundShellManager()
	bgManager.Cleanup()
	bgShell, err := bgManager.StartWithMetadata(context.Background(), execWorkingDir, execBlockFuncs, params.Command, params.Description, GetSessionFromContext(ctx), call.ID, BashToolName)
	if err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("error starting background shell: %w", err)
	}

	publishBashRuntime(ctx, call.ID, toolruntime.StatusBackgroundRunning, "", map[string]any{
		"shell_id":   bgShell.ID,
		"background": true,
	})
	go watchBackgroundShellRuntime(detachedToolRuntimeContext(ctx), bgShell)

	metadata := BashResponseMetadata{
		StartTime:        startTime.UnixMilli(),
		EndTime:          time.Now().UnixMilli(),
		Description:      params.Description,
		WorkingDirectory: bgShell.WorkingDir,
		Background:       true,
		ShellID:          bgShell.ID,
		TimeoutSeconds:   timeoutSeconds,
	}
	appendDeprecationNotes(&metadata, deprecationNotes)

	response := fmt.Sprintf("Background shell started with ID: %s\n\nUse job_output to view a snapshot, job_wait to wait for completion, or job_kill to terminate it.", bgShell.ID)
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
}

func runForegroundBashCommand(ctx context.Context, toolCallID string, execWorkingDir string, execBlockFuncs []shell.BlockFunc, command string, timeoutSeconds int) (string, error, bool, error) {
	execCtx := ctx
	cancel := func() {}
	if timeoutSeconds > 0 {
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	}
	defer cancel()

	runner := shell.NewShell(&shell.Options{
		WorkingDir: execWorkingDir,
		BlockFuncs: execBlockFuncs,
	})
	stdout := &liveOutputBuffer{}
	stderr := &liveOutputBuffer{}
	errCh := make(chan error, 1)

	publishBashRuntime(ctx, toolCallID, toolruntime.StatusRunning, "", nil)
	go func() {
		errCh <- runner.ExecStream(execCtx, command, stdout, stderr)
	}()

	ticker := time.NewTicker(bashStreamThrottle)
	defer ticker.Stop()

	lastSnapshot := ""
	for {
		select {
		case execErr := <-errCh:
			timedOut := execCtx.Err() == context.DeadlineExceeded
			if ctx.Err() != nil && !timedOut {
				publishBashRuntime(ctx, toolCallID, toolruntime.StatusCanceled, truncateOutput(combinedOutputSnapshot(stdout.String(), stderr.String())), nil)
				return "", nil, false, ctx.Err()
			}
			output := finalShellOutput(stdout.String(), stderr.String(), execErr)
			status := toolruntime.StatusCompleted
			switch {
			case timedOut:
				status = toolruntime.StatusFailed
			case shell.IsInterrupt(execErr):
				status = toolruntime.StatusCanceled
			case execErr != nil && shell.ExitCode(execErr) != 0:
				status = toolruntime.StatusFailed
			}
			publishBashRuntime(ctx, toolCallID, status, output, nil)
			return output, execErr, timedOut, nil
		case <-ticker.C:
			snapshot := truncateOutput(combinedOutputSnapshot(stdout.String(), stderr.String()))
			if snapshot == lastSnapshot {
				continue
			}
			lastSnapshot = snapshot
			publishBashRuntime(ctx, toolCallID, toolruntime.StatusRunning, snapshot, nil)
		}
	}
}

func watchBackgroundShellRuntime(ctx context.Context, bgShell *shell.BackgroundShell) {
	if bgShell == nil {
		return
	}

	ticker := time.NewTicker(bashStreamThrottle)
	defer ticker.Stop()
	lastSnapshot := ""

	for {
		stdout, stderr, done, execErr := bgShell.GetOutput()
		snapshot := truncateOutput(combinedOutputSnapshot(stdout, stderr))
		if snapshot != lastSnapshot {
			lastSnapshot = snapshot
			publishShellRuntime(ctx, bgShell, toolruntime.StatusBackgroundRunning, snapshot)
		}
		if done {
			if bgShell.WasKilled() {
				return
			}
			finalSnapshot := finalShellOutput(stdout, stderr, execErr)
			status := toolruntime.StatusCompleted
			switch {
			case shell.IsInterrupt(execErr):
				status = toolruntime.StatusCanceled
			case execErr != nil && shell.ExitCode(execErr) != 0:
				status = toolruntime.StatusFailed
			}
			publishShellRuntime(ctx, bgShell, status, finalSnapshot)
			return
		}
		<-ticker.C
	}
}

type liveOutputBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *liveOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *liveOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func shouldRetryOriginalCommand(execErr error, attemptedCommand, fallbackCommand string, attemptedFallback bool) bool {
	if attemptedFallback || fallbackCommand == "" || attemptedCommand == "" || attemptedCommand == fallbackCommand {
		return false
	}
	if execErr == nil || shell.IsInterrupt(execErr) {
		return false
	}
	return shell.ExitCode(execErr) != 0
}

func formatOutput(stdout, stderr string, execErr error) string {
	interrupted := shell.IsInterrupt(execErr)
	exitCode := shell.ExitCode(execErr)

	stdout = truncateOutput(stdout)
	stderr = truncateOutput(stderr)

	errorMessage := stderr
	if errorMessage == "" && execErr != nil {
		errorMessage = execErr.Error()
	}

	if interrupted {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += "Command was aborted before completion"
	} else if exitCode != 0 {
		if errorMessage != "" {
			errorMessage += "\n"
		}
		errorMessage += fmt.Sprintf("Exit code %d", exitCode)
	}

	hasBothOutputs := stdout != "" && stderr != ""
	if hasBothOutputs {
		stdout += "\n"
	}
	if errorMessage != "" {
		stdout += "\n" + errorMessage
	}
	return strings.TrimPrefix(stdout, "\n")
}

func truncateOutput(content string) string {
	if len(content) <= MaxOutputLength {
		return content
	}

	halfLength := MaxOutputLength / 2
	start := content[:halfLength]
	end := content[len(content)-halfLength:]

	truncatedLinesCount := countLines(content[halfLength : len(content)-halfLength])
	return fmt.Sprintf("%s\n\n... [%d lines truncated] ...\n\n%s", start, truncatedLinesCount, end)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}
