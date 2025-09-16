package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nom-nom-hub/blush/internal/permission"
)

type DepsParams struct {
	Action     string            `json:"action"`               // install, update, remove, list
	Manager    string            `json:"manager,omitempty"`    // npm, pip, go, etc.
	Package    string            `json:"package,omitempty"`    // package name for install/remove
	Version    string            `json:"version,omitempty"`    // specific version
	WorkingDir string            `json:"working_dir,omitempty"` // custom working directory
	Options    map[string]string `json:"options,omitempty"`    // additional options
}

type DepsPermissionsParams struct {
	Action     string            `json:"action"`
	Manager    string            `json:"manager,omitempty"`
	Package    string            `json:"package,omitempty"`
	Version    string            `json:"version,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

type depsTool struct {
	permissions permission.Service
	workingDir  string
}

const (
	DepsToolName    = "deps"
	depsDescription = `Dependency management tool that allows you to inspect, add, update, or remove project dependencies across different package managers.

WHEN TO USE THIS TOOL:
- Use when you need to manage project dependencies
- Helpful for installing new packages or libraries
- Useful for updating existing dependencies
- Perfect for removing unused dependencies
- Great for listing current dependencies

HOW TO USE:
- Specify the action (install, update, remove, list)
- Choose the package manager (npm, pip, go, etc.)
- Provide package name and version if needed
- Optionally specify a custom working directory

FEATURES:
- Supports multiple package managers (npm, pip, go, yarn, pnpm, etc.)
- Can install, update, remove, or list dependencies
- Handles version specifications
- Works with custom working directories
- Provides detailed output of operations

LIMITATIONS:
- Only works with package managers available in the system
- Cannot install packages that don't exist or are inaccessible
- May require internet connection for remote packages

TIPS:
- Use 'list' action first to see current dependencies
- Specify exact versions when possible for reproducible builds
- Use the 'update' action with caution in production environments
- Always verify dependencies after installation`
)

func NewDepsTool(permissions permission.Service, workingDir string) BaseTool {
	return &depsTool{
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (d *depsTool) Name() string {
	return DepsToolName
}

func (d *depsTool) Info() ToolInfo {
	return ToolInfo{
		Name:        DepsToolName,
		Description: depsDescription,
		Parameters: map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "The action to perform (install, update, remove, list)",
				"enum":        []string{"install", "update", "remove", "list"},
			},
			"manager": map[string]any{
				"type":        "string",
				"description": "The package manager to use (npm, pip, go, yarn, pnpm, etc.)",
			},
			"package": map[string]any{
				"type":        "string",
				"description": "The package name to install, update, or remove",
			},
			"version": map[string]any{
				"type":        "string",
				"description": "The specific version of the package",
			},
			"working_dir": map[string]any{
				"type":        "string",
				"description": "Custom working directory (defaults to current project directory)",
			},
			"options": map[string]any{
				"type":        "object",
				"description": "Additional options for the package manager",
			},
		},
		Required: []string{"action"},
	}
}

func (d *depsTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params DepsParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.Action == "" {
		return NewTextErrorResponse("action is required"), nil
	}

	// Validate action
	validActions := map[string]bool{
		"install": true,
		"update":  true,
		"remove":  true,
		"list":    true,
	}
	if !validActions[params.Action] {
		return NewTextErrorResponse(fmt.Sprintf("invalid action: %s. Must be one of: install, update, remove, list", params.Action)), nil
	}

	// Determine working directory
	workingDir := d.workingDir
	if params.WorkingDir != "" {
		if filepath.IsAbs(params.WorkingDir) {
			workingDir = params.WorkingDir
		} else {
			workingDir = filepath.Join(d.workingDir, params.WorkingDir)
		}
	}

	// Check if working directory exists
	if _, err := os.Stat(workingDir); os.IsNotExist(err) {
		return NewTextErrorResponse(fmt.Sprintf("working directory does not exist: %s", workingDir)), nil
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session_id and message_id are required")
	}

	p := d.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        workingDir,
			ToolCallID:  call.ID,
			ToolName:    DepsToolName,
			Action:      params.Action,
			Description: fmt.Sprintf("Manage dependencies with %s action", params.Action),
			Params:      DepsPermissionsParams(params),
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Build command based on action and manager
	cmd, err := d.buildCommand(params, workingDir)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error building command: %s", err)), nil
	}

	// Execute command (we'll use the bash tool for this)
	bashTool := NewBashTool(d.permissions, workingDir)
	bashCall := ToolCall{
		ID:    call.ID,
		Name:  "bash",
		Input: fmt.Sprintf(`{"command": "%s"}`, cmd),
	}
	
	return bashTool.Run(ctx, bashCall)
}

func (d *depsTool) buildCommand(params DepsParams, workingDir string) (string, error) {
	manager := params.Manager
	if manager == "" {
		// Auto-detect package manager based on files in working directory
		manager = d.detectPackageManager(workingDir)
		if manager == "" {
			return "", fmt.Errorf("could not detect package manager and none specified")
		}
	}

	var cmd string
	switch manager {
	case "npm":
		cmd = d.buildNpmCommand(params)
	case "yarn":
		cmd = d.buildYarnCommand(params)
	case "pnpm":
		cmd = d.buildPnpmCommand(params)
	case "pip":
		cmd = d.buildPipCommand(params)
	case "pipenv":
		cmd = d.buildPipenvCommand(params)
	case "poetry":
		cmd = d.buildPoetryCommand(params)
	case "go":
		cmd = d.buildGoCommand(params)
	default:
		return "", fmt.Errorf("unsupported package manager: %s", manager)
	}

	return cmd, nil
}

func (d *depsTool) detectPackageManager(workingDir string) string {
	// Check for package manager specific files
	files := map[string]string{
		"package.json": "npm",
		"yarn.lock":    "yarn",
		"pnpm-lock.yaml": "pnpm",
		"requirements.txt": "pip",
		"Pipfile":      "pipenv",
		"pyproject.toml": "poetry",
		"go.mod":       "go",
	}

	for file, manager := range files {
		if _, err := os.Stat(filepath.Join(workingDir, file)); err == nil {
			return manager
		}
	}

	return ""
}

func (d *depsTool) buildNpmCommand(params DepsParams) string {
	var parts []string
	parts = append(parts, "npm")

	switch params.Action {
	case "install":
		parts = append(parts, "install")
		if params.Package != "" {
			pkg := params.Package
			if params.Version != "" {
				pkg = fmt.Sprintf("%s@%s", pkg, params.Version)
			}
			parts = append(parts, pkg)
		}
	case "update":
		parts = append(parts, "update")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "remove":
		parts = append(parts, "uninstall")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "list":
		parts = append(parts, "list")
	}

	// Add any additional options
	for key, value := range params.Options {
		if value == "" {
			parts = append(parts, fmt.Sprintf("--%s", key))
		} else {
			parts = append(parts, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

func (d *depsTool) buildYarnCommand(params DepsParams) string {
	var parts []string
	parts = append(parts, "yarn")

	switch params.Action {
	case "install":
		if params.Package != "" {
			parts = append(parts, "add")
			pkg := params.Package
			if params.Version != "" {
				pkg = fmt.Sprintf("%s@%s", pkg, params.Version)
			}
			parts = append(parts, pkg)
		} else {
			parts = append(parts, "install")
		}
	case "update":
		parts = append(parts, "upgrade")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "remove":
		parts = append(parts, "remove")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "list":
		parts = append(parts, "list")
	}

	// Add any additional options
	for key, value := range params.Options {
		if value == "" {
			parts = append(parts, fmt.Sprintf("--%s", key))
		} else {
			parts = append(parts, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

func (d *depsTool) buildPnpmCommand(params DepsParams) string {
	var parts []string
	parts = append(parts, "pnpm")

	switch params.Action {
	case "install":
		if params.Package != "" {
			parts = append(parts, "add")
			pkg := params.Package
			if params.Version != "" {
				pkg = fmt.Sprintf("%s@%s", pkg, params.Version)
			}
			parts = append(parts, pkg)
		} else {
			parts = append(parts, "install")
		}
	case "update":
		parts = append(parts, "update")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "remove":
		parts = append(parts, "remove")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "list":
		parts = append(parts, "list")
	}

	// Add any additional options
	for key, value := range params.Options {
		if value == "" {
			parts = append(parts, fmt.Sprintf("--%s", key))
		} else {
			parts = append(parts, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

func (d *depsTool) buildPipCommand(params DepsParams) string {
	var parts []string
	parts = append(parts, "pip")

	switch params.Action {
	case "install":
		parts = append(parts, "install")
		if params.Package != "" {
			pkg := params.Package
			if params.Version != "" {
				pkg = fmt.Sprintf("%s==%s", pkg, params.Version)
			}
			parts = append(parts, pkg)
		}
	case "update":
		parts = append(parts, "install", "--upgrade")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "remove":
		parts = append(parts, "uninstall", "-y")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "list":
		parts = append(parts, "list")
	}

	// Add any additional options
	for key, value := range params.Options {
		if value == "" {
			parts = append(parts, fmt.Sprintf("--%s", key))
		} else {
			parts = append(parts, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

func (d *depsTool) buildPipenvCommand(params DepsParams) string {
	var parts []string
	parts = append(parts, "pipenv")

	switch params.Action {
	case "install":
		parts = append(parts, "install")
		if params.Package != "" {
			pkg := params.Package
			if params.Version != "" {
				pkg = fmt.Sprintf("%s==%s", pkg, params.Version)
			}
			parts = append(parts, pkg)
		}
	case "update":
		parts = append(parts, "update")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "remove":
		parts = append(parts, "uninstall")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "list":
		parts = append(parts, "graph")
	}

	// Add any additional options
	for key, value := range params.Options {
		if value == "" {
			parts = append(parts, fmt.Sprintf("--%s", key))
		} else {
			parts = append(parts, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

func (d *depsTool) buildPoetryCommand(params DepsParams) string {
	var parts []string
	parts = append(parts, "poetry")

	switch params.Action {
	case "install":
		if params.Package != "" {
			parts = append(parts, "add")
			pkg := params.Package
			if params.Version != "" {
				pkg = fmt.Sprintf("%s@%s", pkg, params.Version)
			}
			parts = append(parts, pkg)
		} else {
			parts = append(parts, "install")
		}
	case "update":
		parts = append(parts, "update")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "remove":
		parts = append(parts, "remove")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "list":
		parts = append(parts, "show")
	}

	// Add any additional options
	for key, value := range params.Options {
		if value == "" {
			parts = append(parts, fmt.Sprintf("--%s", key))
		} else {
			parts = append(parts, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}

func (d *depsTool) buildGoCommand(params DepsParams) string {
	var parts []string
	parts = append(parts, "go")

	switch params.Action {
	case "install":
		parts = append(parts, "get")
		if params.Package != "" {
			pkg := params.Package
			if params.Version != "" {
				// For Go, we need to use go install with version
				parts = []string{"go", "install", fmt.Sprintf("%s@%s", pkg, params.Version)}
			} else {
				parts = append(parts, pkg)
			}
		}
	case "update":
		parts = append(parts, "get", "-u")
		if params.Package != "" {
			parts = append(parts, params.Package)
		}
	case "remove":
		// Go doesn't really have a remove command, but we can tidy
		parts = append(parts, "mod", "tidy")
	case "list":
		parts = append(parts, "list", "-m", "all")
	}

	// Add any additional options
	for key, value := range params.Options {
		if value == "" {
			parts = append(parts, fmt.Sprintf("-%s", key))
		} else {
			parts = append(parts, fmt.Sprintf("-%s=%s", key, value))
		}
	}

	return strings.Join(parts, " ")
}