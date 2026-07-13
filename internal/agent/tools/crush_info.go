package tools

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/skills"
)

const CrushInfoToolName = "recode_info"

//go:embed crush_info.md
var crushInfoDescription string

type CrushInfoParams struct {
	Detail        string `json:"detail,omitempty" description:"Detail level: summary (default), mcp, skills, or full."`
	SinceRevision string `json:"since_revision,omitempty" description:"Revision from a previous recode_info result. When unchanged, returns only changed=false."`
}

func NewCrushInfoTool(
	cfg *config.ConfigStore,
	lspManager *lsp.Manager,
	allSkills []*skills.Skill,
	activeSkills []*skills.Skill,
	skillTracker *skills.Tracker,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CrushInfoToolName,
		crushInfoDescription,
		func(ctx context.Context, params CrushInfoParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			detail := strings.ToLower(strings.TrimSpace(params.Detail))
			if detail == "" {
				detail = "summary"
			}
			if !slices.Contains([]string{"summary", "mcp", "skills", "full"}, detail) {
				return fantasy.NewTextErrorResponse("detail must be summary, mcp, skills, or full"), nil
			}

			full := buildCrushInfo(cfg, lspManager, allSkills, activeSkills, skillTracker)
			revision := crushInfoRevision(full)
			if strings.TrimSpace(params.SinceRevision) == revision {
				return fantasy.NewTextResponse(fmt.Sprintf("[revision]\nid = %s\nchanged = false", revision)), nil
			}

			output := buildCrushInfoDetail(detail, full, cfg, lspManager, allSkills, activeSkills, skillTracker)
			return fantasy.NewTextResponse(fmt.Sprintf("[revision]\nid = %s\nchanged = true\ndetail = %s\n\n%s", revision, detail, strings.TrimSpace(output))), nil
		},
	)
}

func crushInfoRevision(output string) string {
	sum := sha256.Sum256([]byte(output))
	return fmt.Sprintf("%x", sum[:6])
}

func buildCrushInfoDetail(
	detail string,
	full string,
	cfg *config.ConfigStore,
	lspManager *lsp.Manager,
	allSkills []*skills.Skill,
	activeSkills []*skills.Skill,
	skillTracker *skills.Tracker,
) string {
	if detail == "full" {
		return full
	}

	var b strings.Builder
	switch detail {
	case "mcp":
		writeConfigFiles(&b, cfg)
		writeConfigStaleness(&b, cfg)
		writeMCP(&b, mcp.GetStates(), cfg)
	case "skills":
		writeSkills(&b, allSkills, activeSkills, skillTracker, cfg)
	default:
		writeConfigFiles(&b, cfg)
		writeConfigStaleness(&b, cfg)
		writeModels(&b, cfg)
		writeOptions(&b, cfg)
	}
	return b.String()
}

func buildCrushInfo(cfg *config.ConfigStore, lspManager *lsp.Manager, allSkills []*skills.Skill, activeSkills []*skills.Skill, skillTracker *skills.Tracker) string {
	var b strings.Builder

	writeConfigFiles(&b, cfg)
	writeConfigStaleness(&b, cfg)
	writeModels(&b, cfg)
	writeProviders(&b, cfg)
	writeLSP(&b, lspManager, cfg)
	writeMCP(&b, mcp.GetStates(), cfg)
	writeSkills(&b, allSkills, activeSkills, skillTracker, cfg)
	writeHooks(&b, cfg)
	writePermissions(&b, cfg)
	writeDisabledTools(&b, cfg)
	writeOptions(&b, cfg)
	writeAttribution(&b, cfg)

	return b.String()
}

func writeConfigFiles(b *strings.Builder, cfg *config.ConfigStore) {
	b.WriteString("[config_files]\n")
	fmt.Fprintf(b, "write_target = %s\n", cfg.WritableConfigPath())
	if projectPath := cfg.ProjectConfigPath(); projectPath != "" {
		fmt.Fprintf(b, "project_target = %s\n", projectPath)
	}
	b.WriteString("instruction = Use write_target by default. Use project_target only for an explicitly project-specific override. Loaded files are merge evidence, not alternative targets. For MCP work, use the structured runtime and saved-configuration sections below instead of reopening a loaded config file.\n")
	paths := cfg.LoadedPaths()
	for _, p := range paths {
		fmt.Fprintf(b, "loaded = %s\n", p)
	}
	b.WriteString("\n")
}

func writeConfigStaleness(b *strings.Builder, cfg *config.ConfigStore) {
	staleness := cfg.ConfigStaleness()

	b.WriteString("[config_state]\n")
	fmt.Fprintf(b, "dirty = %v\n", staleness.Dirty)

	if len(staleness.Changed) > 0 {
		sorted := slices.Clone(staleness.Changed)
		slices.Sort(sorted)
		fmt.Fprintf(b, "changed_paths = %s\n", strings.Join(sorted, ", "))
	}

	if len(staleness.Missing) > 0 {
		sorted := slices.Clone(staleness.Missing)
		slices.Sort(sorted)
		fmt.Fprintf(b, "missing_paths = %s\n", strings.Join(sorted, ", "))
	}

	if len(staleness.Errors) > 0 {
		var paths []string
		for path := range staleness.Errors {
			paths = append(paths, path)
		}
		slices.Sort(paths)
		fmt.Fprintf(b, "errors = %s\n", strings.Join(paths, ", "))
	}

	b.WriteString("\n")
}

func writeModels(b *strings.Builder, cfg *config.ConfigStore) {
	c := cfg.Config()
	if len(c.Models) == 0 {
		return
	}
	b.WriteString("[model]\n")
	for _, typ := range []config.SelectedModelType{
		config.SelectedModelTypeLarge,
		config.SelectedModelTypeSmall,
		config.SelectedModelTypeSummary,
		config.SelectedModelTypeReview,
	} {
		m, ok := c.Models[typ]
		if !ok {
			continue
		}
		fmt.Fprintf(b, "%s = %s (%s)\n", typ, m.Model, m.Provider)
	}
	b.WriteString("\n")
}

func writeProviders(b *strings.Builder, cfg *config.ConfigStore) {
	c := cfg.Config()
	type pv struct {
		name  string
		count int
	}
	var providers []pv
	for name, pc := range c.Providers.Seq2() {
		if pc.Disable {
			continue
		}
		providers = append(providers, pv{name: name, count: len(pc.Models)})
	}
	if len(providers) == 0 {
		return
	}
	slices.SortFunc(providers, func(a, b pv) int { return strings.Compare(a.name, b.name) })
	b.WriteString("[providers]\n")
	for _, p := range providers {
		fmt.Fprintf(b, "%s = enabled (%d models)\n", p.name, p.count)
	}
	b.WriteString("\n")
}

func writeLSP(b *strings.Builder, lspManager *lsp.Manager, cfg *config.ConfigStore) {
	// Write runtime LSP clients
	if lspManager != nil && lspManager.Clients().Len() > 0 {
		type entry struct {
			name      string
			state     lsp.ServerState
			fileTypes []string
		}
		var entries []entry
		for name, client := range lspManager.Clients().Seq2() {
			entries = append(entries, entry{
				name:      name,
				state:     client.GetServerState(),
				fileTypes: client.FileTypes(),
			})
		}
		if len(entries) > 0 {
			slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.name, b.name) })
			b.WriteString("[lsp]\n")
			for _, e := range entries {
				stateStr := lspStateString(e.state)
				if len(e.fileTypes) > 0 {
					sorted := slices.Clone(e.fileTypes)
					slices.Sort(sorted)
					fmt.Fprintf(b, "%s = %s (%s)\n", e.name, stateStr, strings.Join(sorted, ", "))
				} else {
					fmt.Fprintf(b, "%s = %s\n", e.name, stateStr)
				}
			}
			b.WriteString("\n")
		}
	}

	// Write configured but not running LSP servers
	c := cfg.Config()
	if len(c.LSP) > 0 {
		runtimeNames := make(map[string]bool)
		if lspManager != nil {
			for name := range lspManager.Clients().Seq2() {
				runtimeNames[name] = true
			}
		}

		type configuredEntry struct {
			name   string
			status string
		}
		var entries []configuredEntry
		for name, lspCfg := range c.LSP {
			// Skip if already in runtime
			if runtimeNames[name] {
				continue
			}
			status := "not_started"
			if lspCfg.Disabled {
				status = "disabled"
			}
			entries = append(entries, configuredEntry{name: name, status: status})
		}

		if len(entries) > 0 {
			slices.SortFunc(entries, func(a, b configuredEntry) int { return strings.Compare(a.name, b.name) })
			b.WriteString("[lsp_configured]\n")
			for _, e := range entries {
				fmt.Fprintf(b, "%s = %s\n", e.name, e.status)
			}
			b.WriteString("\n")
		}
	}
}

func writeMCP(b *strings.Builder, states map[string]mcp.ClientInfo, cfg *config.ConfigStore) {
	// Write runtime MCP states
	if len(states) > 0 {
		type entry struct {
			name        string
			state       mcp.State
			err         error
			tools       int
			resources   int
			connectedAt string
		}
		var entries []entry
		for name, info := range states {
			e := entry{
				name:  name,
				state: info.State,
				err:   info.Error,
			}
			if info.State == mcp.StateConnected {
				e.tools = info.Counts.Tools
				e.resources = info.Counts.Resources
				if !info.ConnectedAt.IsZero() {
					e.connectedAt = info.ConnectedAt.Format("15:04:05")
				}
			}
			entries = append(entries, e)
		}
		slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.name, b.name) })
		b.WriteString("[mcp]\n")
		for _, e := range entries {
			switch e.state {
			case mcp.StateConnected:
				if e.connectedAt != "" {
					fmt.Fprintf(b, "%s = connected (%d tools, %d resources) since %s\n", e.name, e.tools, e.resources, e.connectedAt)
				} else {
					fmt.Fprintf(b, "%s = connected (%d tools, %d resources)\n", e.name, e.tools, e.resources)
				}
			case mcp.StateError:
				if e.err != nil {
					fmt.Fprintf(b, "%s = error: %s\n", e.name, e.err.Error())
				} else {
					fmt.Fprintf(b, "%s = error\n", e.name)
				}
			default:
				fmt.Fprintf(b, "%s = %s\n", e.name, e.state)
			}
		}
		b.WriteString("\n")
	}

	// Write configured but not running MCP servers
	c := cfg.Config()
	if len(c.MCP) > 0 {
		writeMCPConfig(b, c.MCP)

		runtimeNames := make(map[string]bool)
		for name := range states {
			runtimeNames[name] = true
		}

		type configuredEntry struct {
			name   string
			status string
		}
		var entries []configuredEntry
		for name, mcpCfg := range c.MCP {
			// Skip if already in runtime
			if runtimeNames[name] {
				continue
			}
			status := "not_started"
			if mcpCfg.Disabled {
				status = "disabled"
			}
			entries = append(entries, configuredEntry{name: name, status: status})
		}

		if len(entries) > 0 {
			slices.SortFunc(entries, func(a, b configuredEntry) int { return strings.Compare(a.name, b.name) })
			b.WriteString("[mcp_configured]\n")
			for _, e := range entries {
				fmt.Fprintf(b, "%s = %s\n", e.name, e.status)
			}
			b.WriteString("\n")
		}
	}
}

func writeMCPConfig(b *strings.Builder, mcps map[string]config.MCPConfig) {
	type configuredEntry struct {
		name string
		cfg  config.MCPConfig
	}
	var entries []configuredEntry
	for name, cfg := range mcps {
		entries = append(entries, configuredEntry{name: name, cfg: cfg})
	}
	slices.SortFunc(entries, func(a, b configuredEntry) int { return strings.Compare(a.name, b.name) })

	b.WriteString("[mcp_config]\n")
	b.WriteString("note = Current configured MCP shape. Values are redacted; use this before opening crush.json.\n")
	for _, e := range entries {
		parts := []string{"type=" + string(e.cfg.Type)}
		if e.cfg.Command != "" {
			parts = append(parts, "command="+e.cfg.Command)
		}
		if len(e.cfg.Args) > 0 {
			parts = append(parts, "args="+quoteList(redactArgs(e.cfg.Args)))
		}
		if e.cfg.URL != "" {
			parts = append(parts, "url="+summarizeURL(e.cfg.URL))
		}
		if len(e.cfg.Env) > 0 {
			parts = append(parts, "env_keys="+sortedMapKeys(e.cfg.Env))
		}
		if len(e.cfg.Headers) > 0 {
			parts = append(parts, "header_keys="+sortedMapKeys(e.cfg.Headers))
		}
		if len(e.cfg.EnabledTools) > 0 {
			parts = append(parts, "enabled_tools="+quoteList(e.cfg.EnabledTools))
		}
		if len(e.cfg.DisabledTools) > 0 {
			parts = append(parts, "disabled_tools="+quoteList(e.cfg.DisabledTools))
		}
		if e.cfg.Timeout > 0 {
			parts = append(parts, fmt.Sprintf("timeout=%ds", e.cfg.Timeout))
		}
		if e.cfg.ToolTimeout > 0 {
			parts = append(parts, fmt.Sprintf("tool_timeout=%ds", e.cfg.ToolTimeout))
		}
		if e.cfg.Disabled {
			parts = append(parts, "disabled=true")
		}
		fmt.Fprintf(b, "%s = %s\n", e.name, strings.Join(parts, " "))
	}
	b.WriteString("\n")
}

func quoteList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func redactArgs(values []string) []string {
	redacted := slices.Clone(values)
	for i, value := range redacted {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "token") ||
			strings.Contains(lower, "secret") ||
			strings.Contains(lower, "password") ||
			strings.Contains(lower, "apikey") ||
			strings.Contains(lower, "api-key") ||
			strings.Contains(lower, "auth") {
			if strings.Contains(value, "=") {
				redacted[i] = value[:strings.Index(value, "=")+1] + "<redacted>"
			}
			if i+1 < len(redacted) && !strings.HasPrefix(redacted[i+1], "$") {
				redacted[i+1] = "<redacted>"
			}
		}
	}
	return redacted
}

func sortedMapKeys(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return quoteList(keys)
}

func summarizeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "<set>"
	}
	parts := parsed.Scheme + "://" + parsed.Host + parsed.Path
	query := parsed.Query()
	if len(query) == 0 {
		return parts
	}
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return parts + "?keys=" + quoteList(keys)
}

func writeSkills(b *strings.Builder, allSkills []*skills.Skill, activeSkills []*skills.Skill, tracker *skills.Tracker, cfg *config.ConfigStore) {
	var disabled []string
	if cfg.Config().Options != nil {
		disabled = cfg.Config().Options.DisabledSkills
	}
	if len(activeSkills) == 0 && len(disabled) == 0 {
		return
	}

	// Build origin map from the pre-filter list.
	originMap := make(map[string]string, len(allSkills))
	for _, s := range allSkills {
		if s.Builtin {
			originMap[s.Name] = "builtin"
		} else {
			originMap[s.Name] = "user"
		}
	}

	type entry struct {
		name   string
		origin string
		state  string
	}
	var entries []entry

	// Active skills: loaded or unloaded.
	for _, s := range activeSkills {
		state := "unloaded"
		if tracker.IsLoaded(s.Name) {
			state = "loaded"
		}
		origin := originMap[s.Name]
		entries = append(entries, entry{name: s.Name, origin: origin, state: state})
	}

	// Disabled skills.
	for _, name := range disabled {
		origin := originMap[name]
		if origin == "" {
			origin = "user"
		}
		entries = append(entries, entry{name: name, origin: origin, state: "disabled"})
	}

	slices.SortFunc(entries, func(a, b entry) int { return strings.Compare(a.name, b.name) })
	b.WriteString("[skills]\n")
	fmt.Fprintf(b, "loaded_this_session = %d/%d\n", tracker.LoadedCount(), len(activeSkills))
	for _, e := range entries {
		fmt.Fprintf(b, "%s = %s, %s\n", e.name, e.origin, e.state)
	}
	b.WriteString("\n")
}

func writePermissions(b *strings.Builder, cfg *config.ConfigStore) {
	c := cfg.Config()
	overrides := cfg.Overrides()

	if c.Permissions == nil {
		if !overrides.SkipPermissionRequests {
			return
		}
	} else if !overrides.SkipPermissionRequests && len(c.Permissions.AllowedTools) == 0 {
		return
	}
	b.WriteString("[permissions]\n")
	if overrides.SkipPermissionRequests {
		b.WriteString("mode = yolo\n")
	}
	if c.Permissions != nil && len(c.Permissions.AllowedTools) > 0 {
		sorted := slices.Clone(c.Permissions.AllowedTools)
		slices.Sort(sorted)
		fmt.Fprintf(b, "allowed_tools = %s\n", strings.Join(sorted, ", "))
	}
	b.WriteString("\n")
}

func writeDisabledTools(b *strings.Builder, cfg *config.ConfigStore) {
	c := cfg.Config()
	if c.Options == nil || len(c.Options.DisabledTools) == 0 {
		return
	}
	sorted := slices.Clone(c.Options.DisabledTools)
	slices.Sort(sorted)
	b.WriteString("[tools]\n")
	fmt.Fprintf(b, "disabled = %s\n", strings.Join(sorted, ", "))
	b.WriteString("\n")
}

func writeOptions(b *strings.Builder, cfg *config.ConfigStore) {
	c := cfg.Config()
	if c.Options == nil {
		return
	}
	type kv struct {
		key   string
		value string
	}
	var opts []kv

	opts = append(opts, kv{"data_directory", c.Options.DataDirectory})
	opts = append(opts, kv{"debug", fmt.Sprintf("%v", c.Options.Debug)})
	autoLSP := c.Options.AutoLSP == nil || *c.Options.AutoLSP
	opts = append(opts, kv{"auto_lsp", fmt.Sprintf("%v", autoLSP)})
	autoSummarize := !c.Options.DisableAutoSummarize
	opts = append(opts, kv{"auto_summarize", fmt.Sprintf("%v", autoSummarize)})

	slices.SortFunc(opts, func(a, b kv) int { return strings.Compare(a.key, b.key) })
	b.WriteString("[options]\n")
	for _, o := range opts {
		fmt.Fprintf(b, "%s = %s\n", o.key, o.value)
	}
	b.WriteString("\n")
}

func writeAttribution(b *strings.Builder, cfg *config.ConfigStore) {
	c := cfg.Config()
	if c.Options == nil || c.Options.Attribution == nil {
		return
	}
	b.WriteString("[attribution]\n")
	trailerStyle := c.Options.Attribution.TrailerStyle
	if trailerStyle == "" {
		trailerStyle = config.TrailerStyleCoAuthoredBy
	}
	fmt.Fprintf(b, "trailer_style = %s\n", trailerStyle)
	fmt.Fprintf(b, "generated_with = %v\n", c.Options.Attribution.GeneratedWith)
	b.WriteString("\n")
}

func writeHooks(b *strings.Builder, cfg *config.ConfigStore) {
	c := cfg.Config()
	if len(c.Hooks) == 0 {
		return
	}

	type entry struct {
		event   string
		matcher string
		command string
		timeout int
	}
	var entries []entry
	for event, hookList := range c.Hooks {
		for _, h := range hookList {
			entries = append(entries, entry{
				event:   event,
				matcher: h.Matcher,
				command: h.Command,
				timeout: h.Timeout,
			})
		}
	}
	slices.SortFunc(entries, func(a, b entry) int {
		if a.event != b.event {
			return strings.Compare(a.event, b.event)
		}
		return strings.Compare(a.command, b.command)
	})

	b.WriteString("[hooks]\n")
	for _, e := range entries {
		line := fmt.Sprintf("%s = %s", e.event, e.command)
		if e.matcher != "" {
			line = fmt.Sprintf("%s (matcher: %s) = %s", e.event, e.matcher, e.command)
		}
		if e.timeout > 0 && e.timeout != 30 {
			line += fmt.Sprintf(" (timeout: %ds)", e.timeout)
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n")
}

func lspStateString(state lsp.ServerState) string {
	switch state {
	case lsp.StateUnstarted:
		return "unstarted"
	case lsp.StateStarting:
		return "starting"
	case lsp.StateReady:
		return "ready"
	case lsp.StateError:
		return "error"
	case lsp.StateStopped:
		return "stopped"
	case lsp.StateDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}
