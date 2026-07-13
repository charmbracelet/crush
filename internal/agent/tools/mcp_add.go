package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

const MCPAddToolName = "mcp_add"

//go:embed mcp_add.md
var mcpAddDescription string

var mcpServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type MCPAddParams struct {
	Name           string        `json:"name" description:"Exact user-requested MCP server name."`
	SourceURL      string        `json:"source_url,omitempty" description:"Optional documentation or registry URL supporting this configuration. This is shown during approval but does not gate installation."`
	Scope          string        `json:"scope,omitempty" description:"Configuration scope: global (default) or project."`
	Replace        bool          `json:"replace,omitempty" description:"Allow replacement of a different existing configuration for this exact server."`
	Stdio          *MCPAddStdio  `json:"stdio,omitempty" description:"Local subprocess configuration. Set exactly one of stdio, http, or sse."`
	HTTP           *MCPAddRemote `json:"http,omitempty" description:"Streamable HTTP configuration. Set exactly one of stdio, http, or sse."`
	SSE            *MCPAddRemote `json:"sse,omitempty" description:"Legacy SSE configuration. Set exactly one of stdio, http, or sse."`
	DisabledTools  []string      `json:"disabled_tools,omitempty" description:"Tools to disable after connection."`
	EnabledTools   []string      `json:"enabled_tools,omitempty" description:"Allow list of tools after connection."`
	Timeout        int           `json:"timeout,omitempty" description:"Connection timeout in seconds."`
	ToolTimeout    int           `json:"tool_timeout,omitempty" description:"Individual tool-call timeout in seconds."`
	PollutesMemory *bool         `json:"pollutes_memory,omitempty" description:"Whether successful output can suppress passive memory recording."`
}

type MCPAddStdio struct {
	Command string            `json:"command" description:"Executable only, for example npx. Put every command argument in args."`
	Args    []string          `json:"args,omitempty" description:"Arguments passed to the executable, for example [\"-y\",\"package-name\"]."`
	Env     map[string]string `json:"env,omitempty" description:"Environment variables passed to the subprocess."`
}

type MCPAddRemote struct {
	URL     string            `json:"url" description:"Complete remote MCP endpoint URL."`
	Headers map[string]string `json:"headers,omitempty" description:"HTTP headers passed to the remote MCP server."`
}

type MCPAddPermissionParams struct {
	Name       string   `json:"name"`
	SourceURL  string   `json:"source_url"`
	Scope      string   `json:"scope"`
	Transport  string   `json:"transport"`
	Command    string   `json:"command,omitempty"`
	Args       []string `json:"args,omitempty"`
	URL        string   `json:"url,omitempty"`
	EnvKeys    []string `json:"env_keys,omitempty"`
	HeaderKeys []string `json:"header_keys,omitempty"`
}

type mcpAddDeps struct {
	refresh           mcpRefreshFunc
	getState          func(string) (mcp.ClientInfo, bool)
	getToolFilterInfo func(string) (mcp.ToolFilterInfo, bool)
	setConfig         func(config.Scope, string, any) error
	removeConfig      func(config.Scope, string) error
	authorize         func(context.Context, fantasy.ToolCall, config.Scope, MCPAddPermissionParams) (bool, error)
}

func NewMCPAddTool(cfg *config.ConfigStore, permissions permission.Service) fantasy.AgentTool {
	return newMCPAddTool(cfg, mcpAddDeps{
		refresh:           mcp.Refresh,
		getState:          mcp.GetState,
		getToolFilterInfo: mcp.GetToolFilterInfo,
		setConfig: func(scope config.Scope, key string, value any) error {
			return cfg.SetConfigField(scope, key, value)
		},
		removeConfig: func(scope config.Scope, key string) error {
			return cfg.RemoveConfigField(scope, key)
		},
		authorize: func(ctx context.Context, call fantasy.ToolCall, scope config.Scope, params MCPAddPermissionParams) (bool, error) {
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
				ToolName:    MCPAddToolName,
				Description: fmt.Sprintf("Configure and start MCP server %s", params.Name),
				Action:      "configure",
				Params:      params,
				Path:        path,
				Resource:    fmt.Sprintf("%s:mcp.%s", scope.String(), params.Name),
			})
		},
	})
}

func newMCPAddTool(cfg *config.ConfigStore, deps mcpAddDeps) fantasy.AgentTool {
	if deps.authorize == nil {
		deps.authorize = func(context.Context, fantasy.ToolCall, config.Scope, MCPAddPermissionParams) (bool, error) {
			return true, nil
		}
	}
	if deps.removeConfig == nil {
		deps.removeConfig = func(config.Scope, string) error { return nil }
	}
	return fantasy.NewAgentTool(
		MCPAddToolName,
		mcpAddDescription,
		func(ctx context.Context, _ MCPAddParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			params, err := decodeMCPAddParams(call.Input)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid parameters: %v", err)), nil
			}
			name := strings.TrimSpace(params.Name)
			if !mcpServerNamePattern.MatchString(name) {
				return fantasy.NewTextErrorResponse("name is required and may contain only letters, numbers, hyphens, and underscores"), nil
			}

			scope, err := parseMCPAddScope(params.Scope)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			normalized, permissionParams, err := params.normalized(name, scope)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("%s: invalid_config; configuration_changed=false; %v", name, err)), nil
			}
			existing, configured := cfg.Config().MCP[name]
			if configured && !reflect.DeepEqual(existing, normalized) && !params.Replace {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("%s: already_configured_differently; configuration_present=true; configuration_changed=false; correction_requires_replace=true", name)), nil
			}
			if configured && reflect.DeepEqual(existing, normalized) {
				if state, ok := deps.getState(name); ok && state.State == mcp.StateConnected {
					if filterErr := mcpToolFilterError(name, deps); filterErr != nil {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("%s: invalid_tool_filter; configuration_changed=false; %v", name, filterErr)), nil
					}
					return fantasy.NewTextResponse(formatMCPAddConnected(name, "reused", "unchanged", state)), nil
				}
			}

			allowed, err := deps.authorize(ctx, call, scope, permissionParams)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !allowed {
				response := NewPermissionDeniedResponse()
				response.StopTurn = true
				return response, nil
			}

			action := "reused"
			if !configured || !reflect.DeepEqual(existing, normalized) {
				if err := deps.setConfig(scope, "mcp."+name, normalized); err != nil {
					response := fantasy.NewTextErrorResponse(fmt.Sprintf("%s: config_write_failed; configuration_changed=false; error=%v", name, err))
					response.StopTurn = true
					return response, nil
				}
				if configured {
					action = "updated"
				} else {
					action = "created"
				}
			}

			results, refreshErr := deps.refresh(ctx, cfg, name)
			if refreshErr != nil {
				return rollbackMCPAddFailure(name, action, scope, configured, existing, refreshErr, deps), nil
			}
			if resultErr, ok := results[name]; !ok {
				return rollbackMCPAddFailure(name, action, scope, configured, existing, fmt.Errorf("no refresh result"), deps), nil
			} else if resultErr != nil {
				return rollbackMCPAddFailure(name, action, scope, configured, existing, resultErr, deps), nil
			}
			state, ok := deps.getState(name)
			if !ok {
				return rollbackMCPAddFailure(name, action, scope, configured, existing, fmt.Errorf("no runtime state after start"), deps), nil
			}
			if state.State != mcp.StateConnected {
				if state.Error != nil {
					return rollbackMCPAddFailure(name, action, scope, configured, existing, fmt.Errorf("%s: %w", state.State, state.Error), deps), nil
				}
				return rollbackMCPAddFailure(name, action, scope, configured, existing, fmt.Errorf("%s", state.State), deps), nil
			}
			if filterErr := mcpToolFilterError(name, deps); filterErr != nil {
				return rollbackMCPAddFailure(name, action, scope, configured, existing, filterErr, deps), nil
			}
			scopeLabel := scope.String()
			if action == "reused" {
				scopeLabel = "unchanged"
			}
			return fantasy.NewTextResponse(formatMCPAddConnected(name, action, scopeLabel, state)), nil
		},
	)
}

func mcpToolFilterError(name string, deps mcpAddDeps) error {
	if deps.getToolFilterInfo == nil {
		return nil
	}
	info, ok := deps.getToolFilterInfo(name)
	if !ok || len(info.UnmatchedDisabled) == 0 && len(info.UnmatchedEnabled) == 0 {
		return nil
	}
	return fmt.Errorf(
		"unknown tool filters: disabled=%q enabled=%q; advertised_tools=%q; usable_tools=%q",
		info.UnmatchedDisabled,
		info.UnmatchedEnabled,
		info.Advertised,
		info.Usable,
	)
}

func decodeMCPAddParams(input string) (MCPAddParams, error) {
	var params MCPAddParams
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&params); err != nil {
		return params, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return params, fmt.Errorf("multiple JSON values are not allowed")
		}
		return params, err
	}
	return params, nil
}

func (p MCPAddParams) normalized(name string, scope config.Scope) (config.MCPConfig, MCPAddPermissionParams, error) {
	sourceURL := strings.TrimSpace(p.SourceURL)
	if sourceURL != "" {
		parsedSource, err := url.ParseRequestURI(sourceURL)
		if err != nil || parsedSource.Host == "" || parsedSource.Scheme != "https" && parsedSource.Scheme != "http" {
			return config.MCPConfig{}, MCPAddPermissionParams{}, fmt.Errorf("source_url must be an http or https URL when provided")
		}
	}

	variantCount := 0
	if p.Stdio != nil {
		variantCount++
	}
	if p.HTTP != nil {
		variantCount++
	}
	if p.SSE != nil {
		variantCount++
	}
	if variantCount != 1 {
		return config.MCPConfig{}, MCPAddPermissionParams{}, fmt.Errorf("set exactly one of stdio, http, or sse")
	}

	value := config.MCPConfig{
		DisabledTools:  p.DisabledTools,
		EnabledTools:   p.EnabledTools,
		Timeout:        p.Timeout,
		ToolTimeout:    p.ToolTimeout,
		PollutesMemory: p.PollutesMemory,
	}
	permissionParams := MCPAddPermissionParams{Name: name, SourceURL: sourceURL, Scope: scope.String()}
	switch {
	case p.Stdio != nil:
		value.Type = config.MCPStdio
		value.Command = strings.TrimSpace(p.Stdio.Command)
		value.Args = p.Stdio.Args
		value.Env = p.Stdio.Env
		permissionParams.Transport = string(config.MCPStdio)
		permissionParams.Command = value.Command
		permissionParams.Args = value.Args
		permissionParams.EnvKeys = sortedMCPMapKeys(value.Env)
		if value.Command == "" {
			return value, permissionParams, fmt.Errorf("stdio command is required")
		}
	case p.HTTP != nil:
		value.Type = config.MCPHttp
		value.URL = strings.TrimSpace(p.HTTP.URL)
		value.Headers = p.HTTP.Headers
		permissionParams.Transport = string(config.MCPHttp)
		permissionParams.URL = value.URL
		permissionParams.HeaderKeys = sortedMCPMapKeys(value.Headers)
		if value.URL == "" {
			return value, permissionParams, fmt.Errorf("http url is required")
		}
	case p.SSE != nil:
		value.Type = config.MCPSSE
		value.URL = strings.TrimSpace(p.SSE.URL)
		value.Headers = p.SSE.Headers
		permissionParams.Transport = string(config.MCPSSE)
		permissionParams.URL = value.URL
		permissionParams.HeaderKeys = sortedMCPMapKeys(value.Headers)
		if value.URL == "" {
			return value, permissionParams, fmt.Errorf("sse url is required")
		}
	}
	return value, permissionParams, nil
}

func sortedMCPMapKeys(value map[string]string) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func formatMCPAddConnected(name, action, scope string, state mcp.ClientInfo) string {
	return fmt.Sprintf("%s: connected; config=%s; scope=%s; tools=%d; prompts=%d; resources=%d", name, action, scope, state.Counts.Tools, state.Counts.Prompts, state.Counts.Resources)
}

func rollbackMCPAddFailure(name, action string, scope config.Scope, configured bool, existing config.MCPConfig, startErr error, deps mcpAddDeps) fantasy.ToolResponse {
	present := configured
	changed := false
	rollback := "not_needed"
	if action == "created" {
		rollback = "removed"
		if err := deps.removeConfig(scope, "mcp."+name); err != nil {
			present = true
			changed = true
			rollback = "failed: " + err.Error()
		}
	} else if action == "updated" {
		rollback = "restored"
		if err := deps.setConfig(scope, "mcp."+name, existing); err != nil {
			present = true
			changed = true
			rollback = "failed: " + err.Error()
		}
	}
	response := fantasy.NewTextErrorResponse(formatMCPAddFailure(name, action, scope, present, changed, rollback, startErr))
	response.StopTurn = true
	return response
}

func formatMCPAddFailure(name, action string, scope config.Scope, present, changed bool, rollback string, err error) string {
	scopeLabel := scope.String()
	if action == "reused" {
		scopeLabel = "unchanged"
	}
	return fmt.Sprintf("%s: start_failed; candidate=%s; scope=%s; configuration_present=%t; configuration_changed=%t; rollback=%s; correction_requires_replace=%t; error=%v", name, action, scopeLabel, present, changed, rollback, present, err)
}

func parseMCPAddScope(value string) (config.Scope, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "global":
		return config.ScopeGlobal, nil
	case "project", "workspace":
		return config.ScopeWorkspace, nil
	default:
		return config.ScopeGlobal, fmt.Errorf("scope must be global or project")
	}
}
