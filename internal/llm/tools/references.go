package tools

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type ReferencesParams struct {
	Symbol string `json:"symbol"`
	Path   string `json:"path"`
}

type referencesTool struct {
	lspClients *csync.Map[string, *lsp.Client]
}

const ReferencesToolName = "references"

//go:embed references.md
var referencesDescription []byte

func NewReferencesTool(lspClients *csync.Map[string, *lsp.Client]) BaseTool {
	return &referencesTool{
		lspClients,
	}
}

func (r *referencesTool) Name() string {
	return ReferencesToolName
}

func (r *referencesTool) Info() ToolInfo {
	return ToolInfo{
		Name:        ReferencesToolName,
		Description: string(referencesDescription),
		Parameters: map[string]any{
			"symbol": map[string]any{
				"type":        "string",
				"description": "The symbol name to search for (e.g., function name, variable name, type name).",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "The directory to search in. Should be the entire project most of the time. Defaults to the current working directory.",
			},
		},
		Required: []string{"symbol"},
	}
}

func (r *referencesTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ReferencesParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.Symbol == "" {
		return NewTextErrorResponse("symbol is required"), nil
	}

	if r.lspClients.Len() == 0 {
		return NewTextErrorResponse("no LSP clients available"), nil
	}

	workingDir := cmp.Or(params.Path, ".")

	matches, _, err := searchFiles(ctx, regexp.QuoteMeta(params.Symbol), workingDir, "", 100)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to search for symbol: %s", err)), nil
	}

	if len(matches) == 0 {
		return NewTextResponse(fmt.Sprintf("Symbol '%s' not found", params.Symbol)), nil
	}

	var allLocations []protocol.Location
	var allErrs error
	for _, match := range matches {
		locations, err := r.find(ctx, match, params.Symbol)
		if err != nil {
			if strings.Contains(err.Error(), "no identifier found") {
				// grep probably matched a comment, string value, or something else that's irrelevant
				continue
			}
			slog.Error("Failed to find references", "error", err, "symbol", params.Symbol, "path", match.path, "line", match.lineNum, "char", match.charNum)
			allErrs = errors.Join(allErrs, err)
			continue
		}
		allLocations = append(allLocations, locations...)
	}

	if len(allLocations) > 0 {
		output := formatReferences(allLocations)
		return NewTextResponse(output), nil
	}

	if allErrs != nil {
		return NewTextErrorResponse(allErrs.Error()), nil
	}
	return NewTextResponse(fmt.Sprintf("No references found for symbol '%s'", params.Symbol)), nil
}

func (r *referencesTool) find(ctx context.Context, match grepMatch, symbol string) ([]protocol.Location, error) {
	absPath, err := filepath.Abs(match.path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %s", err)
	}

	var client *lsp.Client
	for c := range r.lspClients.Seq() {
		if c.HandlesFile(absPath) {
			client = c
			break
		}
	}

	if client == nil {
		slog.Warn("No LSP clients to handle", "path", match.path)
		return nil, nil
	}

	return client.FindReferences(ctx, absPath, match.lineNum-1, match.charNum, true)
}

func formatReferences(locations []protocol.Location) string {
	fileRefs := make(map[string][]protocol.Location)
	for _, loc := range locations {
		path, err := loc.URI.Path()
		if err != nil {
			slog.Error("Failed to convert location URI to path", "uri", loc.URI, "error", err)
			continue
		}
		fileRefs[path] = append(fileRefs[path], loc)
	}

	files := slices.Collect(maps.Keys(fileRefs))
	sort.Strings(files)

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d reference(s) in %d file(s):\n\n", len(locations), len(files)))

	for _, file := range files {
		refs := fileRefs[file]
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].Range.Start.Line != refs[j].Range.Start.Line {
				return refs[i].Range.Start.Line < refs[j].Range.Start.Line
			}
			return refs[i].Range.Start.Character < refs[j].Range.Start.Character
		})

		output.WriteString(fmt.Sprintf("%s (%d reference(s)):\n", file, len(refs)))
		for _, ref := range refs {
			line := ref.Range.Start.Line + 1
			char := ref.Range.Start.Character + 1
			output.WriteString(fmt.Sprintf("  Line %d, Column %d\n", line, char))
		}
		output.WriteString("\n")
	}

	return output.String()
}
