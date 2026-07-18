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
	Symbol      string `json:"symbol" description:"The symbol name to target (e.g., function name, method name, type name)"`
	FilePath    string `json:"file_path" description:"The path to the file containing the symbol"`
	Replacement string `json:"replacement,omitempty" description:"The replacement text. Required for 'replace' action. For 'add_before'/'add_after', the text to insert. Ignored for 'delete'."`
	Action      string `json:"action,omitempty" description:"Operation to perform: 'replace' (default, replace entire symbol), 'add_before' (insert before symbol), 'add_after' (insert after symbol), 'delete' (remove symbol entirely)"`
}

const ReplaceSymbolToolName = "lsp_replace_symbol"

//go:embed lsp_replace_symbol.md
var replaceSymbolDescription string

// ReplaceSymbolResponseMetadata carries diff data for the renderer.
type ReplaceSymbolResponseMetadata struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
	Action     string `json:"action"`
}

// ReplaceSymbolPermissionsParams carries diff data for the permission dialog.
type ReplaceSymbolPermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

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

			action := params.Action
			if action == "" {
				action = "replace"
			}
			switch action {
			case "replace", "add_before", "add_after", "delete":
			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid action %q: must be replace, add_before, add_after, or delete", action)), nil
			}
			if (action == "replace" || action == "add_before" || action == "add_after") && params.Replacement == "" {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("replacement is required for action %q", action)), nil
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

			// Compute new content before permission so the dialog can show a diff.
			var newLines []string
			switch action {
			case "replace":
				newLines = make([]string, 0, len(lines))
				newLines = append(newLines, lines[:startLine]...)
				newLines = append(newLines, strings.Split(params.Replacement, "\n")...)
				newLines = append(newLines, lines[endLine+1:]...)
			case "add_before":
				newLines = make([]string, 0, len(lines)+strings.Count(params.Replacement, "\n")+1)
				newLines = append(newLines, lines[:startLine]...)
				newLines = append(newLines, strings.Split(params.Replacement, "\n")...)
				newLines = append(newLines, lines[startLine:]...)
			case "add_after":
				newLines = make([]string, 0, len(lines)+strings.Count(params.Replacement, "\n")+1)
				newLines = append(newLines, lines[:endLine+1]...)
				newLines = append(newLines, strings.Split(params.Replacement, "\n")...)
				newLines = append(newLines, lines[endLine+1:]...)
			case "delete":
				newLines = make([]string, 0, len(lines))
				newLines = append(newLines, lines[:startLine]...)
				newLines = append(newLines, lines[endLine+1:]...)
			}

			newContent := strings.Join(newLines, "\n")

			sessionID := GetSessionFromContext(ctx)
			if sessionID != "" && permissions != nil {
				granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        params.FilePath,
					ToolName:    ReplaceSymbolToolName,
					Description: fmt.Sprintf("%s symbol '%s' in %s", action, params.Symbol, params.FilePath),
					Params: ReplaceSymbolPermissionsParams{
						FilePath:   params.FilePath,
						OldContent: string(content),
						NewContent: newContent,
					},
				})
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("permission request failed: %w", err)
				}
				if !granted {
					return NewPermissionDeniedResponse(), nil
				}
			}

			if files != nil && sessionID != "" {
				if _, err := files.CreateVersion(ctx, sessionID, params.FilePath, string(content)); err != nil {
					slog.Warn("Failed to create file version before replace", "path", params.FilePath, "error", err)
				}
			}

			if err := os.WriteFile(params.FilePath, []byte(newContent), 0o644); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
			}

			if filetracker != nil && sessionID != "" {
				filetracker.RecordRead(ctx, sessionID, params.FilePath)
			}

			notifyLSPs(ctx, lspManager, params.FilePath)

			var summary string
			switch action {
			case "replace":
				summary = fmt.Sprintf("Replaced symbol '%s' in %s (lines %d-%d)", params.Symbol, params.FilePath, startLine+1, endLine+1)
			case "add_before":
				summary = fmt.Sprintf("Inserted before symbol '%s' in %s (before line %d)", params.Symbol, params.FilePath, startLine+1)
			case "add_after":
				summary = fmt.Sprintf("Inserted after symbol '%s' in %s (after line %d)", params.Symbol, params.FilePath, endLine+1)
			case "delete":
				summary = fmt.Sprintf("Deleted symbol '%s' from %s (lines %d-%d)", params.Symbol, params.FilePath, startLine+1, endLine+1)
			}

			resp := fantasy.NewTextResponse(summary + "\n" + getDiagnostics(params.FilePath, lspManager))
			resp = fantasy.WithResponseMetadata(resp, ReplaceSymbolResponseMetadata{
				FilePath:   params.FilePath,
				OldContent: string(content),
				NewContent: newContent,
				Action:     action,
			})
			return resp, nil
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
