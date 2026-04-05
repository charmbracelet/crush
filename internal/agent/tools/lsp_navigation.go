package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type LSPDefinitionParams struct {
	FilePath  string `json:"file_path" description:"The file path containing the symbol to inspect"`
	Line      int    `json:"line" description:"The 1-based line number of the symbol position"`
	Character int    `json:"character" description:"The 1-based column number of the symbol position"`
}

type LSPHoverParams struct {
	FilePath  string `json:"file_path" description:"The file path containing the symbol to inspect"`
	Line      int    `json:"line" description:"The 1-based line number of the symbol position"`
	Character int    `json:"character" description:"The 1-based column number of the symbol position"`
}

type LSPDocumentSymbolsParams struct {
	FilePath string `json:"file_path" description:"The file path to inspect for document symbols"`
}

type LSPWorkspaceSymbolsParams struct {
	Query string `json:"query,omitempty" description:"Optional symbol name query to filter workspace symbols. Leave empty to list available symbols."`
}

type lspPositionParams struct {
	FilePath  string
	Line      int
	Character int
}

const (
	LSPDeclarationToolName      = "lsp_declaration"
	LSPDefinitionToolName       = "lsp_definition"
	LSPImplementationToolName   = "lsp_implementation"
	LSPTypeDefinitionToolName   = "lsp_type_definition"
	LSPHoverToolName            = "lsp_hover"
	LSPDocumentSymbolsToolName  = "lsp_document_symbols"
	LSPWorkspaceSymbolsToolName = "lsp_workspace_symbols"
)

//go:embed lsp_declaration.md
var lspDeclarationDescription []byte

//go:embed lsp_definition.md
var lspDefinitionDescription []byte

//go:embed lsp_implementation.md
var lspImplementationDescription []byte

//go:embed lsp_type_definition.md
var lspTypeDefinitionDescription []byte

//go:embed lsp_hover.md
var lspHoverDescription []byte

//go:embed lsp_document_symbols.md
var lspDocumentSymbolsDescription []byte

//go:embed lsp_workspace_symbols.md
var lspWorkspaceSymbolsDescription []byte

func NewLSPDeclarationTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPDeclarationToolName,
		string(lspDeclarationDescription),
		func(ctx context.Context, params LSPDefinitionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			client, absPath, response, ok := lspClientForPosition(ctx, lspManager, lspPositionParams(params))
			if !ok {
				return response, nil
			}

			locations, err := client.FindDeclaration(ctx, absPath, params.Line, params.Character)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			locations = cleanupLocations(locations)
			if len(locations) == 0 {
				return fantasy.NewTextResponse("No declaration found."), nil
			}
			return fantasy.NewTextResponse(formatLocations("declaration", locations)), nil
		})
}

func NewLSPDefinitionTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPDefinitionToolName,
		string(lspDefinitionDescription),
		func(ctx context.Context, params LSPDefinitionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			client, absPath, response, ok := lspClientForPosition(ctx, lspManager, lspPositionParams(params))
			if !ok {
				return response, nil
			}

			locations, err := client.FindDefinition(ctx, absPath, params.Line, params.Character)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			locations = cleanupLocations(locations)
			if len(locations) == 0 {
				return fantasy.NewTextResponse("No definition found."), nil
			}
			return fantasy.NewTextResponse(formatLocations("definition", locations)), nil
		})
}

func NewLSPImplementationTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPImplementationToolName,
		string(lspImplementationDescription),
		func(ctx context.Context, params LSPDefinitionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			client, absPath, response, ok := lspClientForPosition(ctx, lspManager, lspPositionParams(params))
			if !ok {
				return response, nil
			}

			locations, err := client.FindImplementation(ctx, absPath, params.Line, params.Character)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			locations = cleanupLocations(locations)
			if len(locations) == 0 {
				return fantasy.NewTextResponse("No implementation found."), nil
			}
			return fantasy.NewTextResponse(formatLocations("implementation", locations)), nil
		})
}

func NewLSPTypeDefinitionTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPTypeDefinitionToolName,
		string(lspTypeDefinitionDescription),
		func(ctx context.Context, params LSPDefinitionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			client, absPath, response, ok := lspClientForPosition(ctx, lspManager, lspPositionParams(params))
			if !ok {
				return response, nil
			}

			locations, err := client.FindTypeDefinition(ctx, absPath, params.Line, params.Character)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			locations = cleanupLocations(locations)
			if len(locations) == 0 {
				return fantasy.NewTextResponse("No type definition found."), nil
			}
			return fantasy.NewTextResponse(formatLocations("type definition", locations)), nil
		})
}

func NewLSPHoverTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPHoverToolName,
		string(lspHoverDescription),
		func(ctx context.Context, params LSPHoverParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			client, absPath, response, ok := lspClientForPosition(ctx, lspManager, lspPositionParams(params))
			if !ok {
				return response, nil
			}

			hover, err := client.Hover(ctx, absPath, params.Line, params.Character)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			text := strings.TrimSpace(formatHover(hover))
			if text == "" {
				return fantasy.NewTextResponse("No hover information found."), nil
			}
			return fantasy.NewTextResponse(text), nil
		})
}

func NewLSPDocumentSymbolsTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPDocumentSymbolsToolName,
		string(lspDocumentSymbolsDescription),
		func(ctx context.Context, params LSPDocumentSymbolsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.FilePath) == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}
			if lspManager == nil || lspManager.Clients().Len() == 0 {
				return fantasy.NewTextErrorResponse("no LSP clients available"), nil
			}

			absPath, err := filepath.Abs(filepath.FromSlash(params.FilePath))
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to get absolute path: %s", err)), nil
			}
			openInLSPs(ctx, lspManager, absPath)
			client := firstHandlingClient(lspManager, absPath)
			if client == nil {
				return fantasy.NewTextResponse(fmt.Sprintf("No LSP client handles %s", absPath)), nil
			}

			symbols, err := client.DocumentSymbols(ctx, absPath)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if len(symbols) == 0 {
				return fantasy.NewTextResponse("No document symbols found."), nil
			}
			return fantasy.NewTextResponse(formatDocumentSymbols(absPath, documentSymbolEntries(symbols))), nil
		})
}

func NewLSPWorkspaceSymbolsTool(lspManager *lsp.Manager) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPWorkspaceSymbolsToolName,
		string(lspWorkspaceSymbolsDescription),
		func(ctx context.Context, params LSPWorkspaceSymbolsParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if lspManager == nil || lspManager.Clients().Len() == 0 {
				return fantasy.NewTextErrorResponse("no LSP clients available"), nil
			}

			var all []workspaceSymbolEntry
			for name, client := range lspManager.Clients().Seq2() {
				entries, err := client.WorkspaceSymbols(ctx, params.Query)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("workspace symbols failed for %s: %s", name, err)), nil
				}
				all = append(all, workspaceSymbolEntries(entries)...)
			}
			if len(all) == 0 {
				return fantasy.NewTextResponse("No workspace symbols found."), nil
			}
			return fantasy.NewTextResponse(formatWorkspaceSymbols(all)), nil
		})
}

func lspClientForPosition(ctx context.Context, lspManager *lsp.Manager, params lspPositionParams) (*lsp.Client, string, fantasy.ToolResponse, bool) {
	if strings.TrimSpace(params.FilePath) == "" {
		return nil, "", fantasy.NewTextErrorResponse("file_path is required"), false
	}
	if params.Line <= 0 {
		return nil, "", fantasy.NewTextErrorResponse("line must be >= 1"), false
	}
	if params.Character <= 0 {
		return nil, "", fantasy.NewTextErrorResponse("character must be >= 1"), false
	}
	if lspManager == nil || lspManager.Clients().Len() == 0 {
		return nil, "", fantasy.NewTextErrorResponse("no LSP clients available"), false
	}

	absPath, err := filepath.Abs(filepath.FromSlash(params.FilePath))
	if err != nil {
		return nil, "", fantasy.NewTextErrorResponse(fmt.Sprintf("failed to get absolute path: %s", err)), false
	}
	openInLSPs(ctx, lspManager, absPath)
	client := firstHandlingClient(lspManager, absPath)
	if client == nil {
		return nil, absPath, fantasy.NewTextResponse(fmt.Sprintf("No LSP client handles %s", absPath)), false
	}
	return client, absPath, fantasy.ToolResponse{}, true
}

func firstHandlingClient(lspManager *lsp.Manager, absPath string) *lsp.Client {
	for client := range lspManager.Clients().Seq() {
		if client.HandlesFile(absPath) {
			return client
		}
	}
	return nil
}

func formatLocations(kind string, locations []protocol.Location) string {
	var output strings.Builder
	fmt.Fprintf(&output, "Found %d %s location(s):\n\n", len(locations), kind)
	for _, loc := range locations {
		path, err := loc.URI.Path()
		if err != nil {
			continue
		}
		fmt.Fprintf(&output, "%s:%d:%d\n", path, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
	}
	return strings.TrimSpace(output.String())
}

func formatHover(hover *protocol.Hover) string {
	if hover == nil {
		return ""
	}
	if strings.TrimSpace(hover.Contents.Value) != "" {
		return strings.TrimSpace(hover.Contents.Value)
	}
	if raw, err := json.Marshal(hover.Contents); err == nil {
		return strings.TrimSpace(string(raw))
	}
	return ""
}

type documentSymbolEntry struct {
	Name     string
	Kind     string
	Line     uint32
	Column   uint32
	Children []documentSymbolEntry
}

func formatDocumentSymbols(filePath string, symbols []documentSymbolEntry) string {
	var output strings.Builder
	fmt.Fprintf(&output, "%s\n", filePath)
	for _, symbol := range symbols {
		writeDocumentSymbol(&output, symbol, 0)
	}
	return strings.TrimSpace(output.String())
}

func writeDocumentSymbol(output *strings.Builder, symbol documentSymbolEntry, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(output, "%s- %s (%s) %d:%d\n", indent, symbol.Name, symbol.Kind, symbol.Line, symbol.Column)
	for _, child := range symbol.Children {
		writeDocumentSymbol(output, child, depth+1)
	}
}

type workspaceSymbolEntry struct {
	Name   string
	Kind   string
	Path   string
	Line   uint32
	Column uint32
}

func formatWorkspaceSymbols(symbols []workspaceSymbolEntry) string {
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Path != symbols[j].Path {
			return symbols[i].Path < symbols[j].Path
		}
		if symbols[i].Line != symbols[j].Line {
			return symbols[i].Line < symbols[j].Line
		}
		return symbols[i].Name < symbols[j].Name
	})

	var output strings.Builder
	fmt.Fprintf(&output, "Found %d workspace symbol(s):\n\n", len(symbols))
	for _, symbol := range symbols {
		fmt.Fprintf(&output, "%s (%s) %s:%d:%d\n", symbol.Name, symbol.Kind, symbol.Path, symbol.Line, symbol.Column)
	}
	return strings.TrimSpace(output.String())
}

func symbolKindString(kind protocol.SymbolKind) string {
	if kind == 0 {
		return "unknown"
	}
	return strings.ToLower(fmt.Sprintf("%v", kind))
}

func documentSymbolEntries(results []protocol.DocumentSymbolResult) []documentSymbolEntry {
	entries := make([]documentSymbolEntry, 0, len(results))
	for _, result := range results {
		entry := documentSymbolEntry{
			Name:   result.GetName(),
			Kind:   documentSymbolKind(result),
			Line:   result.GetRange().Start.Line + 1,
			Column: result.GetRange().Start.Character + 1,
		}
		if symbol, ok := result.(*protocol.DocumentSymbol); ok {
			entry.Children = documentSymbolEntries(documentSymbolResultSlice(symbol.Children))
		}
		entries = append(entries, entry)
	}
	return entries
}

func documentSymbolKind(result protocol.DocumentSymbolResult) string {
	switch symbol := result.(type) {
	case *protocol.DocumentSymbol:
		return symbolKindString(symbol.Kind)
	case *protocol.SymbolInformation:
		return symbolKindString(symbol.Kind)
	default:
		return "unknown"
	}
}

func documentSymbolResultSlice(symbols []protocol.DocumentSymbol) []protocol.DocumentSymbolResult {
	results := make([]protocol.DocumentSymbolResult, 0, len(symbols))
	for i := range symbols {
		results = append(results, &symbols[i])
	}
	return results
}

func workspaceSymbolEntries(results []protocol.WorkspaceSymbolResult) []workspaceSymbolEntry {
	entries := make([]workspaceSymbolEntry, 0, len(results))
	for _, result := range results {
		loc := result.GetLocation()
		path, err := loc.URI.Path()
		if err != nil {
			continue
		}
		entries = append(entries, workspaceSymbolEntry{
			Name:   result.GetName(),
			Kind:   workspaceSymbolKind(result),
			Path:   path,
			Line:   loc.Range.Start.Line + 1,
			Column: loc.Range.Start.Character + 1,
		})
	}
	return entries
}

func workspaceSymbolKind(result protocol.WorkspaceSymbolResult) string {
	switch symbol := result.(type) {
	case *protocol.WorkspaceSymbol:
		return symbolKindString(symbol.Kind)
	case *protocol.SymbolInformation:
		return symbolKindString(symbol.Kind)
	default:
		return "unknown"
	}
}

func combineLocationResults(primary any, secondary []protocol.Location) []protocol.Location {
	locations := locationResults(primary)
	locations = append(locations, secondary...)
	return cleanupLocations(locations)
}

func locationResults(value any) []protocol.Location {
	switch v := value.(type) {
	case protocol.Location:
		return []protocol.Location{v}
	case []protocol.Location:
		return append([]protocol.Location(nil), v...)
	case protocol.Or_Definition:
		return locationResults(v.Value)
	case protocol.Or_Declaration:
		return locationResults(v.Value)
	default:
		return nil
	}
}

func effectiveWorkspaceQuery(query string) string {
	query = strings.TrimSpace(query)
	return query
}
