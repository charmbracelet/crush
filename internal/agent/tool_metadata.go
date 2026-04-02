package agent

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/plugin"
)

func builtinToolMetadata(name string) tools.ToolMetadata {
	switch name {
	case AgentToolName:
		return tools.ToolMetadata{RiskHint: "delegation", SearchHint: "delegate independent work to subagents", SearchTags: []string{"subagent", "taskgraph"}}
	case tools.ToolSearchToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "read", SearchHint: "load deferred tool definitions and activate them for this run", SearchTags: []string{"discover", "registry", "deferred", "select"}, Direct: true}
	case tools.AgenticFetchToolName:
		return tools.ToolMetadata{ConcurrencySafe: true, RiskHint: "network", SearchHint: "research web content", SearchTags: []string{"web", "search", "fetch"}}
	case tools.BashToolName:
		return tools.ToolMetadata{RiskHint: "execute", SearchHint: "execute shell commands", SearchTags: []string{"shell", "command"}}
	case tools.JobOutputToolName, tools.JobWaitToolName, tools.JobKillToolName:
		return tools.ToolMetadata{RiskHint: "execute", SearchHint: "inspect or control background shell jobs", SearchTags: []string{"shell", "background"}}
	case tools.DownloadToolName:
		return tools.ToolMetadata{ConcurrencySafe: true, RiskHint: "network", SearchHint: "download URL to local file", SearchTags: []string{"network", "file"}}
	case tools.EditToolName, tools.MultiEditToolName, tools.WriteToolName:
		return tools.ToolMetadata{RiskHint: "write", SearchHint: "modify local files", SearchTags: []string{"file", "edit"}, Direct: true}
	case tools.FetchToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "network", SearchHint: "fetch raw URL content", SearchTags: []string{"network", "http", "read"}, Direct: true}
	case tools.GlobToolName, tools.GrepToolName, tools.LSToolName, tools.ViewToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "read", SearchHint: "inspect local files", SearchTags: []string{"read", "filesystem"}, Direct: true}
	case tools.SourcegraphToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "network", SearchHint: "search public repositories", SearchTags: []string{"code-search", "network", "deferred"}, Exposure: tools.ToolExposureDeferred, Direct: true}
	case tools.HistorySearchToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "read", SearchHint: "search session history", SearchTags: []string{"history", "session"}, Direct: true}
	case tools.LongTermMemoryToolName:
		return tools.ToolMetadata{RiskHint: "write", SearchHint: "manage long-term memory entries", SearchTags: []string{"memory", "state"}}
	case tools.TodosToolName:
		return tools.ToolMetadata{RiskHint: "write", SearchHint: "track structured task progress", SearchTags: []string{"task", "progress"}, Direct: true}
	case tools.SendMessageToolName:
		return tools.ToolMetadata{RiskHint: "write", SearchHint: "send mailbox messages to running task graph tasks", SearchTags: []string{"mailbox", "taskgraph"}, Direct: true}
	case tools.TaskStopToolName:
		return tools.ToolMetadata{RiskHint: "write", SearchHint: "request task cancellation through mailbox protocol", SearchTags: []string{"mailbox", "taskgraph", "cancel"}, Direct: true}
	case tools.SubtaskResultToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "read", SearchHint: "fetch complete output from a child session", SearchTags: []string{"subagent", "session", "result"}, Direct: true}
	case tools.DiagnosticsToolName, tools.ReferencesToolName, tools.LSPDeclarationToolName, tools.LSPDefinitionToolName, tools.LSPImplementationToolName, tools.LSPTypeDefinitionToolName, tools.LSPHoverToolName, tools.LSPDocumentSymbolsToolName, tools.LSPWorkspaceSymbolsToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "read", SearchHint: "query language-server code intelligence", SearchTags: []string{"lsp", "code-intelligence"}, Direct: true}
	case tools.LSPCodeActionToolName, tools.LSPRenameToolName, tools.LSPFormatToolName:
		return tools.ToolMetadata{RiskHint: "write", SearchHint: "apply language-server powered workspace edits", SearchTags: []string{"lsp", "code-intelligence", "edit"}}
	case tools.LSPRestartToolName:
		return tools.ToolMetadata{RiskHint: "execute", SearchHint: "restart language-server clients", SearchTags: []string{"lsp", "lifecycle"}}
	case tools.ListMCPResourcesToolName, tools.ReadMCPResourceToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "network", SearchHint: "inspect MCP server resources", SearchTags: []string{"mcp", "resource"}, Direct: true}
	case tools.RequestUserInputToolName, tools.PlanExitToolName:
		return tools.ToolMetadata{ReadOnly: true, ConcurrencySafe: true, RiskHint: "read", SearchHint: "plan mode interaction control", SearchTags: []string{"plan", "interaction"}, Direct: true}
	default:
		return tools.ToolMetadata{RiskHint: "execute"}
	}
}

func metadataFromPluginToolDefinition(def plugin.ToolDefinition) tools.ToolMetadata {
	metadata := builtinToolMetadata(def.Name)
	if len(def.Metadata) == 0 {
		if metadata.RiskHint == "" {
			metadata.RiskHint = "execute"
		}
		if metadata.SearchHint == "" {
			metadata.SearchHint = strings.TrimSpace(def.Description)
		}
		return metadata
	}

	if value, ok := boolMetadataValue(def.Metadata, "read_only"); ok {
		metadata.ReadOnly = value
	}
	if value, ok := boolMetadataValue(def.Metadata, "concurrency_safe"); ok {
		metadata.ConcurrencySafe = value
	}
	if value, ok := boolMetadataValue(def.Metadata, "direct"); ok {
		metadata.Direct = value
	}
	if value, ok := stringMetadataValue(def.Metadata, "risk_hint"); ok {
		metadata.RiskHint = value
	}
	if value, ok := stringMetadataValue(def.Metadata, "search_hint"); ok {
		metadata.SearchHint = value
	}
	if value, ok := stringMetadataValue(def.Metadata, "exposure"); ok {
		metadata.Exposure = tools.ToolExposure(value)
	}
	if values := stringSliceMetadataValue(def.Metadata, "search_tags"); len(values) > 0 {
		metadata.SearchTags = values
	}
	if metadata.SearchHint == "" {
		metadata.SearchHint = strings.TrimSpace(def.Description)
	}
	if metadata.RiskHint == "" {
		metadata.RiskHint = "execute"
	}
	return metadata
}

func boolMetadataValue(metadata map[string]any, key string) (bool, bool) {
	raw, ok := metadata[key]
	if !ok {
		return false, false
	}
	switch value := raw.(type) {
	case bool:
		return value, true
	case string:
		normalized := strings.TrimSpace(strings.ToLower(value))
		switch normalized {
		case "true", "1", "yes", "on":
			return true, true
		case "false", "0", "no", "off":
			return false, true
		}
	}
	return false, false
}

func stringMetadataValue(metadata map[string]any, key string) (string, bool) {
	raw, ok := metadata[key]
	if !ok {
		return "", false
	}
	switch value := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value)), true
	}
}

func stringSliceMetadataValue(metadata map[string]any, key string) []string {
	raw, ok := metadata[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return normalizeSearchTags(value)
	case []any:
		items := make([]string, 0, len(value))
		for _, item := range value {
			items = append(items, fmt.Sprintf("%v", item))
		}
		return normalizeSearchTags(items)
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		parts := strings.Split(value, ",")
		return normalizeSearchTags(parts)
	default:
		return nil
	}
}

func normalizeSearchTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
