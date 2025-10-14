package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type ReferencesParams struct {
	FilePath           string `json:"file_path"`
	Line               int    `json:"line"`
	Character          int    `json:"character"`
	IncludeDeclaration *bool  `json:"include_declaration,omitempty"`
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
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file containing the symbol",
			},
			"line": map[string]any{
				"type":        "integer",
				"description": "The line number (0-based) where the symbol is located",
			},
			"character": map[string]any{
				"type":        "integer",
				"description": "The character position (0-based) where the symbol is located",
			},
			"include_declaration": map[string]any{
				"type":        "boolean",
				"description": "Whether to include the declaration in the results (default: true)",
			},
		},
		Required: []string{"file_path", "line", "character"},
	}
}

func (r *referencesTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ReferencesParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if r.lspClients.Len() == 0 {
		return NewTextErrorResponse("no LSP clients available"), nil
	}

	// Default to including declaration
	includeDeclaration := true
	if params.IncludeDeclaration != nil {
		includeDeclaration = *params.IncludeDeclaration
	}

	// Find the appropriate LSP client for this file
	var client *lsp.Client
	for c := range r.lspClients.Seq() {
		if c.HandlesFile(params.FilePath) {
			client = c
			break
		}
	}

	if client == nil {
		return NewTextErrorResponse(fmt.Sprintf("no LSP client available for file: %s", params.FilePath)), nil
	}

	// Get absolute path
	absPath, err := filepath.Abs(params.FilePath)
	if err != nil {
		return NewTextErrorResponse(fmt.Sprintf("failed to get absolute path: %s", err)), nil
	}

	// Check if file exists
	if _, err := os.Stat(absPath); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("file does not exist: %s", absPath)), nil
	}

	// Find references
	locations, err := client.FindReferences(ctx, absPath, params.Line, params.Character, includeDeclaration)
	if err != nil {
		slog.Error("Failed to find references", "error", err)
		return NewTextErrorResponse(fmt.Sprintf("failed to find references: %s", err)), nil
	}

	if len(locations) == 0 {
		return NewTextResponse("No references found"), nil
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
