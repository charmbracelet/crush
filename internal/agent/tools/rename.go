package tools

import (
	"cmp"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/taigrr/fantasy"

	"github.com/taigrr/crush/internal/editor"
	"github.com/taigrr/crush/internal/fsext"
	"github.com/taigrr/crush/internal/lsp"
	lsputil "github.com/taigrr/crush/internal/lsp/util"
	"github.com/taigrr/crush/internal/permission"
)

const RenameToolName = "lsp_rename"

//go:embed rename.md
var renameDescription string

// RenameParams takes the same symbol-locating shape as references and
// definition, plus the new name. Same one-symbol-per-call contract.
type RenameParams struct {
	Symbol  string `json:"symbol" description:"The symbol name to rename (must match an existing identifier)"`
	NewName string `json:"new_name" description:"The new name for the symbol"`
	Path    string `json:"path,omitempty" description:"Directory or file to seed the rename from. Defaults to the current working directory."`
}

// RenamePermissionsParams is the payload shown to the user when they're
// asked to authorize a rename.
type RenamePermissionsParams struct {
	Symbol  string   `json:"symbol"`
	NewName string   `json:"new_name"`
	Files   []string `json:"files"`
}

// NewRenameTool returns the lsp_rename tool. The bridge is optional and
// used to flash highlights and silence the W11 prompt on every modified
// file once the rename is applied.
func NewRenameTool(lspManager *lsp.Manager, permissions permission.Service, workingDir WorkingDirFunc, bridge editor.Bridge) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		RenameToolName,
		renameDescription,
		func(ctx context.Context, params RenameParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Symbol == "" {
				return fantasy.NewTextErrorResponse("symbol is required"), nil
			}
			if params.NewName == "" {
				return fantasy.NewTextErrorResponse("new_name is required"), nil
			}
			if params.Symbol == params.NewName {
				return fantasy.NewTextErrorResponse("new_name must differ from symbol"), nil
			}
			if lspManager.Clients().Len() == 0 {
				return fantasy.NewTextErrorResponse("no LSP clients available"), nil
			}

			searchDir := cmp.Or(params.Path, ".")

			matches, _, err := searchFiles(ctx, regexp.QuoteMeta(params.Symbol), searchDir, "", 50)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to locate symbol: %s", err)), nil
			}
			if len(matches) == 0 {
				return fantasy.NewTextResponse(fmt.Sprintf("Symbol '%s' not found", params.Symbol)), nil
			}

			// Try each match in turn until an LSP server gives us back a
			// non-empty WorkspaceEdit. Some hits may be in comments or
			// strings; skip those (LSP returns an "no identifier" error).
			var (
				workspaceEdit *protocol.WorkspaceEdit
				owningClient  *lsp.Client
				allErrs       error
			)
			for _, match := range matches {
				edit, client, err := requestRename(ctx, lspManager, params, match)
				if err != nil {
					if strings.Contains(err.Error(), "no identifier found") {
						continue
					}
					slog.Error("Rename request failed", "error", err, "path", match.path, "line", match.lineNum)
					allErrs = errors.Join(allErrs, err)
					continue
				}
				if edit != nil && hasChanges(edit) {
					workspaceEdit = edit
					owningClient = client
					break
				}
			}

			if workspaceEdit == nil {
				if allErrs != nil {
					return fantasy.NewTextErrorResponse(allErrs.Error()), nil
				}
				return fantasy.NewTextResponse(fmt.Sprintf("LSP returned no edits for '%s' -> '%s'", params.Symbol, params.NewName)), nil
			}

			// Compute affected paths up front so we can show them in the
			// permission prompt and emit per-file notifications later.
			files := affectedPaths(workspaceEdit)

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session_id is required for rename")
			}

			ok, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        fsext.PathOrPrefix(filepath.Dir(files[0]), workingDir()),
				ToolCallID:  call.ID,
				ToolName:    RenameToolName,
				Action:      "rename",
				Description: fmt.Sprintf("Rename '%s' to '%s' across %d file(s)", params.Symbol, params.NewName, len(files)),
				Params: RenamePermissionsParams{
					Symbol:  params.Symbol,
					NewName: params.NewName,
					Files:   files,
				},
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !ok {
				return NewPermissionDeniedResponse(), nil
			}

			if err := lsputil.ApplyWorkspaceEdit(*workspaceEdit, owningClient.OffsetEncoding()); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to apply rename: %s", err)), nil
			}

			// Best-effort editor sync: flash + checktime per touched file
			// so the user's nvim picks the changes up silently.
			if bridge != nil && bridge.Available() {
				for _, p := range files {
					if err := bridge.NotifyFileChanged(ctx, p); err != nil {
						slog.Debug("Editor notify failed", "path", p, "error", err)
					}
				}
			}

			return fantasy.NewTextResponse(formatRenameResult(params.Symbol, params.NewName, files)), nil
		},
	)
}

// requestRename runs through the same client-resolution logic as
// references/definition.
func requestRename(ctx context.Context, lspManager *lsp.Manager, params RenameParams, match grepMatch) (*protocol.WorkspaceEdit, *lsp.Client, error) {
	absPath, err := filepath.Abs(match.path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path: %s", err)
	}

	var client *lsp.Client
	for c := range lspManager.Clients().Seq() {
		if c.HandlesFile(absPath) {
			client = c
			break
		}
	}
	if client == nil {
		return nil, nil, nil
	}

	edit, err := client.Rename(ctx, absPath, match.lineNum, match.charNum+getSymbolOffset(params.Symbol), params.NewName)
	return edit, client, err
}

// hasChanges returns true if the WorkspaceEdit actually proposes any
// edits across either Changes or DocumentChanges.
func hasChanges(edit *protocol.WorkspaceEdit) bool {
	if edit == nil {
		return false
	}
	if len(edit.Changes) > 0 {
		return true
	}
	return len(edit.DocumentChanges) > 0
}

// affectedPaths extracts every workspace path referenced by the edit, in
// sorted order, for the permission prompt and the editor notify pass.
func affectedPaths(edit *protocol.WorkspaceEdit) []string {
	seen := map[string]struct{}{}
	add := func(uri protocol.DocumentURI) {
		p, err := uri.Path()
		if err != nil {
			return
		}
		seen[p] = struct{}{}
	}
	for uri := range edit.Changes {
		add(uri)
	}
	for _, dc := range edit.DocumentChanges {
		if dc.TextDocumentEdit != nil {
			add(dc.TextDocumentEdit.TextDocument.URI)
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

func formatRenameResult(symbol, newName string, files []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Renamed '%s' -> '%s' in %d file(s):\n", symbol, newName, len(files))
	for _, p := range files {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	return b.String()
}
