package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/util"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type DefinitionParams struct {
	Symbol string `json:"symbol" description:"The symbol name to find the definition of"`
	Path   string `json:"path,omitempty" description:"The directory to search in. Defaults to the current working directory."`
}

const DefinitionToolName = "lsp_definition"

//go:embed lsp_definition.md
var definitionDescription string

// DefinitionResponseMetadata carries structured data for the renderer.
type DefinitionResponseMetadata struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Content  string `json:"content"`
}

func NewDefinitionTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DefinitionToolName,
		definitionDescription,
		func(ctx context.Context, params DefinitionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Symbol == "" {
				return fantasy.NewTextErrorResponse("symbol is required"), nil
			}
			workingDir := cmp.Or(params.Path, ".")
			resolved, err := resolveSymbol(ctx, lspManager, params.Symbol, workingDir)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Symbol '%s' not found", params.Symbol)), nil
			}

			locations, err := resolved.client.Definition(ctx, resolved.path, resolved.line, resolved.char)
			if err != nil {
				if isNoIdentifierError(err) {
					return fantasy.NewTextResponse(fmt.Sprintf("No definition found for symbol '%s'", params.Symbol)), nil
				}
				slog.Error("Failed to find definition", "error", err, "symbol", params.Symbol)
				return fantasy.NewTextErrorResponse(fmt.Sprintf("definition lookup failed: %s", err)), nil
			}

			if len(locations) == 0 {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("No definition found for symbol '%s'", params.Symbol)), nil
			}

			text, meta := formatDefinitions(locations)
			resp := fantasy.NewTextResponse(text)
			if meta != nil {
				resp = fantasy.WithResponseMetadata(resp, meta)
			}
			return resp, nil
		},
	)
}

func formatDefinitions(locations []protocol.Location) (string, *DefinitionResponseMetadata) {
	locations = cleanupLocations(locations)

	var b strings.Builder
	fmt.Fprintf(&b, "Found %d definition(s):\n\n", len(locations))

	var firstMeta *DefinitionResponseMetadata

	for _, loc := range locations {
		path, err := util.PathFromURI(loc.URI)
		if err != nil {
			slog.Error("Failed to convert URI to path", "uri", loc.URI, "error", err)
			continue
		}
		line := loc.Range.Start.Line + 1
		snippet := readSourceContext(path, int(loc.Range.Start.Line), 3)

		fmt.Fprintf(&b, "%s:%d\n", path, line)
		if snippet != "" {
			b.WriteString(snippet)
			b.WriteString("\n")
		}

		// Capture metadata for the first definition (most common case).
		if firstMeta == nil && snippet != "" {
			firstMeta = &DefinitionResponseMetadata{
				FilePath: path,
				Line:     int(loc.Range.Start.Line),
				Content:  readSourceLines(path, int(loc.Range.Start.Line), 3),
			}
		}
	}

	return b.String(), firstMeta
}

func readSourceContext(filePath string, targetLine int, contextLines int) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	start := max(0, targetLine-contextLines)
	end := min(len(lines), targetLine+contextLines+1)

	var b strings.Builder
	for i := start; i < end; i++ {
		marker := "  "
		if i == targetLine {
			marker = "> "
		}
		fmt.Fprintf(&b, "%s%4d | %s\n", marker, i+1, lines[i])
	}
	return b.String()
}

// readSourceLines returns raw source lines around targetLine without markers.
func readSourceLines(filePath string, targetLine int, contextLines int) string {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(data), "\n")
	start := max(0, targetLine-contextLines)
	end := min(len(lines), targetLine+contextLines+1)

	return strings.Join(lines[start:end], "\n")
}
