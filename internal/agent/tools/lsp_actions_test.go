package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
	"github.com/stretchr/testify/require"
)

func runLSPActionTool(t *testing.T, tool fantasy.AgentTool, name string, params any) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{ID: "call-1", Name: name, Input: string(input)})
	require.NoError(t, err)
	return resp
}

func TestLSPActionToolsRequireLSPClients(t *testing.T) {
	t.Parallel()

	codeActionResp := runLSPActionTool(t, NewLSPCodeActionTool(nil, nil, ""), LSPCodeActionToolName, LSPCodeActionParams{
		FilePath:  "main.go",
		Line:      1,
		Character: 1,
	})
	require.True(t, codeActionResp.IsError)
	require.Contains(t, codeActionResp.Content, "no LSP clients available")

	renameResp := runLSPActionTool(t, NewLSPRenameTool(nil, nil, ""), LSPRenameToolName, LSPRenameParams{
		FilePath:  "main.go",
		Line:      1,
		Character: 1,
		NewName:   "mainRenamed",
	})
	require.True(t, renameResp.IsError)
	require.Contains(t, renameResp.Content, "no LSP clients available")

	formatResp := runLSPActionTool(t, NewLSPFormatTool(nil, nil, ""), LSPFormatToolName, LSPFormatParams{
		FilePath: "main.go",
	})
	require.True(t, formatResp.IsError)
	require.Contains(t, formatResp.Content, "no LSP clients available")
}

func TestFormattingOptionsDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	defaults := formattingOptions(LSPFormatParams{})
	require.Equal(t, uint32(4), defaults.TabSize)
	require.True(t, defaults.InsertSpaces)
	require.False(t, defaults.TrimTrailingWhitespace)
	require.False(t, defaults.InsertFinalNewline)
	require.False(t, defaults.TrimFinalNewlines)

	insertSpaces := false
	trimTrailing := true
	insertFinal := true
	trimFinal := true
	overrides := formattingOptions(LSPFormatParams{
		TabSize:                2,
		InsertSpaces:           &insertSpaces,
		TrimTrailingWhitespace: &trimTrailing,
		InsertFinalNewline:     &insertFinal,
		TrimFinalNewlines:      &trimFinal,
	})
	require.Equal(t, uint32(2), overrides.TabSize)
	require.False(t, overrides.InsertSpaces)
	require.True(t, overrides.TrimTrailingWhitespace)
	require.True(t, overrides.InsertFinalNewline)
	require.True(t, overrides.TrimFinalNewlines)
}

func TestFormatCodeActions(t *testing.T) {
	t.Parallel()

	output := formatCodeActions([]protocol.CodeAction{
		{Title: "Apply quick fix", Kind: protocol.QuickFix, Edit: &protocol.WorkspaceEdit{}},
		{Title: "Run command", Command: &protocol.Command{Title: "Run command", Command: "go.test"}},
		{Title: "Disabled action", Disabled: &protocol.CodeActionDisabled{Reason: "unsupported context"}},
	})

	require.Contains(t, output, "Found 3 code action(s):")
	require.Contains(t, output, "1. Apply quick fix [quickfix] (edit)")
	require.Contains(t, output, "2. Run command (command)")
	require.Contains(t, output, "3. Disabled action (disabled: unsupported context)")
}

func TestWorkspaceEditHelpers(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	fileA := filepath.Join(tempDir, "a.go")
	fileB := filepath.Join(tempDir, "b.go")
	fileC := filepath.Join(tempDir, "c.go")
	fileD := filepath.Join(tempDir, "d.go")
	fileE := filepath.Join(tempDir, "e.go")

	edit := protocol.WorkspaceEdit{
		Changes: map[protocol.DocumentURI][]protocol.TextEdit{
			protocol.URIFromPath(fileA): {},
			protocol.URIFromPath(fileB): {},
		},
		DocumentChanges: []protocol.DocumentChange{
			{
				TextDocumentEdit: &protocol.TextDocumentEdit{
					TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
						Version: 1,
						TextDocumentIdentifier: protocol.TextDocumentIdentifier{
							URI: protocol.URIFromPath(fileC),
						},
					},
					Edits: []protocol.Or_TextDocumentEdit_edits_Elem{{
						Value: protocol.TextEdit{},
					}},
				},
			},
			{CreateFile: &protocol.CreateFile{URI: protocol.URIFromPath(fileD)}},
			{RenameFile: &protocol.RenameFile{OldURI: protocol.URIFromPath(fileA), NewURI: protocol.URIFromPath(fileE)}},
		},
	}

	require.False(t, workspaceEditEmpty(edit))
	paths := workspaceEditPaths(edit)
	require.Equal(t, []string{fileA, fileB, fileC, fileD, fileE}, paths)
	require.True(t, workspaceEditEmpty(protocol.WorkspaceEdit{}))
}
