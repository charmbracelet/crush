package tools

import (
	"cmp"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type ReferencesParams struct {
	Symbol string `json:"symbol" description:"The symbol name to search for (e.g., function name, variable name, type name)"`
	Path   string `json:"path,omitempty" description:"The directory to search in. Use a directory/file to narrow down the symbol search. Defaults to the current working directory."`
}

const ReferencesToolName = "lsp_references"

//go:embed references.md
var referencesDescription string

func NewReferencesTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ReferencesToolName,
		referencesDescription,
		func(ctx context.Context, params ReferencesParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Symbol == "" {
				return fantasy.NewTextErrorResponse("symbol is required"), nil
			}

			workingDir := cmp.Or(params.Path, ".")
			results, err := resolveSymbolResults(ctx, lspManager, params.Symbol, workingDir)
			if err != nil {
				return fantasy.NewTextResponse(fmt.Sprintf("Symbol '%s' not found", params.Symbol)), nil
			}

			var allLocations []protocol.Location
			var allErrs error
			for _, r := range results {
				locations, err := r.client.FindReferences(ctx, r.path, r.line, r.char, true)
				if err != nil {
					if strings.Contains(err.Error(), "no identifier found") {
						continue
					}
					slog.Error("Failed to find references", "error", err, "symbol", params.Symbol, "path", r.path, "line", r.line)
					allErrs = errors.Join(allErrs, err)
					continue
				}
				allLocations = append(allLocations, locations...)
				// LSP returns all references for the symbol, not just from this file.
				if len(locations) > 0 {
					break
				}
			}

			if len(allLocations) > 0 {
				output := formatReferences(cleanupLocations(allLocations))
				return fantasy.NewTextResponse(output), nil
			}

			if allErrs != nil {
				return fantasy.NewTextErrorResponse(allErrs.Error()), nil
			}
			return fantasy.NewTextResponse(fmt.Sprintf("No references found for symbol '%s'", params.Symbol)), nil
		},
	)
}

func groupByFilename(locations []protocol.Location) map[string][]protocol.Location {
	files := make(map[string][]protocol.Location)
	for _, loc := range locations {
		path, err := loc.URI.Path()
		if err != nil {
			slog.Error("Failed to convert location URI to path", "uri", loc.URI, "error", err)
			continue
		}
		files[path] = append(files[path], loc)
	}
	return files
}

func formatReferences(locations []protocol.Location) string {
	fileRefs := groupByFilename(locations)
	files := slices.Collect(maps.Keys(fileRefs))
	sort.Strings(files)

	var output strings.Builder
	fmt.Fprintf(&output, "Found %d reference(s) in %d file(s):\n\n", len(locations), len(files))

	for _, file := range files {
		refs := fileRefs[file]
		fmt.Fprintf(&output, "%s (%d reference(s)):\n", file, len(refs))
		for _, ref := range refs {
			line := ref.Range.Start.Line + 1
			char := ref.Range.Start.Character + 1
			fmt.Fprintf(&output, "  Line %d, Column %d\n", line, char)
		}
		output.WriteString("\n")
	}

	return output.String()
}
