package tools

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type SymbolsParams struct {
	FilePath string `json:"file_path" description:"The path to the file to get symbols for"`
}

const SymbolsToolName = "lsp_symbols"

//go:embed lsp_symbols.md
var symbolsDescription string

func NewSymbolsTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		SymbolsToolName,
		symbolsDescription,
		func(ctx context.Context, params SymbolsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}
			lspManager.Start(ctx, params.FilePath)

			client := findLSPClient(lspManager, params.FilePath)
			if client == nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("no LSP client handles file: %s", params.FilePath)), nil
			}

			symbols, err := client.DocumentSymbols(ctx, params.FilePath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to get document symbols: %s", err)), nil
			}
			if len(symbols) == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("No symbols found in %s", params.FilePath)), nil
			}

			return fantasy.NewTextResponse(formatSymbols(symbols, 0)), nil
		},
	)
}

func formatSymbols(symbols []protocol.DocumentSymbolResult, indent int) string {
	var b strings.Builder
	prefix := strings.Repeat("  ", indent)
	for _, sym := range symbols {
		rng := sym.GetRange()
		line := rng.Start.Line + 1
		kind := symbolKindString(sym)
		fmt.Fprintf(&b, "%s%s %s (line %d)\n", prefix, kind, sym.GetName(), line)
		if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
			b.WriteString(formatDocumentSymbolChildren(ds.Children, indent+1))
		}
	}
	return b.String()
}

func formatDocumentSymbolChildren(children []protocol.DocumentSymbol, indent int) string {
	var b strings.Builder
	prefix := strings.Repeat("  ", indent)
	for i := range children {
		c := &children[i]
		kind := symbolKindString(c)
		fmt.Fprintf(&b, "%s%s %s (line %d)\n", prefix, kind, c.Name, c.Range.Start.Line+1)
		if len(c.Children) > 0 {
			b.WriteString(formatDocumentSymbolChildren(c.Children, indent+1))
		}
	}
	return b.String()
}

func symbolKindString(sym protocol.DocumentSymbolResult) string {
	var kind protocol.SymbolKind
	switch s := sym.(type) {
	case *protocol.DocumentSymbol:
		kind = s.Kind
	case *protocol.SymbolInformation:
		kind = s.Kind
	default:
		return "Symbol"
	}
	if name, ok := symbolKindNames[kind]; ok {
		return name
	}
	return "Symbol"
}

var symbolKindNames = map[protocol.SymbolKind]string{
	protocol.File:          "File",
	protocol.Module:        "Module",
	protocol.Namespace:     "Namespace",
	protocol.Package:       "Package",
	protocol.Class:         "Class",
	protocol.Method:        "Method",
	protocol.Property:      "Property",
	protocol.Field:         "Field",
	protocol.Constructor:   "Constructor",
	protocol.Enum:          "Enum",
	protocol.Interface:     "Interface",
	protocol.Function:      "Function",
	protocol.Variable:      "Variable",
	protocol.Constant:      "Constant",
	protocol.String:        "String",
	protocol.Number:        "Number",
	protocol.Boolean:       "Boolean",
	protocol.Array:         "Array",
	protocol.Object:        "Object",
	protocol.Key:           "Key",
	protocol.Null:          "Null",
	protocol.EnumMember:    "EnumMember",
	protocol.Struct:        "Struct",
	protocol.Event:         "Event",
	protocol.Operator:      "Operator",
	protocol.TypeParameter: "TypeParameter",
}
