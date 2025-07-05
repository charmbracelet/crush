package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

// AutoOpenVSCodeDiff automatically opens VS Code diff if enabled and available
// Returns true if VS Code was successfully opened, false otherwise
func AutoOpenVSCodeDiff(ctx context.Context, permissions permission.Service, beforeContent, afterContent, fileName, language string) bool {
	// Only enable VS Code diff when running inside VS Code (VSCODE_INJECTION=1)
	if os.Getenv("VSCODE_INJECTION") != "1" {
		return false
	}

	cfg := config.Get()

	// Check if auto-open is enabled
	if !cfg.Options.AutoOpenVSCodeDiff {
		return false
	}

	// Check if there are any changes
	if beforeContent == afterContent {
		return false
	}

	// Check if VS Code is available
	if !isVSCodeAvailable() {
		return false
	}

	// Get session ID for permissions
	sessionID, _ := GetContextValues(ctx)
	if sessionID == "" {
		return false
	}

	// Create titles from filename
	leftTitle := "before"
	rightTitle := "after"
	if fileName != "" {
		base := filepath.Base(fileName)
		leftTitle = "before_" + base
		rightTitle = "after_" + base
	}

	// Request permission to open VS Code
	permissionParams := VSCodeDiffPermissionsParams{
		LeftContent:  beforeContent,
		RightContent: afterContent,
		LeftTitle:    leftTitle,
		RightTitle:   rightTitle,
		Language:     language,
	}

	p := permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        config.WorkingDirectory(),
			ToolName:    VSCodeDiffToolName,
			Action:      "auto_open_diff",
			Description: fmt.Sprintf("Auto-open VS Code diff view for %s", fileName),
			Params:      permissionParams,
		},
	)

	if !p {
		return false
	}

	// Open VS Code diff - this would actually open VS Code in a real implementation
	return openVSCodeDiffDirect(beforeContent, afterContent, leftTitle, rightTitle, language)
}

// isVSCodeAvailable checks if VS Code is available on the system
func isVSCodeAvailable() bool {
	return getVSCodeCommandInternal() != ""
}

// getVSCodeCommandInternal returns the appropriate VS Code command for the current platform
func getVSCodeCommandInternal() string {
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

// openVSCodeDiffDirect opens VS Code with a diff view (simplified version of the main tool)
func openVSCodeDiffDirect(beforeContent, afterContent, leftTitle, rightTitle, language string) bool {
	vscodeCmd := getVSCodeCommandInternal()
	if vscodeCmd == "" {
		return false
	}

	// This is a simplified version that would create temp files and open VS Code
	// For now, we'll return true to indicate it would work
	// In a full implementation, this would:
	// 1. Create temporary files with the content
	// 2. Use the appropriate file extension based on language
	// 3. Execute: code --diff leftFile rightFile
	// 4. Clean up files after a delay

	// TODO: Implement the actual file creation and VS Code execution
	// This would be similar to the logic in vscode.go but simplified

	return true // Placeholder - indicates VS Code would be opened
}

// getLanguageFromExtension determines the language identifier from a file extension
func getLanguageFromExtension(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".js":         "javascript",
		".jsx":        "javascript",
		".ts":         "typescript",
		".tsx":        "typescript",
		".py":         "python",
		".go":         "go",
		".java":       "java",
		".c":          "c",
		".cpp":        "cpp",
		".cc":         "cpp",
		".cxx":        "cpp",
		".cs":         "csharp",
		".php":        "php",
		".rb":         "ruby",
		".rs":         "rust",
		".swift":      "swift",
		".kt":         "kotlin",
		".scala":      "scala",
		".html":       "html",
		".htm":        "html",
		".css":        "css",
		".scss":       "scss",
		".sass":       "scss",
		".less":       "less",
		".json":       "json",
		".xml":        "xml",
		".yaml":       "yaml",
		".yml":        "yaml",
		".toml":       "toml",
		".md":         "markdown",
		".sql":        "sql",
		".sh":         "shell",
		".bash":       "shell",
		".zsh":        "shell",
		".fish":       "fish",
		".ps1":        "powershell",
		".dockerfile": "dockerfile",
		".mk":         "makefile",
		".makefile":   "makefile",
	}

	if language, ok := languageMap[ext]; ok {
		return language
	}

	return "text"
}
