package tools

import (
	"cmp"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/taigrr/fantasy"

	"github.com/taigrr/crush/internal/lsp"
)

const DefinitionToolName = "lsp_definition"

//go:embed definition.md
var definitionDescription string

// DefinitionParams takes the same shape as the references tool so the
// agent can pivot from "find references" to "go to definition" with the
// same inputs.
type DefinitionParams struct {
	Symbol string `json:"symbol" description:"The symbol name to look up (e.g., function name, variable name, type name)"`
	Path   string `json:"path,omitempty" description:"Directory or file to search for the symbol's first occurrence. Defaults to the current working directory."`
}

// NewDefinitionTool returns the lsp_definition tool. The implementation
// mirrors lsp_references: locate the symbol with grep, then ask the
// owning LSP for the canonical location.
func NewDefinitionTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DefinitionToolName,
		definitionDescription,
		func(ctx context.Context, params DefinitionParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Symbol == "" {
				return fantasy.NewTextErrorResponse("symbol is required"), nil
			}
			if lspManager.Clients().Len() == 0 {
				return fantasy.NewTextErrorResponse("no LSP clients available"), nil
			}

			workingDir := cmp.Or(params.Path, ".")

			// Definition only needs one usage site to seed the LSP query;
			// cap matches small to keep latency low.
			matches, _, err := searchFiles(ctx, regexp.QuoteMeta(params.Symbol), workingDir, "", 50)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to search for symbol: %s", err)), nil
			}
			if len(matches) == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("Symbol '%s' not found", params.Symbol)), nil
			}

			var locations []protocol.Location
			var allErrs error
			for _, match := range matches {
				locs, err := findDefinition(ctx, lspManager, params.Symbol, match)
				if err != nil {
					if strings.Contains(err.Error(), "no identifier found") {
						continue
					}
					slog.Error("Failed to find definition", "error", err, "symbol", params.Symbol, "path", match.path, "line", match.lineNum)
					allErrs = errors.Join(allErrs, err)
					continue
				}
				if len(locs) > 0 {
					locations = locs
					break
				}
			}

			if len(locations) > 0 {
				return fantasy.NewTextResponse(formatDefinitions(cleanupLocations(locations))), nil
			}
			if allErrs != nil {
				return fantasy.NewTextErrorResponse(allErrs.Error()), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("No definition found for symbol '%s'", params.Symbol)), nil
		},
	)
}

// findDefinition locates the LSP client that owns match and asks it for
// the symbol's definition.
func findDefinition(ctx context.Context, lspManager *lsp.Manager, symbol string, match grepMatch) ([]protocol.Location, error) {
	absPath, err := filepath.Abs(match.path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %s", err)
	}

	var client *lsp.Client
	for c := range lspManager.Clients().Seq() {
		if c.HandlesFile(absPath) {
			client = c
			break
		}
	}
	if client == nil {
		slog.Warn("No LSP client handles file", "path", match.path)
		return nil, nil
	}

	return client.FindDefinition(ctx, absPath, match.lineNum, match.charNum+getSymbolOffset(symbol))
}

// formatDefinitions reuses the references formatter style so the model
// gets a consistent shape across the LSP tool family.
func formatDefinitions(locations []protocol.Location) string {
	var output strings.Builder
	fmt.Fprintf(&output, "Found %d definition location(s):\n\n", len(locations))
	for _, loc := range locations {
		path, err := loc.URI.Path()
		if err != nil {
			path = string(loc.URI)
		}
		fmt.Fprintf(&output, "%s:%d:%d\n",
			path,
			loc.Range.Start.Line+1,
			loc.Range.Start.Character+1,
		)
	}
	return output.String()
}
