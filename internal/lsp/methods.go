// Generated code. Do not edit
package lsp

import (
	"context"

	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

// Implementation sends a textDocument/implementation request to the LSP server.
// A request to resolve the implementation locations of a symbol at a given text document position. The request's parameter is of type TextDocumentPositionParams the response is of type Definition or a Thenable that resolves to such.
func (c *Client) Implementation(ctx context.Context, params protocol.ImplementationParams) (protocol.Or_Result_textDocument_implementation, error) {
	var result protocol.Or_Result_textDocument_implementation
	err := c.Call(ctx, "textDocument/implementation", params, &result)
	return result, err
}

// TypeDefinition sends a textDocument/typeDefinition request to the LSP server.
// A request to resolve the type definition locations of a symbol at a given text document position. The request's parameter is of type TextDocumentPositionParams the response is of type Definition or a Thenable that resolves to such.
func (c *Client) TypeDefinition(ctx context.Context, params protocol.TypeDefinitionParams) (protocol.Or_Result_textDocument_typeDefinition, error) {
	var result protocol.Or_Result_textDocument_typeDefinition
	err := c.Call(ctx, "textDocument/typeDefinition", params, &result)
	return result, err
}

// DocumentColor sends a textDocument/documentColor request to the LSP server.
// A request to list all color symbols found in a given text document. The request's parameter is of type DocumentColorParams the response is of type ColorInformation ColorInformation[] or a Thenable that resolves to such.
func (c *Client) DocumentColor(ctx context.Context, params protocol.DocumentColorParams) ([]protocol.ColorInformation, error) {
	var result []protocol.ColorInformation
	err := c.Call(ctx, "textDocument/documentColor", params, &result)
	return result, err
}

// ColorPresentation sends a textDocument/colorPresentation request to the LSP server.
// A request to list all presentation for a color. The request's parameter is of type ColorPresentationParams the response is of type ColorInformation ColorInformation[] or a Thenable that resolves to such.
func (c *Client) ColorPresentation(ctx context.Context, params protocol.ColorPresentationParams) ([]protocol.ColorPresentation, error) {
	var result []protocol.ColorPresentation
	err := c.Call(ctx, "textDocument/colorPresentation", params, &result)
	return result, err
}

// FoldingRange sends a textDocument/foldingRange request to the LSP server.
// A request to provide folding ranges in a document. The request's parameter is of type FoldingRangeParams, the response is of type FoldingRangeList or a Thenable that resolves to such.
func (c *Client) FoldingRange(ctx context.Context, params protocol.FoldingRangeParams) ([]protocol.FoldingRange, error) {
	var result []protocol.FoldingRange
	err := c.Call(ctx, "textDocument/foldingRange", params, &result)
	return result, err
}

// Declaration sends a textDocument/declaration request to the LSP server.
// A request to resolve the type definition locations of a symbol at a given text document position. The request's parameter is of type TextDocumentPositionParams the response is of type Declaration or a typed array of DeclarationLink or a Thenable that resolves to such.
func (c *Client) Declaration(ctx context.Context, params protocol.DeclarationParams) (protocol.Or_Result_textDocument_declaration, error) {
	var result protocol.Or_Result_textDocument_declaration
	err := c.Call(ctx, "textDocument/declaration", params, &result)
	return result, err
}

// SelectionRange sends a textDocument/selectionRange request to the LSP server.
// A request to provide selection ranges in a document. The request's parameter is of type SelectionRangeParams, the response is of type SelectionRange SelectionRange[] or a Thenable that resolves to such.
func (c *Client) SelectionRange(ctx context.Context, params protocol.SelectionRangeParams) ([]protocol.SelectionRange, error) {
	var result []protocol.SelectionRange
	err := c.Call(ctx, "textDocument/selectionRange", params, &result)
	return result, err
}

// PrepareCallHierarchy sends a textDocument/prepareCallHierarchy request to the LSP server.
// A request to result a CallHierarchyItem in a document at a given position. Can be used as an input to an incoming or outgoing call hierarchy. Since 3.16.0
func (c *Client) PrepareCallHierarchy(ctx context.Context, params protocol.CallHierarchyPrepareParams) ([]protocol.CallHierarchyItem, error) {
	var result []protocol.CallHierarchyItem
	err := c.Call(ctx, "textDocument/prepareCallHierarchy", params, &result)
	return result, err
}

// IncomingCalls sends a callHierarchy/incomingCalls request to the LSP server.
// A request to resolve the incoming calls for a given CallHierarchyItem. Since 3.16.0
func (c *Client) IncomingCalls(ctx context.Context, params protocol.CallHierarchyIncomingCallsParams) ([]protocol.CallHierarchyIncomingCall, error) {
	var result []protocol.CallHierarchyIncomingCall
	err := c.Call(ctx, "callHierarchy/incomingCalls", params, &result)
	return result, err
}

// OutgoingCalls sends a callHierarchy/outgoingCalls request to the LSP server.
// A request to resolve the outgoing calls for a given CallHierarchyItem. Since 3.16.0
func (c *Client) OutgoingCalls(ctx context.Context, params protocol.CallHierarchyOutgoingCallsParams) ([]protocol.CallHierarchyOutgoingCall, error) {
	var result []protocol.CallHierarchyOutgoingCall
	err := c.Call(ctx, "callHierarchy/outgoingCalls", params, &result)
	return result, err
}

// SemanticTokensFull sends a textDocument/semanticTokens/full request to the LSP server.
// Since 3.16.0
func (c *Client) SemanticTokensFull(ctx context.Context, params protocol.SemanticTokensParams) (protocol.SemanticTokens, error) {
	var result protocol.SemanticTokens
	err := c.Call(ctx, "textDocument/semanticTokens/full", params, &result)
	return result, err
}

// SemanticTokensFullDelta sends a textDocument/semanticTokens/full/delta request to the LSP server.
// Since 3.16.0
func (c *Client) SemanticTokensFullDelta(ctx context.Context, params protocol.SemanticTokensDeltaParams) (protocol.Or_Result_textDocument_semanticTokens_full_delta, error) {
	var result protocol.Or_Result_textDocument_semanticTokens_full_delta
	err := c.Call(ctx, "textDocument/semanticTokens/full/delta", params, &result)
	return result, err
}

// SemanticTokensRange sends a textDocument/semanticTokens/range request to the LSP server.
// Since 3.16.0
func (c *Client) SemanticTokensRange(ctx context.Context, params protocol.SemanticTokensRangeParams) (protocol.SemanticTokens, error) {
	var result protocol.SemanticTokens
	err := c.Call(ctx, "textDocument/semanticTokens/range", params, &result)
	return result, err
}

// LinkedEditingRange sends a textDocument/linkedEditingRange request to the LSP server.
// A request to provide ranges that can be edited together. Since 3.16.0
func (c *Client) LinkedEditingRange(ctx context.Context, params protocol.LinkedEditingRangeParams) (protocol.LinkedEditingRanges, error) {
	var result protocol.LinkedEditingRanges
	err := c.Call(ctx, "textDocument/linkedEditingRange", params, &result)
	return result, err
}

// WillCreateFiles sends a workspace/willCreateFiles request to the LSP server.
// The will create files request is sent from the client to the server before files are actually created as long as the creation is triggered from within the client. The request can return a WorkspaceEdit which will be applied to workspace before the files are created. Hence the WorkspaceEdit can not manipulate the content of the file to be created. Since 3.16.0
func (c *Client) WillCreateFiles(ctx context.Context, params protocol.CreateFilesParams) (protocol.WorkspaceEdit, error) {
	var result protocol.WorkspaceEdit
	err := c.Call(ctx, "workspace/willCreateFiles", params, &result)
	return result, err
}

// WillRenameFiles sends a workspace/willRenameFiles request to the LSP server.
// The will rename files request is sent from the client to the server before files are actually renamed as long as the rename is triggered from within the client. Since 3.16.0
func (c *Client) WillRenameFiles(ctx context.Context, params protocol.RenameFilesParams) (protocol.WorkspaceEdit, error) {
	var result protocol.WorkspaceEdit
	err := c.Call(ctx, "workspace/willRenameFiles", params, &result)
	return result, err
}

// WillDeleteFiles sends a workspace/willDeleteFiles request to the LSP server.
// The did delete files notification is sent from the client to the server when files were deleted from within the client. Since 3.16.0
func (c *Client) WillDeleteFiles(ctx context.Context, params protocol.DeleteFilesParams) (protocol.WorkspaceEdit, error) {
	var result protocol.WorkspaceEdit
	err := c.Call(ctx, "workspace/willDeleteFiles", params, &result)
	return result, err
}

// Moniker sends a textDocument/moniker request to the LSP server.
// A request to get the moniker of a symbol at a given text document position. The request parameter is of type TextDocumentPositionParams. The response is of type Moniker Moniker[] or null.
func (c *Client) Moniker(ctx context.Context, params protocol.MonikerParams) ([]protocol.Moniker, error) {
	var result []protocol.Moniker
	err := c.Call(ctx, "textDocument/moniker", params, &result)
	return result, err
}

// PrepareTypeHierarchy sends a textDocument/prepareTypeHierarchy request to the LSP server.
// A request to result a TypeHierarchyItem in a document at a given position. Can be used as an input to a subtypes or supertypes type hierarchy. Since 3.17.0
func (c *Client) PrepareTypeHierarchy(ctx context.Context, params protocol.TypeHierarchyPrepareParams) ([]protocol.TypeHierarchyItem, error) {
	var result []protocol.TypeHierarchyItem
	err := c.Call(ctx, "textDocument/prepareTypeHierarchy", params, &result)
	return result, err
}

// Supertypes sends a typeHierarchy/supertypes request to the LSP server.
// A request to resolve the supertypes for a given TypeHierarchyItem. Since 3.17.0
func (c *Client) Supertypes(ctx context.Context, params protocol.TypeHierarchySupertypesParams) ([]protocol.TypeHierarchyItem, error) {
	var result []protocol.TypeHierarchyItem
	err := c.Call(ctx, "typeHierarchy/supertypes", params, &result)
	return result, err
}

// Subtypes sends a typeHierarchy/subtypes request to the LSP server.
// A request to resolve the subtypes for a given TypeHierarchyItem. Since 3.17.0
func (c *Client) Subtypes(ctx context.Context, params protocol.TypeHierarchySubtypesParams) ([]protocol.TypeHierarchyItem, error) {
	var result []protocol.TypeHierarchyItem
	err := c.Call(ctx, "typeHierarchy/subtypes", params, &result)
	return result, err
}

// InlineValue sends a textDocument/inlineValue request to the LSP server.
// A request to provide inline values in a document. The request's parameter is of type InlineValueParams, the response is of type InlineValue InlineValue[] or a Thenable that resolves to such. Since 3.17.0
func (c *Client) InlineValue(ctx context.Context, params protocol.InlineValueParams) ([]protocol.InlineValue, error) {
	var result []protocol.InlineValue
	err := c.Call(ctx, "textDocument/inlineValue", params, &result)
	return result, err
}

// InlayHint sends a textDocument/inlayHint request to the LSP server.
// A request to provide inlay hints in a document. The request's parameter is of type InlayHintsParams, the response is of type InlayHint InlayHint[] or a Thenable that resolves to such. Since 3.17.0
func (c *Client) InlayHint(ctx context.Context, params protocol.InlayHintParams) ([]protocol.InlayHint, error) {
	var result []protocol.InlayHint
	err := c.Call(ctx, "textDocument/inlayHint", params, &result)
	return result, err
}

// Resolve sends a inlayHint/resolve request to the LSP server.
// A request to resolve additional properties for an inlay hint. The request's parameter is of type InlayHint, the response is of type InlayHint or a Thenable that resolves to such. Since 3.17.0
func (c *Client) Resolve(ctx context.Context, params protocol.InlayHint) (protocol.InlayHint, error) {
	var result protocol.InlayHint
	err := c.Call(ctx, "inlayHint/resolve", params, &result)
	return result, err
}

// Diagnostic sends a textDocument/diagnostic request to the LSP server.
// The document diagnostic request definition. Since 3.17.0
func (c *Client) Diagnostic(ctx context.Context, params protocol.DocumentDiagnosticParams) (protocol.DocumentDiagnosticReport, error) {
	var result protocol.DocumentDiagnosticReport
	err := c.Call(ctx, "textDocument/diagnostic", params, &result)
	return result, err
}

// DiagnosticWorkspace sends a workspace/diagnostic request to the LSP server.
// The workspace diagnostic request definition. Since 3.17.0
func (c *Client) DiagnosticWorkspace(ctx context.Context, params protocol.WorkspaceDiagnosticParams) (protocol.WorkspaceDiagnosticReport, error) {
	var result protocol.WorkspaceDiagnosticReport
	err := c.Call(ctx, "workspace/diagnostic", params, &result)
	return result, err
}

// InlineCompletion sends a textDocument/inlineCompletion request to the LSP server.
// A request to provide inline completions in a document. The request's parameter is of type InlineCompletionParams, the response is of type InlineCompletion InlineCompletion[] or a Thenable that resolves to such. Since 3.18.0 PROPOSED
func (c *Client) InlineCompletion(ctx context.Context, params protocol.InlineCompletionParams) (protocol.Or_Result_textDocument_inlineCompletion, error) {
	var result protocol.Or_Result_textDocument_inlineCompletion
	err := c.Call(ctx, "textDocument/inlineCompletion", params, &result)
	return result, err
}

// TextDocumentContent sends a workspace/textDocumentContent request to the LSP server.
// The workspace/textDocumentContent request is sent from the client to the server to request the content of a text document. Since 3.18.0 PROPOSED
func (c *Client) TextDocumentContent(ctx context.Context, params protocol.TextDocumentContentParams) (string, error) {
	var result string
	err := c.Call(ctx, "workspace/textDocumentContent", params, &result)
	return result, err
}

// Initialize sends a initialize request to the LSP server.
// The initialize request is sent from the client to the server. It is sent once as the request after starting up the server. The requests parameter is of type InitializeParams the response if of type InitializeResult of a Thenable that resolves to such.
func (c *Client) Initialize(ctx context.Context, params protocol.ParamInitialize) (protocol.InitializeResult, error) {
	var result protocol.InitializeResult
	err := c.Call(ctx, "initialize", params, &result)
	return result, err
}

// Shutdown sends a shutdown request to the LSP server.
// A shutdown request is sent from the client to the server. It is sent once when the client decides to shutdown the server. The only notification that is sent after a shutdown request is the exit event.
func (c *Client) Shutdown(ctx context.Context) error {
	return c.Call(ctx, "shutdown", nil, nil)
}

// WillSaveWaitUntil sends a textDocument/willSaveWaitUntil request to the LSP server.
// A document will save request is sent from the client to the server before the document is actually saved. The request can return an array of TextEdits which will be applied to the text document before it is saved. Please note that clients might drop results if computing the text edits took too long or if a server constantly fails on this request. This is done to keep the save fast and reliable.
func (c *Client) WillSaveWaitUntil(ctx context.Context, params protocol.WillSaveTextDocumentParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit
	err := c.Call(ctx, "textDocument/willSaveWaitUntil", params, &result)
	return result, err
}

// Completion sends a textDocument/completion request to the LSP server.
// Request to request completion at a given text document position. The request's parameter is of type TextDocumentPosition the response is of type CompletionItem CompletionItem[] or CompletionList or a Thenable that resolves to such. The request can delay the computation of the CompletionItem.detail detail and CompletionItem.documentation documentation properties to the completionItem/resolve request. However, properties that are needed for the initial sorting and filtering, like sortText, filterText, insertText, and textEdit, must not be changed during resolve.
func (c *Client) Completion(ctx context.Context, params protocol.CompletionParams) (protocol.Or_Result_textDocument_completion, error) {
	var result protocol.Or_Result_textDocument_completion
	err := c.Call(ctx, "textDocument/completion", params, &result)
	return result, err
}

// ResolveCompletionItem sends a completionItem/resolve request to the LSP server.
// Request to resolve additional information for a given completion item.The request's parameter is of type CompletionItem the response is of type CompletionItem or a Thenable that resolves to such.
func (c *Client) ResolveCompletionItem(ctx context.Context, params protocol.CompletionItem) (protocol.CompletionItem, error) {
	var result protocol.CompletionItem
	err := c.Call(ctx, "completionItem/resolve", params, &result)
	return result, err
}

// Hover sends a textDocument/hover request to the LSP server.
// Request to request hover information at a given text document position. The request's parameter is of type TextDocumentPosition the response is of type Hover or a Thenable that resolves to such.
func (c *Client) Hover(ctx context.Context, params protocol.HoverParams) (protocol.Hover, error) {
	var result protocol.Hover
	err := c.Call(ctx, "textDocument/hover", params, &result)
	return result, err
}

// SignatureHelp sends a textDocument/signatureHelp request to the LSP server.
func (c *Client) SignatureHelp(ctx context.Context, params protocol.SignatureHelpParams) (protocol.SignatureHelp, error) {
	var result protocol.SignatureHelp
	err := c.Call(ctx, "textDocument/signatureHelp", params, &result)
	return result, err
}

// Definition sends a textDocument/definition request to the LSP server.
// A request to resolve the definition location of a symbol at a given text document position. The request's parameter is of type TextDocumentPosition the response is of either type Definition or a typed array of DefinitionLink or a Thenable that resolves to such.
func (c *Client) Definition(ctx context.Context, params protocol.DefinitionParams) (protocol.Or_Result_textDocument_definition, error) {
	var result protocol.Or_Result_textDocument_definition
	err := c.Call(ctx, "textDocument/definition", params, &result)
	return result, err
}

// References sends a textDocument/references request to the LSP server.
// A request to resolve project-wide references for the symbol denoted by the given text document position. The request's parameter is of type ReferenceParams the response is of type Location Location[] or a Thenable that resolves to such.
func (c *Client) References(ctx context.Context, params protocol.ReferenceParams) ([]protocol.Location, error) {
	var result []protocol.Location
	err := c.Call(ctx, "textDocument/references", params, &result)
	return result, err
}

// DocumentHighlight sends a textDocument/documentHighlight request to the LSP server.
// Request to resolve a DocumentHighlight for a given text document position. The request's parameter is of type TextDocumentPosition the request response is an array of type DocumentHighlight or a Thenable that resolves to such.
func (c *Client) DocumentHighlight(ctx context.Context, params protocol.DocumentHighlightParams) ([]protocol.DocumentHighlight, error) {
	var result []protocol.DocumentHighlight
	err := c.Call(ctx, "textDocument/documentHighlight", params, &result)
	return result, err
}

// DocumentSymbol sends a textDocument/documentSymbol request to the LSP server.
// A request to list all symbols found in a given text document. The request's parameter is of type TextDocumentIdentifier the response is of type SymbolInformation SymbolInformation[] or a Thenable that resolves to such.
func (c *Client) DocumentSymbol(ctx context.Context, params protocol.DocumentSymbolParams) (protocol.Or_Result_textDocument_documentSymbol, error) {
	var result protocol.Or_Result_textDocument_documentSymbol
	err := c.Call(ctx, "textDocument/documentSymbol", params, &result)
	return result, err
}

// CodeAction sends a textDocument/codeAction request to the LSP server.
// A request to provide commands for the given text document and range.
func (c *Client) CodeAction(ctx context.Context, params protocol.CodeActionParams) ([]protocol.Or_Result_textDocument_codeAction_Item0_Elem, error) {
	var result []protocol.Or_Result_textDocument_codeAction_Item0_Elem
	err := c.Call(ctx, "textDocument/codeAction", params, &result)
	return result, err
}

// ResolveCodeAction sends a codeAction/resolve request to the LSP server.
// Request to resolve additional information for a given code action.The request's parameter is of type CodeAction the response is of type CodeAction or a Thenable that resolves to such.
func (c *Client) ResolveCodeAction(ctx context.Context, params protocol.CodeAction) (protocol.CodeAction, error) {
	var result protocol.CodeAction
	err := c.Call(ctx, "codeAction/resolve", params, &result)
	return result, err
}

// Symbol sends a workspace/symbol request to the LSP server.
// A request to list project-wide symbols matching the query string given by the WorkspaceSymbolParams. The response is of type SymbolInformation SymbolInformation[] or a Thenable that resolves to such. Since 3.17.0 - support for WorkspaceSymbol in the returned data. Clients need to advertise support for WorkspaceSymbols via the client capability workspace.symbol.resolveSupport.
func (c *Client) Symbol(ctx context.Context, params protocol.WorkspaceSymbolParams) (protocol.Or_Result_workspace_symbol, error) {
	var result protocol.Or_Result_workspace_symbol
	err := c.Call(ctx, "workspace/symbol", params, &result)
	return result, err
}

// ResolveWorkspaceSymbol sends a workspaceSymbol/resolve request to the LSP server.
// A request to resolve the range inside the workspace symbol's location. Since 3.17.0
func (c *Client) ResolveWorkspaceSymbol(ctx context.Context, params protocol.WorkspaceSymbol) (protocol.WorkspaceSymbol, error) {
	var result protocol.WorkspaceSymbol
	err := c.Call(ctx, "workspaceSymbol/resolve", params, &result)
	return result, err
}

// CodeLens sends a textDocument/codeLens request to the LSP server.
// A request to provide code lens for the given text document.
func (c *Client) CodeLens(ctx context.Context, params protocol.CodeLensParams) ([]protocol.CodeLens, error) {
	var result []protocol.CodeLens
	err := c.Call(ctx, "textDocument/codeLens", params, &result)
	return result, err
}

// ResolveCodeLens sends a codeLens/resolve request to the LSP server.
// A request to resolve a command for a given code lens.
func (c *Client) ResolveCodeLens(ctx context.Context, params protocol.CodeLens) (protocol.CodeLens, error) {
	var result protocol.CodeLens
	err := c.Call(ctx, "codeLens/resolve", params, &result)
	return result, err
}

// DocumentLink sends a textDocument/documentLink request to the LSP server.
// A request to provide document links
func (c *Client) DocumentLink(ctx context.Context, params protocol.DocumentLinkParams) ([]protocol.DocumentLink, error) {
	var result []protocol.DocumentLink
	err := c.Call(ctx, "textDocument/documentLink", params, &result)
	return result, err
}

// ResolveDocumentLink sends a documentLink/resolve request to the LSP server.
// Request to resolve additional information for a given document link. The request's parameter is of type DocumentLink the response is of type DocumentLink or a Thenable that resolves to such.
func (c *Client) ResolveDocumentLink(ctx context.Context, params protocol.DocumentLink) (protocol.DocumentLink, error) {
	var result protocol.DocumentLink
	err := c.Call(ctx, "documentLink/resolve", params, &result)
	return result, err
}

// Formatting sends a textDocument/formatting request to the LSP server.
// A request to format a whole document.
func (c *Client) Formatting(ctx context.Context, params protocol.DocumentFormattingParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit
	err := c.Call(ctx, "textDocument/formatting", params, &result)
	return result, err
}

// RangeFormatting sends a textDocument/rangeFormatting request to the LSP server.
// A request to format a range in a document.
func (c *Client) RangeFormatting(ctx context.Context, params protocol.DocumentRangeFormattingParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit
	err := c.Call(ctx, "textDocument/rangeFormatting", params, &result)
	return result, err
}

// RangesFormatting sends a textDocument/rangesFormatting request to the LSP server.
// A request to format ranges in a document. Since 3.18.0 PROPOSED
func (c *Client) RangesFormatting(ctx context.Context, params protocol.DocumentRangesFormattingParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit
	err := c.Call(ctx, "textDocument/rangesFormatting", params, &result)
	return result, err
}

// OnTypeFormatting sends a textDocument/onTypeFormatting request to the LSP server.
// A request to format a document on type.
func (c *Client) OnTypeFormatting(ctx context.Context, params protocol.DocumentOnTypeFormattingParams) ([]protocol.TextEdit, error) {
	var result []protocol.TextEdit
	err := c.Call(ctx, "textDocument/onTypeFormatting", params, &result)
	return result, err
}

// Rename sends a textDocument/rename request to the LSP server.
// A request to rename a symbol.
func (c *Client) Rename(ctx context.Context, params protocol.RenameParams) (protocol.WorkspaceEdit, error) {
	var result protocol.WorkspaceEdit
	err := c.Call(ctx, "textDocument/rename", params, &result)
	return result, err
}

// PrepareRename sends a textDocument/prepareRename request to the LSP server.
// A request to test and perform the setup necessary for a rename. Since 3.16 - support for default behavior
func (c *Client) PrepareRename(ctx context.Context, params protocol.PrepareRenameParams) (protocol.PrepareRenameResult, error) {
	var result protocol.PrepareRenameResult
	err := c.Call(ctx, "textDocument/prepareRename", params, &result)
	return result, err
}

// ExecuteCommand sends a workspace/executeCommand request to the LSP server.
// A request send from the client to the server to execute a command. The request might return a workspace edit which the client will apply to the workspace.
func (c *Client) ExecuteCommand(ctx context.Context, params protocol.ExecuteCommandParams) (any, error) {
	var result any
	err := c.Call(ctx, "workspace/executeCommand", params, &result)
	return result, err
}

// DidChangeWorkspaceFolders sends a workspace/didChangeWorkspaceFolders notification to the LSP server.
// The workspace/didChangeWorkspaceFolders notification is sent from the client to the server when the workspace folder configuration changes.
func (c *Client) DidChangeWorkspaceFolders(ctx context.Context, params protocol.DidChangeWorkspaceFoldersParams) error {
	return c.Notify(ctx, "workspace/didChangeWorkspaceFolders", params)
}

// WorkDoneProgressCancel sends a window/workDoneProgress/cancel notification to the LSP server.
// The window/workDoneProgress/cancel notification is sent from  the client to the server to cancel a progress initiated on the server side.
func (c *Client) WorkDoneProgressCancel(ctx context.Context, params protocol.WorkDoneProgressCancelParams) error {
	return c.Notify(ctx, "window/workDoneProgress/cancel", params)
}

// DidCreateFiles sends a workspace/didCreateFiles notification to the LSP server.
// The did create files notification is sent from the client to the server when files were created from within the client. Since 3.16.0
func (c *Client) DidCreateFiles(ctx context.Context, params protocol.CreateFilesParams) error {
	return c.Notify(ctx, "workspace/didCreateFiles", params)
}

// DidRenameFiles sends a workspace/didRenameFiles notification to the LSP server.
// The did rename files notification is sent from the client to the server when files were renamed from within the client. Since 3.16.0
func (c *Client) DidRenameFiles(ctx context.Context, params protocol.RenameFilesParams) error {
	return c.Notify(ctx, "workspace/didRenameFiles", params)
}

// DidDeleteFiles sends a workspace/didDeleteFiles notification to the LSP server.
// The will delete files request is sent from the client to the server before files are actually deleted as long as the deletion is triggered from within the client. Since 3.16.0
func (c *Client) DidDeleteFiles(ctx context.Context, params protocol.DeleteFilesParams) error {
	return c.Notify(ctx, "workspace/didDeleteFiles", params)
}

// DidOpenNotebookDocument sends a notebookDocument/didOpen notification to the LSP server.
// A notification sent when a notebook opens. Since 3.17.0
func (c *Client) DidOpenNotebookDocument(ctx context.Context, params protocol.DidOpenNotebookDocumentParams) error {
	return c.Notify(ctx, "notebookDocument/didOpen", params)
}

// DidChangeNotebookDocument sends a notebookDocument/didChange notification to the LSP server.
func (c *Client) DidChangeNotebookDocument(ctx context.Context, params protocol.DidChangeNotebookDocumentParams) error {
	return c.Notify(ctx, "notebookDocument/didChange", params)
}

// DidSaveNotebookDocument sends a notebookDocument/didSave notification to the LSP server.
// A notification sent when a notebook document is saved. Since 3.17.0
func (c *Client) DidSaveNotebookDocument(ctx context.Context, params protocol.DidSaveNotebookDocumentParams) error {
	return c.Notify(ctx, "notebookDocument/didSave", params)
}

// DidCloseNotebookDocument sends a notebookDocument/didClose notification to the LSP server.
// A notification sent when a notebook closes. Since 3.17.0
func (c *Client) DidCloseNotebookDocument(ctx context.Context, params protocol.DidCloseNotebookDocumentParams) error {
	return c.Notify(ctx, "notebookDocument/didClose", params)
}

// Initialized sends a initialized notification to the LSP server.
// The initialized notification is sent from the client to the server after the client is fully initialized and the server is allowed to send requests from the server to the client.
func (c *Client) Initialized(ctx context.Context, params protocol.InitializedParams) error {
	return c.Notify(ctx, "initialized", params)
}

// Exit sends a exit notification to the LSP server.
// The exit event is sent from the client to the server to ask the server to exit its process.
func (c *Client) Exit(ctx context.Context) error {
	return c.Notify(ctx, "exit", nil)
}

// DidChangeConfiguration sends a workspace/didChangeConfiguration notification to the LSP server.
// The configuration change notification is sent from the client to the server when the client's configuration has changed. The notification contains the changed configuration as defined by the language client.
func (c *Client) DidChangeConfiguration(ctx context.Context, params protocol.DidChangeConfigurationParams) error {
	return c.Notify(ctx, "workspace/didChangeConfiguration", params)
}

// DidOpen sends a textDocument/didOpen notification to the LSP server.
// The document open notification is sent from the client to the server to signal newly opened text documents. The document's truth is now managed by the client and the server must not try to read the document's truth using the document's uri. Open in this sense means it is managed by the client. It doesn't necessarily mean that its content is presented in an editor. An open notification must not be sent more than once without a corresponding close notification send before. This means open and close notification must be balanced and the max open count is one.
func (c *Client) DidOpen(ctx context.Context, params protocol.DidOpenTextDocumentParams) error {
	return c.Notify(ctx, "textDocument/didOpen", params)
}

// DidChange sends a textDocument/didChange notification to the LSP server.
// The document change notification is sent from the client to the server to signal changes to a text document.
func (c *Client) DidChange(ctx context.Context, params protocol.DidChangeTextDocumentParams) error {
	return c.Notify(ctx, "textDocument/didChange", params)
}

// DidClose sends a textDocument/didClose notification to the LSP server.
// The document close notification is sent from the client to the server when the document got closed in the client. The document's truth now exists where the document's uri points to (e.g. if the document's uri is a file uri the truth now exists on disk). As with the open notification the close notification is about managing the document's content. Receiving a close notification doesn't mean that the document was open in an editor before. A close notification requires a previous open notification to be sent.
func (c *Client) DidClose(ctx context.Context, params protocol.DidCloseTextDocumentParams) error {
	return c.Notify(ctx, "textDocument/didClose", params)
}

// DidSave sends a textDocument/didSave notification to the LSP server.
// The document save notification is sent from the client to the server when the document got saved in the client.
func (c *Client) DidSave(ctx context.Context, params protocol.DidSaveTextDocumentParams) error {
	return c.Notify(ctx, "textDocument/didSave", params)
}

// WillSave sends a textDocument/willSave notification to the LSP server.
// A document will save notification is sent from the client to the server before the document is actually saved.
func (c *Client) WillSave(ctx context.Context, params protocol.WillSaveTextDocumentParams) error {
	return c.Notify(ctx, "textDocument/willSave", params)
}

// DidChangeWatchedFiles sends a workspace/didChangeWatchedFiles notification to the LSP server.
// The watched files notification is sent from the client to the server when the client detects changes to file watched by the language client.
func (c *Client) DidChangeWatchedFiles(ctx context.Context, params protocol.DidChangeWatchedFilesParams) error {
	return c.Notify(ctx, "workspace/didChangeWatchedFiles", params)
}

// SetTrace sends a $/setTrace notification to the LSP server.
func (c *Client) SetTrace(ctx context.Context, params protocol.SetTraceParams) error {
	return c.Notify(ctx, "$/setTrace", params)
}

// Progress sends a $/progress notification to the LSP server.
func (c *Client) Progress(ctx context.Context, params protocol.ProgressParams) error {
	return c.Notify(ctx, "$/progress", params)
}
