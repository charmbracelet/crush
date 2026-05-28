package tools

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type ReplaceSymbolParams struct {
	Symbol      string `json:"symbol" description:"The symbol name to replace (e.g., function name, method name, type name)"`
	FilePath    string `json:"file_path" description:"The path to the file containing the symbol"`
	Replacement string `json:"replacement" description:"The full replacement text for the symbol (including signature/declaration)"`
}

const ReplaceSymbolToolName = "lsp_replace_symbol"

//go:embed lsp_replace_symbol.md
var replaceSymbolDescription string

func NewReplaceSymbolTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	files history.Service,
	filetracker filetracker.Service,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ReplaceSymbolToolName,
		replaceSymbolDescription,
		func(ctx context.Context, params ReplaceSymbolParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Symbol == "" {
				return fantasy.NewTextErrorResponse("symbol is required"), nil
			}
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}
			if params.Replacement == "" {
				return fantasy.NewTextErrorResponse("replacement is required"), nil
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

			target := findSymbolByName(symbols, params.Symbol)
			if target == nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("symbol '%s' not found in %s", params.Symbol, params.FilePath)), nil
			}

			rng := target.GetRange()

			content, err := os.ReadFile(params.FilePath)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to read file: %w", err)
			}

			lines := strings.Split(string(content), "\n")
			startLine := int(rng.Start.Line)
			endLine := int(rng.End.Line)
			if startLine >= len(lines) || endLine >= len(lines) {
				return fantasy.NewTextErrorResponse("symbol range exceeds file length"), nil
			}

			oldText := strings.Join(lines[startLine:endLine+1], "\n")

			sessionID := GetSessionFromContext(ctx)
			if sessionID != "" && permissions != nil {
				granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
					SessionID:   sessionID,
					ToolName:    ReplaceSymbolToolName,
					Description: fmt.Sprintf("Replace symbol '%s' in %s", params.Symbol, params.FilePath),
				})
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("permission request failed: %w", err)
				}
				if !granted {
					return fantasy.NewTextErrorResponse("edit denied by user"), nil
				}
			}

			if files != nil && sessionID != "" {
				if _, err := files.CreateVersion(ctx, sessionID, params.FilePath, string(content)); err != nil {
					slog.Warn("Failed to create file version before replace", "path", params.FilePath, "error", err)
				}
			}

			newLines := make([]string, 0, len(lines))
			newLines = append(newLines, lines[:startLine]...)
			newLines = append(newLines, strings.Split(params.Replacement, "\n")...)
			newLines = append(newLines, lines[endLine+1:]...)

			newContent := strings.Join(newLines, "\n")
			if err := os.WriteFile(params.FilePath, []byte(newContent), 0o644); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
			}

			if filetracker != nil && sessionID != "" {
				filetracker.RecordRead(ctx, sessionID, params.FilePath)
			}

			notifyLSPs(ctx, lspManager, params.FilePath)

			var b strings.Builder
			fmt.Fprintf(&b, "Replaced symbol '%s' in %s (lines %d-%d)\n\n", params.Symbol, params.FilePath, startLine+1, endLine+1)
			fmt.Fprintf(&b, "Old (%d lines):\n%s\n\n", endLine-startLine+1, truncateText(oldText, 500))
			fmt.Fprintf(&b, "New (%d lines):\n%s\n", strings.Count(params.Replacement, "\n")+1, truncateText(params.Replacement, 500))

			text := b.String()
			text += "\n" + getDiagnostics(params.FilePath, lspManager)

			return fantasy.NewTextResponse(text), nil
		},
	)
}

// findSymbolByName searches for a symbol by name in the document symbol tree.
func findSymbolByName(symbols []protocol.DocumentSymbolResult, name string) protocol.DocumentSymbolResult {
	for _, sym := range symbols {
		if sym.GetName() == name {
			return sym
		}
		if ds, ok := sym.(*protocol.DocumentSymbol); ok && len(ds.Children) > 0 {
			children := make([]protocol.DocumentSymbolResult, len(ds.Children))
			for i := range ds.Children {
				children[i] = &ds.Children[i]
			}
			if found := findSymbolByName(children, name); found != nil {
				return found
			}
		}
	}
	return nil
}

// truncateText truncates a string to maxLen bytes, appending a note if truncated.
func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
