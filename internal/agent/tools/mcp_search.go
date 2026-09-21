package tools

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools/mcp"
	"github.com/charmbracelet/crush/internal/config"
)

type MCPSearchParams struct {
	Query string `json:"query" description:"What you need the tool to do, e.g. create a pull request. Also accepts 'server:<name>' to load a whole server or 'select:<tool>,<tool>' for exact names."`
	Limit int    `json:"limit,omitempty" description:"Maximum number of tools to load (default 8, max 32)"`
}

const MCPSearchToolName = "mcp_search"

//go:embed mcp_search.md.tpl
var mcpSearchDescriptionTmpl []byte

var mcpSearchDescriptionTpl = template.Must(
	template.New("mcpSearchDescription").
		Parse(string(mcpSearchDescriptionTmpl)),
)

func mcpSearchDescription() string {
	return renderTemplate(mcpSearchDescriptionTpl, struct {
		MaxLoaded int
	}{
		MaxLoaded: mcp.MaxActivatedPerSession,
	})
}

// NewMCPSearchTool builds the discovery tool that makes hidden MCP tools
// callable. Searching is what keeps MCP out of the model context: instead of
// paying for every server's schemas on every step, the model spends one call
// on the tools it actually needs.
func NewMCPSearchTool(cfg *config.ConfigStore) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		MCPSearchToolName,
		mcpSearchDescription(),
		func(ctx context.Context, params MCPSearchParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for searching MCP tools")
			}

			refs := mcp.Search(cfg, params.Query, params.Limit)
			names := make([]string, 0, len(refs))
			for _, ref := range refs {
				names = append(names, ref.Name)
			}
			// Activation is unconditional, so an empty query doubles as
			// "show me what exists" without special-casing the response.
			newly := mcp.Activate(sessionID, names...)

			return fantasy.NewTextResponse(renderSearchResult(
				refs,
				len(refs)-len(newly),
				mcp.DeferredCount(cfg, sessionID),
				mcp.IndexLines(cfg),
			)), nil
		},
	)
}

// renderSearchResult formats a search outcome for the model.
//
// The wording matters: a model that finishes a search and finds its tool
// missing from the current request will otherwise conclude the tool does not
// exist and stop. Both the "callable on your next step" line and the
// machine-readable LOADED list exist to keep it moving.
func renderSearchResult(refs []mcp.ToolRef, alreadyLoaded, hiddenRemaining int, indexLines []string) string {
	if len(refs) == 0 {
		var out strings.Builder
		out.WriteString("No matching MCP tools found.\n")
		if hiddenRemaining == 0 {
			out.WriteString("No MCP tools are hidden right now: every connected server's tools are already available, or no server is connected.")
			return out.String()
		}
		out.WriteString("Retry with a broader query, or load a whole server with \"server:<name>\":\n")
		for _, line := range indexLines {
			out.WriteString(line + "\n")
		}
		return strings.TrimRight(out.String(), "\n")
	}

	out := strings.Builder{}
	fmt.Fprintf(&out,
		"Loaded %d MCP tool%s. They are callable on your next step - continue the task now, do not ask the user to proceed.\n\n",
		len(refs), toolPlural(len(refs)),
	)
	for _, ref := range refs {
		out.WriteString("- " + ref.Name)
		if ref.Description != "" {
			out.WriteString(": " + ref.Description)
		}
		if len(ref.Required) > 0 {
			out.WriteString(" [required: " + strings.Join(ref.Required, ", ") + "]")
		}
		out.WriteString("\n")
	}
	out.WriteString("\nLOADED: " + strings.Join(names(refs), ",") + "\n")
	if alreadyLoaded > 0 {
		fmt.Fprintf(&out, "(%d were already loaded)\n", alreadyLoaded)
	}
	fmt.Fprintf(&out, "Hidden MCP tools remaining: %d", hiddenRemaining)
	return strings.TrimRight(out.String(), "\n")
}

func names(refs []mcp.ToolRef) []string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.Name)
	}
	return out
}

func toolPlural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
