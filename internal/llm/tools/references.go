package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
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
				"description": "The directory to search in. Defaults to the current working directory.",
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

	return r.findReferencesBySymbol(ctx, params)
}

func (r *referencesTool) findReferencesBySymbol(ctx context.Context, params ReferencesParams) (ToolResponse, error) {
	// Use grep to find the symbol
	workingDir := "."
	if params.Path != "" {
		workingDir = params.Path
	}

	matches, _, err := searchFiles(ctx, regexp.QuoteMeta(params.Symbol), workingDir, "", 100)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to search for symbol: %s", err)), nil
	}

	if len(matches) == 0 {
		return NewTextResponse(fmt.Sprintf("Symbol '%s' not found", params.Symbol)), nil
	}

	// Try to find references for the first match
	firstMatch := matches[0]
	absPath, err := filepath.Abs(firstMatch.path)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to get absolute path: %s", err)), nil
	}

	// Find the appropriate LSP client for this file
	var client *lsp.Client
	for c := range r.lspClients.Seq() {
		if c.HandlesFile(absPath) {
			client = c
			break
		}
	}

	if client == nil {
		return NewTextErrorResponse(fmt.Sprintf("no LSP client available for file: %s", absPath)), nil
	}

	// Read the file to find the exact character position
	content, err := os.ReadFile(absPath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to read file: %s", err)), nil
	}

	lines := strings.Split(string(content), "\n")
	if firstMatch.lineNum < 1 || firstMatch.lineNum > len(lines) {
		return NewTextErrorResponse(fmt.Sprintf("invalid line number: %d", firstMatch.lineNum)), nil
	}

	line := lines[firstMatch.lineNum-1]
	charPos := strings.Index(line, params.Symbol)
	if charPos == -1 {
		return NewTextErrorResponse(fmt.Sprintf("symbol not found on line %d", firstMatch.lineNum)), nil
	}

	// Find references using LSP (line numbers are 0-based in LSP)
	locations, err := client.FindReferences(ctx, absPath, firstMatch.lineNum-1, charPos, true)
	if err != nil {
		slog.Error("Failed to find references", "error", err)
		return NewTextErrorResponse(fmt.Sprintf("failed to find references: %s", err)), nil
	}

	if len(locations) == 0 {
		return NewTextResponse(fmt.Sprintf("No references found for symbol '%s'", params.Symbol)), nil
	}

	output := formatReferences(locations)
	return NewTextResponse(output), nil
}

func formatReferences(locations []protocol.Location) string {
	// Group references by file
	fileRefs := make(map[string][]protocol.Location)
	for _, loc := range locations {
		path, err := loc.URI.Path()
		if err != nil {
			slog.Error("Failed to convert location URI to path", "uri", loc.URI, "error", err)
			continue
		}
		fileRefs[path] = append(fileRefs[path], loc)
	}

	// Sort files
	files := make([]string, 0, len(fileRefs))
	for file := range fileRefs {
		files = append(files, file)
	}
	sort.Strings(files)

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Found %d reference(s) in %d file(s):\n\n", len(locations), len(files)))

	for _, file := range files {
		refs := fileRefs[file]

		// Sort references by line number
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
