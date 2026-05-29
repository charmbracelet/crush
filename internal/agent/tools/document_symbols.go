package tools

import (
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/taigrr/fantasy"

	"github.com/taigrr/crush/internal/filepathext"
	"github.com/taigrr/crush/internal/lsp"
)

const DocumentSymbolsToolName = "lsp_document_symbols"

//go:embed document_symbols.md
var documentSymbolsDescription string

// DocumentSymbolsParams takes a file path; everything else is owned by
// the LSP server.
type DocumentSymbolsParams struct {
	FilePath string `json:"file_path" description:"The absolute path to the file to outline"`
}

// NewDocumentSymbolsTool returns the lsp_document_symbols tool, which
// renders an indented outline of the symbols defined in a single file.
func NewDocumentSymbolsTool(lspManager *lsp.Manager, workingDir WorkingDirFunc) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DocumentSymbolsToolName,
		documentSymbolsDescription,
		func(ctx context.Context, params DocumentSymbolsParams, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}
			if lspManager.Clients().Len() == 0 {
				return fantasy.NewTextErrorResponse("no LSP clients available"), nil
			}

			absPath, err := filepath.Abs(filepathext.SmartJoin(workingDir(), params.FilePath))
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid path: %s", err)), nil
			}

			var client *lsp.Client
			for c := range lspManager.Clients().Seq() {
				if c.HandlesFile(absPath) {
					client = c
					break
				}
			}
			if client == nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("no LSP client handles %s", absPath)), nil
			}

			results, err := client.DocumentSymbols(ctx, absPath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("document_symbols failed: %s", err)), nil
			}
			if len(results) == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("No symbols found in %s", absPath)), nil
			}

			return fantasy.NewTextResponse(formatDocumentSymbols(absPath, results)), nil
		},
	)
}

// formatDocumentSymbols renders the LSP result as an indented outline.
// Hierarchical (DocumentSymbol) results are walked recursively; flat
// (SymbolInformation) results render as a single level.
func formatDocumentSymbols(path string, results []protocol.DocumentSymbolResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Outline of %s (%d top-level symbol(s)):\n\n", path, len(results))
	for _, r := range results {
		switch v := r.(type) {
		case *protocol.DocumentSymbol:
			writeDocumentSymbol(&b, v, 0)
		case *protocol.SymbolInformation:
			fmt.Fprintf(&b, "%s %s (line %d)\n",
				symbolKindName(v.Kind), v.Name, v.Location.Range.Start.Line+1)
		default:
			fmt.Fprintf(&b, "%s (line %d)\n", r.GetName(), r.GetRange().Start.Line+1)
		}
	}
	return b.String()
}

func writeDocumentSymbol(b *strings.Builder, s *protocol.DocumentSymbol, depth int) {
	indent := strings.Repeat("  ", depth)
	detail := ""
	if s.Detail != "" {
		detail = "  " + s.Detail
	}
	fmt.Fprintf(b, "%s%s %s%s (line %d)\n",
		indent, symbolKindName(s.Kind), s.Name, detail, s.Range.Start.Line+1)
	for i := range s.Children {
		writeDocumentSymbol(b, &s.Children[i], depth+1)
	}
}

// symbolKindName maps the LSP enum to a short label. We accept
// unknown values (LSP could be extended) by returning the numeric kind
// instead of panicking.
func symbolKindName(k protocol.SymbolKind) string {
	switch k {
	case protocol.File:
		return "[file]"
	case protocol.Module:
		return "[module]"
	case protocol.Namespace:
		return "[namespace]"
	case protocol.Package:
		return "[package]"
	case protocol.Class:
		return "[class]"
	case protocol.Method:
		return "[method]"
	case protocol.Property:
		return "[property]"
	case protocol.Field:
		return "[field]"
	case protocol.Constructor:
		return "[ctor]"
	case protocol.Enum:
		return "[enum]"
	case protocol.Interface:
		return "[interface]"
	case protocol.Function:
		return "[func]"
	case protocol.Variable:
		return "[var]"
	case protocol.Constant:
		return "[const]"
	case protocol.Struct:
		return "[struct]"
	case protocol.EnumMember:
		return "[enum-member]"
	case protocol.TypeParameter:
		return "[type-param]"
	default:
		return fmt.Sprintf("[kind:%d]", k)
	}
}
