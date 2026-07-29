package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/lsp"
)

type CallHierarchyParams struct {
	Symbol    string `json:"symbol" description:"The symbol name to show call hierarchy for"`
	Direction string `json:"direction" description:"Either 'incoming' (who calls this) or 'outgoing' (what does this call)"`
	Path      string `json:"path,omitempty" description:"The directory to search in. Defaults to the current working directory."`
}

const CallHierarchyToolName = "lsp_call_hierarchy"

//go:embed lsp_call_hierarchy.md
var callHierarchyDescription string

func NewCallHierarchyTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		CallHierarchyToolName,
		callHierarchyDescription,
		func(ctx context.Context, params CallHierarchyParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Symbol == "" {
				return fantasy.NewTextErrorResponse("symbol is required"), nil
			}
			if params.Direction != "incoming" && params.Direction != "outgoing" {
				return fantasy.NewTextErrorResponse("direction must be 'incoming' or 'outgoing'"), nil
			}
			workingDir := cmp.Or(params.Path, ".")
			resolved, err := resolveSymbol(ctx, lspManager, params.Symbol, workingDir)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Symbol '%s' not found", params.Symbol)), nil
			}

			items, err := resolved.client.PrepareCallHierarchy(ctx, resolved.path, resolved.line, resolved.char)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to prepare call hierarchy: %s", err)), nil
			}
			if len(items) == 0 {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("No call hierarchy information for '%s'", params.Symbol)), nil
			}

			item := items[0]

			var b strings.Builder
			fmt.Fprintf(&b, "Call hierarchy for '%s':\n\n", item.Name)

			if params.Direction == "incoming" {
				calls, err := resolved.client.IncomingCalls(ctx, item)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to get incoming calls: %s", err)), nil
				}
				if len(calls) == 0 {
					b.WriteString("No incoming calls found.\n")
				} else {
					fmt.Fprintf(&b, "%d caller(s):\n\n", len(calls))
					for _, c := range calls {
						path, _ := c.From.URI.Path()
						line := c.From.Range.Start.Line + 1
						fmt.Fprintf(&b, "  %s:%d — %s\n", path, line, c.From.Name)
					}
				}
			} else {
				calls, err := resolved.client.OutgoingCalls(ctx, item)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to get outgoing calls: %s", err)), nil
				}
				if len(calls) == 0 {
					b.WriteString("No outgoing calls found.\n")
				} else {
					fmt.Fprintf(&b, "%d callee(s):\n\n", len(calls))
					for _, c := range calls {
						path, _ := c.To.URI.Path()
						line := c.To.Range.Start.Line + 1
						fmt.Fprintf(&b, "  %s:%d — %s\n", path, line, c.To.Name)
					}
				}
			}

			return fantasy.NewTextResponse(b.String()), nil
		},
	)
}
