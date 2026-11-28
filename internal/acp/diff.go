package acp

import (
	"github.com/charmbracelet/crush/internal/agent/tools"
	acp "github.com/coder/acp-go-sdk"
)

// toolDiffContent builds ACP diff content from a file-modifying tool's
// permission params, so the client can render the change before it is
// approved. The params are the ones the tools pass to permission.Request
// (edit, write, multiedit, lsp_replace_symbol); each carries the file path and
// its old and new content. An empty old content means the file is being
// created, which the protocol represents as a diff without oldText.
func toolDiffContent(abs func(string) string, params any) []acp.ToolCallContent {
	path, oldContent, newContent, ok := diffParams(params)
	if !ok || newContent == "" || newContent == oldContent {
		return nil
	}
	if abs != nil {
		path = abs(path)
	}
	if oldContent == "" {
		return []acp.ToolCallContent{acp.ToolDiffContent(path, newContent)}
	}
	return []acp.ToolCallContent{acp.ToolDiffContent(path, newContent, oldContent)}
}

// toolFilePath returns the file a file-modifying tool acts on, or "" for a
// tool that names no file. It comes from the tool's own params rather than the
// request path, which for a write is the containing directory.
func toolFilePath(params any) string {
	path, _, _, _ := diffParams(params)
	return path
}

// diffParams extracts the file path and the old and new content from a
// file-modifying tool's permission params. A tool that changes no file content
// reports ok as false, so it contributes no diff.
func diffParams(params any) (path, oldContent, newContent string, ok bool) {
	switch p := params.(type) {
	case tools.EditPermissionsParams:
		return p.FilePath, p.OldContent, p.NewContent, true
	case tools.WritePermissionsParams:
		return p.FilePath, p.OldContent, p.NewContent, true
	case tools.MultiEditPermissionsParams:
		return p.FilePath, p.OldContent, p.NewContent, true
	case tools.ReplaceSymbolPermissionsParams:
		return p.FilePath, p.OldContent, p.NewContent, true
	default:
		return "", "", "", false
	}
}
