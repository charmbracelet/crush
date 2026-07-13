package tools

import (
	"context"
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

const MCPManageToolName = "mcp_manage"

//go:embed mcp_manage.md
var mcpManageDescription string

type MCPManageParams struct {
	Action string `json:"action" description:"Action to perform: status, refresh, enable, disable, or remove."`
	Name   string `json:"name,omitempty" description:"Exact configured MCP server name. Required except for status and explicit all refresh."`
	Scope  string `json:"scope,omitempty" description:"Configuration scope for enable, disable, or remove: global (default) or project."`
	All    bool   `json:"all,omitempty" description:"Set true only to refresh all configured MCP servers."`
}

type mcpManageDeps struct {
	refresh      mcpRefreshFunc
	getState     func(string) (mcp.ClientInfo, bool)
	setConfig    func(config.Scope, string, any) error
	removeConfig func(config.Scope, string) error
	authorize    func(context.Context, fantasy.ToolCall, config.Scope, MCPManageParams) (bool, error)
}

func NewMCPManageTool(cfg *config.ConfigStore, permissions permission.Service) fantasy.AgentTool {
	return newMCPManageTool(cfg, mcpManageDeps{
		refresh:  mcp.Refresh,
		getState: mcp.GetState,
		setConfig: func(scope config.Scope, key string, value any) error {
			return cfg.SetConfigField(scope, key, value)
		},
		removeConfig: func(scope config.Scope, key string) error {
			return cfg.RemoveConfigField(scope, key)
		},
		authorize: func(ctx context.Context, call fantasy.ToolCall, scope config.Scope, params MCPManageParams) (bool, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return false, fmt.Errorf("session ID is required for MCP configuration")
			}
			path := cfg.WritableConfigPath()
			if scope == config.ScopeWorkspace {
				path = cfg.ProjectConfigPath()
			}
			return permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				ToolCallID:  call.ID,
				ToolName:    MCPManageToolName,
				Description: fmt.Sprintf("%s MCP server %s", params.Action, params.Name),
				Action:      params.Action,
				Params:      params,
				Path:        path,
				Resource:    fmt.Sprintf("%s:mcp.%s", scope.String(), params.Name),
			})
		},
	})
}

func newMCPManageTool(cfg *config.ConfigStore, deps mcpManageDeps) fantasy.AgentTool {
	if deps.refresh == nil {
		deps.refresh = mcp.Refresh
	}
	if deps.getState == nil {
		deps.getState = mcp.GetState
	}
	if deps.setConfig == nil {
		deps.setConfig = func(config.Scope, string, any) error { return nil }
	}
	if deps.removeConfig == nil {
		deps.removeConfig = func(config.Scope, string) error { return nil }
	}
	if deps.authorize == nil {
		deps.authorize = func(context.Context, fantasy.ToolCall, config.Scope, MCPManageParams) (bool, error) {
			return true, nil
		}
	}

	return fantasy.NewAgentTool(
		MCPManageToolName,
		mcpManageDescription,
		func(ctx context.Context, params MCPManageParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			action := strings.ToLower(strings.TrimSpace(params.Action))
			name := strings.TrimSpace(params.Name)
			if action == "" {
				return fantasy.NewTextErrorResponse("action is required: status, refresh, enable, disable, or remove"), nil
			}
			if name != "" && !mcpServerNamePattern.MatchString(name) {
				return fantasy.NewTextErrorResponse("name may contain only letters, numbers, hyphens, and underscores"), nil
			}

			switch action {
			case "status":
				return fantasy.NewTextResponse(formatMCPManageStatus(cfg, deps, name)), nil
			case "refresh":
				if name == "" && !params.All {
					return fantasy.NewTextErrorResponse("name is required for refresh unless all=true"), nil
				}
				if name != "" && params.All {
					return fantasy.NewTextErrorResponse("name and all=true are mutually exclusive"), nil
				}
				target := name
				if params.All {
					target = ""
				}
				return runMCPManageRefresh(ctx, cfg, deps, target)
			case "enable", "disable", "remove":
			default:
				return fantasy.NewTextErrorResponse("action must be status, refresh, enable, disable, or remove"), nil
			}

			if name == "" {
				return fantasy.NewTextErrorResponse("name is required for enable, disable, or remove"), nil
			}
			if _, configured := cfg.Config().MCP[name]; !configured {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("mcp %q is not configured; use mcp_add to create it", name)), nil
			}
			scope, err := parseMCPAddScope(params.Scope)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			allowed, err := deps.authorize(ctx, call, scope, params)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !allowed {
				return NewPermissionDeniedResponse(), nil
			}

			key := "mcp." + name
			switch action {
			case "enable":
				err = deps.setConfig(scope, key+".disabled", false)
			case "disable":
				err = deps.setConfig(scope, key+".disabled", true)
			case "remove":
				err = deps.removeConfig(scope, key)
			}
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("%s %s failed: %v", action, name, err)), nil
			}

			response, err := runMCPManageRefresh(ctx, cfg, deps, name)
			if err != nil {
				return response, err
			}
			if response.IsError {
				return response, nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("%s: %s; %s", name, action, response.Content)), nil
		},
	)
}

func formatMCPManageStatus(cfg *config.ConfigStore, deps mcpManageDeps, requested string) string {
	names := make([]string, 0, len(cfg.Config().MCP))
	for name := range cfg.Config().MCP {
		if requested == "" || name == requested {
			names = append(names, name)
		}
	}
	if requested != "" && len(names) == 0 {
		return fmt.Sprintf("%s: not configured", requested)
	}
	if len(names) == 0 {
		return "No MCP servers are configured."
	}
	slices.Sort(names)

	var output strings.Builder
	for _, name := range names {
		configured := cfg.Config().MCP[name]
		state := "not initialized"
		if info, ok := deps.getState(name); ok {
			state = info.State.String()
		}
		fmt.Fprintf(&output, "%s: %s; disabled=%t\n", name, state, configured.Disabled)
	}
	return strings.TrimSpace(output.String())
}

func runMCPManageRefresh(ctx context.Context, cfg *config.ConfigStore, deps mcpManageDeps, name string) (fantasy.ToolResponse, error) {
	results, err := deps.refresh(ctx, cfg, name)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}
	names := make([]string, 0, len(results))
	for resultName := range results {
		names = append(names, resultName)
	}
	slices.Sort(names)
	if len(names) == 0 {
		return fantasy.NewTextResponse("Configuration reloaded; no MCP clients are configured."), nil
	}

	var output strings.Builder
	hasError := false
	for _, resultName := range names {
		if results[resultName] != nil {
			hasError = true
			fmt.Fprintf(&output, "%s: error: %v\n", resultName, results[resultName])
			continue
		}
		if info, ok := deps.getState(resultName); ok {
			fmt.Fprintf(&output, "%s: %s\n", resultName, info.State)
			continue
		}
		fmt.Fprintf(&output, "%s: removed\n", resultName)
	}
	text := strings.TrimSpace(output.String())
	if hasError {
		return fantasy.NewTextErrorResponse(text), nil
	}
	return fantasy.NewTextResponse(text), nil
}
