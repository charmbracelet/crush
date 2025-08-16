package lsp

import "github.com/charmbracelet/crush/internal/lsp/protocol"

func (c *Client) setCapabilities(caps protocol.ServerCapabilities) {
	c.capsMu.Lock()
	defer c.capsMu.Unlock()
	c.caps = caps
	c.capsSet.Store(true)
}

func (c *Client) getCapabilities() (protocol.ServerCapabilities, bool) {
	c.capsMu.RLock()
	defer c.capsMu.RUnlock()
	return c.caps, c.capsSet.Load()
}

func (c *Client) IsMethodSupported(method string) bool {
	// Always allow core lifecycle and generic methods
	switch method {
	case "initialize", "shutdown", "exit", "$/cancelRequest":
		return true
	}

	caps, ok := c.getCapabilities()
	if !ok {
		// caps not set yet, be permissive
		return true
	}

	switch method {
	case "textDocument/hover":
		return caps.HoverProvider != nil
	case "textDocument/definition":
		return caps.DefinitionProvider != nil
	case "textDocument/references":
		return caps.ReferencesProvider != nil
	case "textDocument/implementation":
		return caps.ImplementationProvider != nil
	case "textDocument/typeDefinition":
		return caps.TypeDefinitionProvider != nil
	case "textDocument/documentColor", "textDocument/colorPresentation":
		return caps.ColorProvider != nil
	case "textDocument/foldingRange":
		return caps.FoldingRangeProvider != nil
	case "textDocument/declaration":
		return caps.DeclarationProvider != nil
	case "textDocument/selectionRange":
		return caps.SelectionRangeProvider != nil
	case "textDocument/prepareCallHierarchy", "callHierarchy/incomingCalls", "callHierarchy/outgoingCalls":
		return caps.CallHierarchyProvider != nil
	case "textDocument/semanticTokens/full", "textDocument/semanticTokens/full/delta", "textDocument/semanticTokens/range":
		return caps.SemanticTokensProvider != nil
	case "textDocument/linkedEditingRange":
		return caps.LinkedEditingRangeProvider != nil
	case "workspace/willCreateFiles":
		return caps.Workspace != nil && caps.Workspace.FileOperations != nil && caps.Workspace.FileOperations.WillCreate != nil
	case "workspace/willRenameFiles":
		return caps.Workspace != nil && caps.Workspace.FileOperations != nil && caps.Workspace.FileOperations.WillRename != nil
	case "workspace/willDeleteFiles":
		return caps.Workspace != nil && caps.Workspace.FileOperations != nil && caps.Workspace.FileOperations.WillDelete != nil
	case "textDocument/moniker":
		return caps.MonikerProvider != nil
	case "textDocument/prepareTypeHierarchy", "typeHierarchy/supertypes", "typeHierarchy/subtypes":
		return caps.TypeHierarchyProvider != nil
	case "textDocument/inlineValue":
		return caps.InlineValueProvider != nil
	case "textDocument/inlayHint", "inlayHint/resolve":
		return caps.InlayHintProvider != nil
	case "textDocument/diagnostic", "workspace/diagnostic":
		return caps.DiagnosticProvider != nil
	case "textDocument/inlineCompletion":
		return caps.InlineCompletionProvider != nil
	case "workspace/textDocumentContent":
		return caps.Workspace != nil && caps.Workspace.TextDocumentContent != nil
	case "textDocument/willSaveWaitUntil":
		if caps.TextDocumentSync == nil {
			return false
		}
		return true
	case "textDocument/completion", "completionItem/resolve":
		return caps.CompletionProvider != nil
	case "textDocument/signatureHelp":
		return caps.SignatureHelpProvider != nil
	case "textDocument/documentHighlight":
		return caps.DocumentHighlightProvider != nil
	case "textDocument/documentSymbol":
		return caps.DocumentSymbolProvider != nil
	case "textDocument/codeAction", "codeAction/resolve":
		return caps.CodeActionProvider != nil
	case "workspace/symbol", "workspaceSymbol/resolve":
		return caps.WorkspaceSymbolProvider != nil
	case "textDocument/codeLens", "codeLens/resolve":
		return caps.CodeLensProvider != nil
	case "textDocument/documentLink", "documentLink/resolve":
		return caps.DocumentLinkProvider != nil
	case "textDocument/formatting":
		return caps.DocumentFormattingProvider != nil
	case "textDocument/rangeFormatting":
		return caps.DocumentRangeFormattingProvider != nil
	case "textDocument/rangesFormatting":
		return caps.DocumentRangeFormattingProvider != nil
	case "textDocument/onTypeFormatting":
		return caps.DocumentOnTypeFormattingProvider != nil
	case "textDocument/rename", "textDocument/prepareRename":
		return caps.RenameProvider != nil
	case "workspace/executeCommand":
		return caps.ExecuteCommandProvider != nil
	default:
		return true
	}
}
