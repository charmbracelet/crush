package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

type VSCodeDiffParams struct {
	LeftContent  string `json:"left_content"`
	RightContent string `json:"right_content"`
	LeftTitle    string `json:"left_title,omitempty"`
	RightTitle   string `json:"right_title,omitempty"`
	Language     string `json:"language,omitempty"`
}

type VSCodeDiffPermissionsParams struct {
	LeftContent  string `json:"left_content"`
	RightContent string `json:"right_content"`
	LeftTitle    string `json:"left_title,omitempty"`
	RightTitle   string `json:"right_title,omitempty"`
	Language     string `json:"language,omitempty"`
}

type vscodeDiffTool struct {
	permissions permission.Service
}

const (
	VSCodeDiffToolName = "vscode_diff"
)

func NewVSCodeDiffTool(permissions permission.Service) BaseTool {
	return &vscodeDiffTool{
		permissions: permissions,
	}
}

func (t *vscodeDiffTool) Name() string {
	return VSCodeDiffToolName
}

func (t *vscodeDiffTool) Info() ToolInfo {
	return ToolInfo{
		Name:        VSCodeDiffToolName,
		Description: "Opens VS Code with a diff view comparing two pieces of content. Useful for visualizing code changes, comparing files, or reviewing modifications.",
		Parameters: map[string]any{
			"left_content": map[string]any{
				"type":        "string",
				"description": "The content for the left side of the diff (typically the 'before' or original content)",
			},
			"right_content": map[string]any{
				"type":        "string",
				"description": "The content for the right side of the diff (typically the 'after' or modified content)",
			},
			"left_title": map[string]any{
				"type":        "string",
				"description": "Optional title for the left side (e.g., 'Original', 'Before', or a filename)",
			},
			"right_title": map[string]any{
				"type":        "string",
				"description": "Optional title for the right side (e.g., 'Modified', 'After', or a filename)",
			},
			"language": map[string]any{
				"type":        "string",
				"description": "Optional language identifier for syntax highlighting (e.g., 'javascript', 'python', 'go')",
			},
		},
		Required: []string{"left_content", "right_content"},
	}
}

func (t *vscodeDiffTool) Run(ctx context.Context, params ToolCall) (ToolResponse, error) {
	var diffParams VSCodeDiffParams
	if err := json.Unmarshal([]byte(params.Input), &diffParams); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to parse parameters: %v", err)), nil
	}

	// Check if VS Code is available
	vscodeCmd := getVSCodeCommand()
	if vscodeCmd == "" {
		return NewTextErrorResponse("VS Code is not available. Please install VS Code and ensure 'code' command is in PATH."), nil
	}

	// Check permissions
	sessionID, _ := GetContextValues(ctx)
	permissionParams := VSCodeDiffPermissionsParams(diffParams)

	p := t.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        config.WorkingDirectory(),
			ToolName:    VSCodeDiffToolName,
			Action:      "open_diff",
			Description: fmt.Sprintf("Open VS Code diff view comparing '%s' and '%s'", diffParams.LeftTitle, diffParams.RightTitle),
			Params:      permissionParams,
		},
	)
	if !p {
		return NewTextErrorResponse("Permission denied to open VS Code diff"), nil
	}

	// Create temporary directory for diff files
	tempDir, err := os.MkdirTemp("", "crush-vscode-diff-*")
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("Failed to create temporary directory: %v", err)), nil
	}

	// Determine file extension based on language
	ext := getFileExtension(diffParams.Language)

	// Create temporary files
	leftTitle := diffParams.LeftTitle
	if leftTitle == "" {
		leftTitle = "before"
	}
	rightTitle := diffParams.RightTitle
	if rightTitle == "" {
		rightTitle = "after"
	}

	leftFile := filepath.Join(tempDir, sanitizeFilename(leftTitle)+ext)
	rightFile := filepath.Join(tempDir, sanitizeFilename(rightTitle)+ext)

	// Write content to temporary files
	if err := os.WriteFile(leftFile, []byte(diffParams.LeftContent), 0644); err != nil {
		os.RemoveAll(tempDir)
		return NewTextErrorResponse(fmt.Sprintf("Failed to write left file: %v", err)), nil
	}

	if err := os.WriteFile(rightFile, []byte(diffParams.RightContent), 0644); err != nil {
		os.RemoveAll(tempDir)
		return NewTextErrorResponse(fmt.Sprintf("Failed to write right file: %v", err)), nil
	}

	// Open VS Code with diff view
	cmd := exec.Command(vscodeCmd, "--diff", leftFile, rightFile)

	// Set working directory to current directory
	cwd := config.WorkingDirectory()
	if cwd != "" {
		cmd.Dir = cwd
	}

	if err := cmd.Start(); err != nil {
		os.RemoveAll(tempDir)
		return NewTextErrorResponse(fmt.Sprintf("Failed to open VS Code: %v", err)), nil
	}

	// Clean up temporary files after a delay (VS Code should have opened them by then)
	go func() {
		time.Sleep(5 * time.Second)
		os.RemoveAll(tempDir)
	}()

	response := fmt.Sprintf("Opened VS Code diff view comparing '%s' and '%s'", leftTitle, rightTitle)
	if diffParams.Language != "" {
		response += fmt.Sprintf(" with %s syntax highlighting", diffParams.Language)
	}

	return NewTextResponse(response), nil
}

// getVSCodeCommand returns the appropriate VS Code command for the current platform
func getVSCodeCommand() string {
	// Try common VS Code command names
	commands := []string{"code", "code-insiders"}

	// On macOS, also try the full path
	if runtime.GOOS == "darwin" {
		commands = append(commands,
			"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
			"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
		)
	}

	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd
		}
	}

	return ""
}

// getFileExtension returns the appropriate file extension for syntax highlighting
func getFileExtension(language string) string {
	if language == "" {
		return ".txt"
	}

	extensions := map[string]string{
		"javascript": ".js",
		"typescript": ".ts",
		"python":     ".py",
		"go":         ".go",
		"java":       ".java",
		"c":          ".c",
		"cpp":        ".cpp",
		"csharp":     ".cs",
		"php":        ".php",
		"ruby":       ".rb",
		"rust":       ".rs",
		"swift":      ".swift",
		"kotlin":     ".kt",
		"scala":      ".scala",
		"html":       ".html",
		"css":        ".css",
		"scss":       ".scss",
		"less":       ".less",
		"json":       ".json",
		"xml":        ".xml",
		"yaml":       ".yaml",
		"yml":        ".yml",
		"toml":       ".toml",
		"markdown":   ".md",
		"sql":        ".sql",
		"shell":      ".sh",
		"bash":       ".sh",
		"zsh":        ".sh",
		"fish":       ".fish",
		"powershell": ".ps1",
		"dockerfile": ".dockerfile",
		"makefile":   ".mk",
	}

	if ext, ok := extensions[strings.ToLower(language)]; ok {
		return ext
	}

	return ".txt"
}

// sanitizeFilename removes or replaces characters that are not safe for filenames
func sanitizeFilename(filename string) string {
	// Replace common unsafe characters
	replacements := map[string]string{
		"/":  "_",
		"\\": "_",
		":":  "_",
		"*":  "_",
		"?":  "_",
		"\"": "_",
		"<":  "_",
		">":  "_",
		"|":  "_",
	}

	result := filename
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	// Trim spaces and dots from the beginning and end
	result = strings.Trim(result, " .")

	// If the result is empty, use a default name
	if result == "" {
		result = "file"
	}

	return result
}
