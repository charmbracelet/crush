package tools

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/hooks"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
)

type BashParams struct {
	Description         string `json:"description" description:"A brief description of what the command does, try to keep it under 30 characters or so"`
	Command             string `json:"command" description:"The command to execute"`
	WorkingDir          string `json:"working_dir,omitempty" description:"The working directory to execute the command in (defaults to current directory)"`
	RunInBackground     bool   `json:"run_in_background,omitempty" description:"Set to true (boolean) to run this command in the background. Use job_output to read the output later."`
	AutoBackgroundAfter int    `json:"auto_background_after,omitempty" description:"Seconds to wait before automatically moving the command to a background job (default: 60)"`
}

type BashPermissionsParams struct {
	Description         string `json:"description"`
	Command             string `json:"command"`
	WorkingDir          string `json:"working_dir"`
	RunInBackground     bool   `json:"run_in_background"`
	AutoBackgroundAfter int    `json:"auto_background_after"`
}

type BashResponseMetadata struct {
	StartTime        int64  `json:"start_time"`
	EndTime          int64  `json:"end_time"`
	Output           string `json:"output"`
	Description      string `json:"description"`
	WorkingDirectory string `json:"working_directory"`
	Background       bool   `json:"background,omitempty"`
	ShellID          string `json:"shell_id,omitempty"`
}

const (
	BashToolName = "bash"

	DefaultAutoBackgroundAfter = 60 // Commands taking longer automatically become background jobs
	MaxOutputLength            = 30000
	BashNoOutput               = "no output"
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
		// this should never happen.
		panic("failed to execute bash description template: " + err.Error())
	}
	return out.String()
}

func blockFuncs() []shell.BlockFunc {
	return []shell.BlockFunc{
		shell.CommandsBlocker(bannedCommands),

		// System package managers
		shell.ArgumentsBlocker("apk", []string{"add"}, nil),
		shell.ArgumentsBlocker("apt", []string{"install"}, nil),
		shell.ArgumentsBlocker("apt-get", []string{"install"}, nil),
		shell.ArgumentsBlocker("dnf", []string{"install"}, nil),
		shell.ArgumentsBlocker("pacman", nil, []string{"-S"}),
		shell.ArgumentsBlocker("pkg", []string{"install"}, nil),
		shell.ArgumentsBlocker("yum", []string{"install"}, nil),
		shell.ArgumentsBlocker("zypper", []string{"install"}, nil),

		// Language-specific package managers
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

		// `go test -exec` can run arbitrary commands
		shell.ArgumentsBlocker("go", []string{"test"}, []string{"-exec"}),
	}
}

func NewBashTool(permissions permission.Service, workingDir string, attribution *config.Attribution, modelName string, hookMgr *hooks.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		BashToolName,
		string(bashDescription(attribution, modelName)),
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Command == "" {
				return fantasy.NewTextErrorResponse("missing command"), nil
			}

			// Determine working directory
			execWorkingDir := cmp.Or(params.WorkingDir, workingDir)
			fallbackCommand := ""

			sessionID := GetSessionFromContext(ctx)

			// Run PreToolUse hooks before permission checks and execution.
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

			isSafeReadOnly := false
			cmdLower := strings.ToLower(params.Command)

			for _, safe := range safeCommands {
				if strings.HasPrefix(cmdLower, safe) {
					if len(cmdLower) == len(safe) || cmdLower[len(safe)] == ' ' || cmdLower[len(safe)] == '-' {
						isSafeReadOnly = true
						break
					}
				}
			}

			// Check if fallback command needs permission (it may be the original unsafe command)
			fallbackNeedsPermission := false
			if fallbackCommand != "" {
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
				if !fallbackIsSafe {
					fallbackNeedsPermission = true
				}
			}

			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for executing shell command")
			}
			if !isSafeReadOnly {
				p, err := permissions.Request(ctx,
					permission.CreatePermissionRequest{
						SessionID:   sessionID,
						Path:        execWorkingDir,
						ToolCallID:  call.ID,
						ToolName:    BashToolName,
						Action:      "execute",
						Description: fmt.Sprintf("Execute command: %s", params.Command),
						Params:      BashPermissionsParams(params),
					},
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !p {
					return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
				}
			}

			// If explicitly requested as background, start immediately with detached context
			if params.RunInBackground {
				startTime := time.Now()
				bgManager := shell.GetBackgroundShellManager()
				bgManager.Cleanup()
				commandToRun := params.Command
				attemptedFallback := false
				for {
					// Use background context so it continues after tool returns
					bgShell, err := bgManager.Start(context.Background(), execWorkingDir, blockFuncs(), commandToRun, params.Description)
					if err != nil {
						return fantasy.ToolResponse{}, fmt.Errorf("error starting background shell: %w", err)
					}

					// Wait a short time to detect fast failures (blocked commands, syntax errors, etc.)
					time.Sleep(1 * time.Second)
					stdout, stderr, done, execErr := bgShell.GetOutput()

					if done {
						// Command failed or completed very quickly
						bgManager.Remove(bgShell.ID)

						if shouldRetryOriginalCommand(execErr, commandToRun, fallbackCommand, attemptedFallback) {
							// Check permission for fallback command if needed
							if fallbackNeedsPermission && !attemptedFallback {
								p, err := permissions.Request(ctx,
									permission.CreatePermissionRequest{
										SessionID:   sessionID,
										Path:        execWorkingDir,
										ToolCallID:  call.ID,
										ToolName:    BashToolName,
										Action:      "execute",
										Description: fmt.Sprintf("Execute command: %s", fallbackCommand),
										Params:      BashPermissionsParams(BashParams{Command: fallbackCommand}),
									},
								)
								if err != nil {
									return fantasy.ToolResponse{}, err
								}
								if !p {
									return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
								}
							}
							commandToRun = fallbackCommand
							attemptedFallback = true
							continue
						}

						interrupted := shell.IsInterrupt(execErr)
						exitCode := shell.ExitCode(execErr)
						if exitCode == 0 && !interrupted && execErr != nil {
							return fantasy.ToolResponse{}, fmt.Errorf("[Job %s] error executing command: %w", bgShell.ID, execErr)
						}

						stdout = formatOutput(stdout, stderr, execErr)

						metadata := BashResponseMetadata{
							StartTime:        startTime.UnixMilli(),
							EndTime:          time.Now().UnixMilli(),
							Output:           stdout,
							Description:      params.Description,
							Background:       params.RunInBackground,
							WorkingDirectory: bgShell.WorkingDir,
						}
						if stdout == "" {
							return fantasy.WithResponseMetadata(fantasy.NewTextResponse(BashNoOutput), metadata), nil
						}
						stdout += fmt.Sprintf("\n\n<cwd>%s</cwd>", normalizeWorkingDir(bgShell.WorkingDir))
						return fantasy.WithResponseMetadata(fantasy.NewTextResponse(stdout), metadata), nil
					}

					// Still running after fast-failure check - return as background job
					metadata := BashResponseMetadata{
						StartTime:        startTime.UnixMilli(),
						EndTime:          time.Now().UnixMilli(),
						Description:      params.Description,
						WorkingDirectory: bgShell.WorkingDir,
						Background:       true,
						ShellID:          bgShell.ID,
					}
					response := fmt.Sprintf("Background shell started with ID: %s\n\nUse job_output tool to view output or job_kill to terminate.", bgShell.ID)
					return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
				}
			}

			// Start synchronous execution with auto-background support
			startTime := time.Now()

			// Start with detached context so it can survive if moved to background
			bgManager := shell.GetBackgroundShellManager()
			bgManager.Cleanup()
			commandToRun := params.Command
			attemptedFallback := false
			for {
				bgShell, err := bgManager.Start(context.Background(), execWorkingDir, blockFuncs(), commandToRun, params.Description)
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("error starting shell: %w", err)
				}

				// Wait for either completion, auto-background threshold, or context cancellation
				ticker := time.NewTicker(100 * time.Millisecond)
				autoBackgroundAfter := cmp.Or(params.AutoBackgroundAfter, DefaultAutoBackgroundAfter)
				autoBackgroundAfter = max(autoBackgroundAfter, 1)
				autoBackgroundThreshold := time.Duration(autoBackgroundAfter) * time.Second
				timeout := time.After(autoBackgroundThreshold)

				var stdout, stderr string
				var done bool
				var execErr error

			waitLoop:
				for {
					select {
					case <-ticker.C:
						stdout, stderr, done, execErr = bgShell.GetOutput()
						if done {
							break waitLoop
						}
					case <-timeout:
						stdout, stderr, done, execErr = bgShell.GetOutput()
						break waitLoop
					case <-ctx.Done():
						ticker.Stop()
						// Incoming context was cancelled before we moved to background
						// Kill the shell and return error
						bgManager.Kill(bgShell.ID)
						return fantasy.ToolResponse{}, ctx.Err()
					}
				}
				ticker.Stop()

				if done {
					// Command completed within threshold - return synchronously
					// Remove from background manager since we're returning directly
					// Don't call Kill() as it cancels the context and corrupts the exit code
					bgManager.Remove(bgShell.ID)

					if shouldRetryOriginalCommand(execErr, commandToRun, fallbackCommand, attemptedFallback) {
						// Check permission for fallback command if needed
						if fallbackNeedsPermission && !attemptedFallback {
							p, err := permissions.Request(ctx,
								permission.CreatePermissionRequest{
									SessionID:   sessionID,
									Path:        execWorkingDir,
									ToolCallID:  call.ID,
									ToolName:    BashToolName,
									Action:      "execute",
									Description: fmt.Sprintf("Execute command: %s", fallbackCommand),
									Params:      BashPermissionsParams(BashParams{Command: fallbackCommand}),
								},
							)
							if err != nil {
								return fantasy.ToolResponse{}, err
							}
							if !p {
								return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
							}
						}
						commandToRun = fallbackCommand
						attemptedFallback = true
						continue
					}

					interrupted := shell.IsInterrupt(execErr)
					exitCode := shell.ExitCode(execErr)
					if exitCode == 0 && !interrupted && execErr != nil {
						return fantasy.ToolResponse{}, fmt.Errorf("[Job %s] error executing command: %w", bgShell.ID, execErr)
					}

					stdout = formatOutput(stdout, stderr, execErr)

					metadata := BashResponseMetadata{
						StartTime:        startTime.UnixMilli(),
						EndTime:          time.Now().UnixMilli(),
						Output:           stdout,
						Description:      params.Description,
						Background:       params.RunInBackground,
						WorkingDirectory: bgShell.WorkingDir,
					}
					if stdout == "" {
						return fantasy.WithResponseMetadata(fantasy.NewTextResponse(BashNoOutput), metadata), nil
					}
					stdout += fmt.Sprintf("\n\n<cwd>%s</cwd>", normalizeWorkingDir(bgShell.WorkingDir))
					return fantasy.WithResponseMetadata(fantasy.NewTextResponse(stdout), metadata), nil
				}

				// Still running - keep as background job
				metadata := BashResponseMetadata{
					StartTime:        startTime.UnixMilli(),
					EndTime:          time.Now().UnixMilli(),
					Description:      params.Description,
					WorkingDirectory: bgShell.WorkingDir,
					Background:       true,
					ShellID:          bgShell.ID,
				}
				response := fmt.Sprintf("Command is taking longer than expected and has been moved to background.\n\nBackground shell ID: %s\n\nUse job_output tool to view output or job_kill to terminate.", bgShell.ID)
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
			}
		})
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

// formatOutput formats the output of a completed command with error handling
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

	return stdout
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

func normalizeWorkingDir(path string) string {
	if runtime.GOOS == "windows" {
		path = strings.ReplaceAll(path, fsext.WindowsWorkingDirDrive(), "")
	}
	return filepath.ToSlash(path)
}
