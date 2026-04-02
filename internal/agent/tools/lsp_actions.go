package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

type LSPCodeActionParams struct {
	FilePath    string `json:"file_path" description:"The file path containing the symbol or range to inspect for code actions"`
	Line        int    `json:"line" description:"The 1-based line number of the symbol position"`
	Character   int    `json:"character" description:"The 1-based column number of the symbol position"`
	ActionKind  string `json:"action_kind,omitempty" description:"Optional code action kind filter (for example quickfix, refactor.extract)"`
	Apply       bool   `json:"apply,omitempty" description:"If true, apply a selected code action edit to the workspace"`
	ActionIndex int    `json:"action_index,omitempty" description:"1-based index of the action to apply when apply=true (defaults to 1)"`
}

type LSPRenameParams struct {
	FilePath  string `json:"file_path" description:"The file path containing the symbol to rename"`
	Line      int    `json:"line" description:"The 1-based line number of the symbol position"`
	Character int    `json:"character" description:"The 1-based column number of the symbol position"`
	NewName   string `json:"new_name" description:"The new symbol name to apply"`
}

type LSPFormatParams struct {
	FilePath               string `json:"file_path" description:"The file path to format"`
	TabSize                int    `json:"tab_size,omitempty" description:"Tab width in spaces (default 4)"`
	InsertSpaces           *bool  `json:"insert_spaces,omitempty" description:"Prefer spaces over tabs (default true)"`
	TrimTrailingWhitespace *bool  `json:"trim_trailing_whitespace,omitempty" description:"Trim trailing whitespace on each line"`
	InsertFinalNewline     *bool  `json:"insert_final_newline,omitempty" description:"Ensure file ends with a final newline"`
	TrimFinalNewlines      *bool  `json:"trim_final_newlines,omitempty" description:"Trim extra trailing newlines at end of file"`
}

const (
	LSPCodeActionToolName = "lsp_code_action"
	LSPRenameToolName     = "lsp_rename"
	LSPFormatToolName     = "lsp_format"
)

//go:embed lsp_code_action.md
var lspCodeActionDescription []byte

//go:embed lsp_rename.md
var lspRenameDescription []byte

//go:embed lsp_format.md
var lspFormatDescription []byte

func NewLSPCodeActionTool(lspManager *lsp.Manager, permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPCodeActionToolName,
		string(lspCodeActionDescription),
		func(ctx context.Context, params LSPCodeActionParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			client, absPath, response, ok := lspClientForPosition(ctx, lspManager, lspPositionParams{
				FilePath:  params.FilePath,
				Line:      params.Line,
				Character: params.Character,
			})
			if !ok {
				return response, nil
			}

			only := make([]protocol.CodeActionKind, 0, 1)
			if strings.TrimSpace(params.ActionKind) != "" {
				only = append(only, protocol.CodeActionKind(strings.TrimSpace(params.ActionKind)))
			}

			actions, err := client.CodeActions(ctx, absPath, params.Line, params.Character, only)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if len(actions) == 0 {
				return fantasy.NewTextResponse("No code actions found."), nil
			}

			if !params.Apply {
				return fantasy.NewTextResponse(formatCodeActions(actions)), nil
			}

			selectedIndex := params.ActionIndex
			if selectedIndex <= 0 {
				selectedIndex = 1
			}
			if selectedIndex > len(actions) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("action_index %d is out of range (1-%d)", selectedIndex, len(actions))), nil
			}

			selected := actions[selectedIndex-1]
			if selected.Edit == nil {
				return fantasy.NewTextErrorResponse("selected code action does not provide a workspace edit"), nil
			}

			permissionResponse, err := requestLSPWritePermission(ctx, permissions, workingDir, call, absPath, LSPCodeActionToolName, fmt.Sprintf("Apply LSP code action in %s", absPath), params)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if permissionResponse != nil {
				return *permissionResponse, nil
			}

			if err := client.ApplyWorkspaceEdit(*selected.Edit); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to apply code action workspace edit: %s", err)), nil
			}
			notifyWorkspaceEditPaths(ctx, lspManager, *selected.Edit)
			return fantasy.NewTextResponse(fmt.Sprintf("Applied code action #%d: %s", selectedIndex, strings.TrimSpace(selected.Title))), nil
		},
	)
}

func NewLSPRenameTool(lspManager *lsp.Manager, permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPRenameToolName,
		string(lspRenameDescription),
		func(ctx context.Context, params LSPRenameParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.NewName) == "" {
				return fantasy.NewTextErrorResponse("new_name is required"), nil
			}

			client, absPath, response, ok := lspClientForPosition(ctx, lspManager, lspPositionParams{
				FilePath:  params.FilePath,
				Line:      params.Line,
				Character: params.Character,
			})
			if !ok {
				return response, nil
			}

			workspaceEdit, err := client.Rename(ctx, absPath, params.Line, params.Character, strings.TrimSpace(params.NewName))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if workspaceEdit == nil || workspaceEditEmpty(*workspaceEdit) {
				return fantasy.NewTextResponse("Rename completed with no edits."), nil
			}

			permissionResponse, err := requestLSPWritePermission(ctx, permissions, workingDir, call, absPath, LSPRenameToolName, fmt.Sprintf("Rename symbol in %s", absPath), params)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if permissionResponse != nil {
				return *permissionResponse, nil
			}

			if err := client.ApplyWorkspaceEdit(*workspaceEdit); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to apply rename workspace edit: %s", err)), nil
			}
			notifyWorkspaceEditPaths(ctx, lspManager, *workspaceEdit)
			return fantasy.NewTextResponse(fmt.Sprintf("Renamed symbol to %s.", strings.TrimSpace(params.NewName))), nil
		},
	)
}

func NewLSPFormatTool(lspManager *lsp.Manager, permissions permission.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		LSPFormatToolName,
		string(lspFormatDescription),
		func(ctx context.Context, params LSPFormatParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			client, absPath, response, ok := lspClientForFile(ctx, lspManager, params.FilePath)
			if !ok {
				return response, nil
			}

			edits, err := client.FormatDocument(ctx, absPath, formattingOptions(params))
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if len(edits) == 0 {
				return fantasy.NewTextResponse("No formatting changes returned."), nil
			}

			workspaceEdit := protocol.WorkspaceEdit{
				Changes: map[protocol.DocumentURI][]protocol.TextEdit{
					protocol.URIFromPath(absPath): edits,
				},
			}

			permissionResponse, err := requestLSPWritePermission(ctx, permissions, workingDir, call, absPath, LSPFormatToolName, fmt.Sprintf("Format %s with LSP", absPath), params)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if permissionResponse != nil {
				return *permissionResponse, nil
			}

			if err := client.ApplyWorkspaceEdit(workspaceEdit); err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to apply formatting edits: %s", err)), nil
			}
			notifyWorkspaceEditPaths(ctx, lspManager, workspaceEdit)
			return fantasy.NewTextResponse(fmt.Sprintf("Applied %d formatting edit(s).", len(edits))), nil
		},
	)
}

func lspClientForFile(ctx context.Context, lspManager *lsp.Manager, filePath string) (*lsp.Client, string, fantasy.ToolResponse, bool) {
	if strings.TrimSpace(filePath) == "" {
		return nil, "", fantasy.NewTextErrorResponse("file_path is required"), false
	}
	if lspManager == nil || lspManager.Clients().Len() == 0 {
		return nil, "", fantasy.NewTextErrorResponse("no LSP clients available"), false
	}

	absPath, err := filepath.Abs(filepath.FromSlash(filePath))
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

func requestLSPWritePermission(ctx context.Context, permissions permission.Service, workingDir string, call fantasy.ToolCall, filePath, toolName, description string, params any) (*fantasy.ToolResponse, error) {
	if permissions == nil {
		return nil, nil
	}
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	effectiveWorkingDir := cmp.Or(GetWorkingDirFromContext(ctx), workingDir)
	permissionResponse, err := RequestPermission(ctx, permissions,
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        fsext.PathOrPrefix(filePath, effectiveWorkingDir),
			ToolCallID:  call.ID,
			ToolName:    toolName,
			Action:      "write",
			Description: description,
			Params:      params,
		},
	)
	if err != nil {
		return nil, err
	}
	return permissionResponse, nil
}

func formattingOptions(params LSPFormatParams) protocol.FormattingOptions {
	tabSize := params.TabSize
	if tabSize <= 0 {
		tabSize = 4
	}
	insertSpaces := true
	if params.InsertSpaces != nil {
		insertSpaces = *params.InsertSpaces
	}

	options := protocol.FormattingOptions{
		TabSize:      uint32(tabSize),
		InsertSpaces: insertSpaces,
	}
	if params.TrimTrailingWhitespace != nil {
		options.TrimTrailingWhitespace = *params.TrimTrailingWhitespace
	}
	if params.InsertFinalNewline != nil {
		options.InsertFinalNewline = *params.InsertFinalNewline
	}
	if params.TrimFinalNewlines != nil {
		options.TrimFinalNewlines = *params.TrimFinalNewlines
	}
	return options
}

func formatCodeActions(actions []protocol.CodeAction) string {
	var output strings.Builder
	fmt.Fprintf(&output, "Found %d code action(s):\n\n", len(actions))
	for index, action := range actions {
		parts := make([]string, 0, 4)
		parts = append(parts, fmt.Sprintf("%d.", index+1), strings.TrimSpace(action.Title))
		if strings.TrimSpace(string(action.Kind)) != "" {
			parts = append(parts, fmt.Sprintf("[%s]", action.Kind))
		}
		if action.Edit != nil {
			parts = append(parts, "(edit)")
		}
		if action.Command != nil {
			parts = append(parts, "(command)")
		}
		if action.Disabled != nil && strings.TrimSpace(action.Disabled.Reason) != "" {
			parts = append(parts, fmt.Sprintf("(disabled: %s)", strings.TrimSpace(action.Disabled.Reason)))
		}
		output.WriteString(strings.Join(parts, " "))
		output.WriteByte('\n')
	}
	return strings.TrimSpace(output.String())
}

func workspaceEditEmpty(edit protocol.WorkspaceEdit) bool {
	return len(edit.Changes) == 0 && len(edit.DocumentChanges) == 0
}

func notifyWorkspaceEditPaths(ctx context.Context, lspManager *lsp.Manager, edit protocol.WorkspaceEdit) {
	paths := workspaceEditPaths(edit)
	for _, path := range paths {
		notifyLSPs(ctx, lspManager, path)
	}
}

func workspaceEditPaths(edit protocol.WorkspaceEdit) []string {
	seen := make(map[string]struct{})
	paths := make([]string, 0, len(edit.Changes)+len(edit.DocumentChanges))
	appendPath := func(path string) {
		path = normalizeWorkspaceEditPath(path)
		if path == "" {
			return
		}
		if _, exists := seen[path]; exists {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	for uri := range edit.Changes {
		path, err := uri.Path()
		if err == nil {
			appendPath(path)
		}
	}

	for _, change := range edit.DocumentChanges {
		if change.TextDocumentEdit != nil {
			path, err := change.TextDocumentEdit.TextDocument.URI.Path()
			if err == nil {
				appendPath(path)
			}
		}
		if change.CreateFile != nil {
			path, err := change.CreateFile.URI.Path()
			if err == nil {
				appendPath(path)
			}
		}
		if change.DeleteFile != nil {
			path, err := change.DeleteFile.URI.Path()
			if err == nil {
				appendPath(path)
			}
		}
		if change.RenameFile != nil {
			oldPath, oldErr := change.RenameFile.OldURI.Path()
			if oldErr == nil {
				appendPath(oldPath)
			}
			newPath, newErr := change.RenameFile.NewURI.Path()
			if newErr == nil {
				appendPath(newPath)
			}
		}
	}

	sort.Strings(paths)
	return paths
}

func normalizeWorkspaceEditPath(path string) string {
	path = strings.TrimSpace(path)
	if len(path) >= 3 && path[0] == '\\' && path[2] == ':' {
		return path[1:]
	}
	return path
}
