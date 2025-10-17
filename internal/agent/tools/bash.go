package tools

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
)

type BashParams struct {
	Command     string `json:"command" description:"The command to execute"`
	Description string `json:"description,omitempty" description:"A brief description of what the command does"`
	Timeout     int    `json:"timeout,omitempty" description:"Optional timeout in milliseconds (max 600000)"`
}

type BashPermissionsParams struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	Timeout     int    `json:"timeout"`
}

type BashResponseMetadata struct {
	StartTime        int64  `json:"start_time"`
	EndTime          int64  `json:"end_time"`
	Output           string `json:"output"`
	Description      string `json:"description"`
	WorkingDirectory string `json:"working_directory"`
}

const (
	BashToolName = "bash"

	DefaultTimeout  = 1 * 60 * 1000  // 1 minutes in milliseconds
	MaxTimeout      = 10 * 60 * 1000 // 10 minutes in milliseconds
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

func bashDescription(attribution *config.Attribution) string {
	bannedCommandsStr := strings.Join(bannedCommands, ", ")
	var out bytes.Buffer
	if err := bashDescriptionTpl.Execute(&out, bashDescriptionData{
		BannedCommands:  bannedCommandsStr,
		MaxOutputLength: MaxOutputLength,
		Attribution:     *attribution,
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

func NewBashTool(permissions permission.Service, workingDir string, attribution *config.Attribution) fantasy.AgentTool {
	// Set up command blocking on the persistent shell
	persistentShell := shell.GetPersistentShell(workingDir)
	persistentShell.SetBlockFuncs(blockFuncs())
	return fantasy.NewAgentTool(
		BashToolName,
		string(bashDescription(attribution)),
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Timeout > MaxTimeout {
				params.Timeout = MaxTimeout
			} else if params.Timeout <= 0 {
				params.Timeout = DefaultTimeout
			}

			if params.Command == "" {
				return fantasy.NewTextErrorResponse("missing command"), nil
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

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for executing shell command")
			}
			if !isSafeReadOnly {
				shell := shell.GetPersistentShell(workingDir)
				p := permissions.Request(
					permission.CreatePermissionRequest{
						SessionID:   sessionID,
						Path:        shell.GetWorkingDir(),
						ToolCallID:  call.ID,
						ToolName:    BashToolName,
						Action:      "execute",
						Description: fmt.Sprintf("Execute command: %s", params.Command),
						Params: BashPermissionsParams{
							Command:     params.Command,
							Description: params.Description,
						},
					},
				)
				if !p {
					return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
				}
			}
			startTime := time.Now()
			if params.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Millisecond)
				defer cancel()
			}

			persistentShell := shell.GetPersistentShell(workingDir)
			stdout, stderr, err := persistentShell.Exec(ctx, params.Command)

			// Get the current working directory after command execution
			currentWorkingDir := persistentShell.GetWorkingDir()
			interrupted := shell.IsInterrupt(err)
			exitCode := shell.ExitCode(err)
			if exitCode == 0 && !interrupted && err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error executing command: %w", err)
			}

			stdout = truncateOutput(stdout)
			stderr = truncateOutput(stderr)

			errorMessage := stderr
			if errorMessage == "" && err != nil {
				errorMessage = err.Error()
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

			metadata := BashResponseMetadata{
				StartTime:        startTime.UnixMilli(),
				EndTime:          time.Now().UnixMilli(),
				Output:           stdout,
				Description:      params.Description,
				WorkingDirectory: currentWorkingDir,
			}
			if stdout == "" {
				return fantasy.WithResponseMetadata(fantasy.NewTextResponse(BashNoOutput), metadata), nil
			}
			stdout += fmt.Sprintf("\n\n<cwd>%s</cwd>", currentWorkingDir)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(stdout), metadata), nil
		})
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
