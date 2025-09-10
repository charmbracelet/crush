package lsp

import (
	"encoding/json"
	"log/slog"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

// ServerRequestHandler handles LSP server requests
type ServerRequestHandler func(params json.RawMessage) (any, error)

// HandleDiagnostics handles diagnostic notifications from the LSP server
func HandleDiagnostics(client *Client, params json.RawMessage) {
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(params, &diagParams); err != nil {
		slog.Error("Error unmarshaling diagnostic params", "error", err)
		return
	}

	// Convert to our internal format and update the client's diagnostic cache
	uri := protocol.DocumentURI(diagParams.URI)
	diagnostics := make([]protocol.Diagnostic, len(diagParams.Diagnostics))

	for i, diag := range diagParams.Diagnostics {
		diagnostics[i] = protocol.Diagnostic{
			Range:    diag.Range,
			Severity: diag.Severity,
			Code:     diag.Code,
			Source:   diag.Source,
			Message:  diag.Message,
		}
	}

	// Clear old diagnostics and set new ones
	client.ClearDiagnosticsForURI(uri)
	// Note: We can't directly set diagnostics on the interface,
	// so we rely on the client's internal notification handler
}

// HandleServerMessage handles server messages
func HandleServerMessage(params json.RawMessage) {
	var msgParams protocol.ShowMessageParams
	if err := json.Unmarshal(params, &msgParams); err != nil {
		slog.Error("Error unmarshaling message params", "error", err)
		return
	}

	switch msgParams.Type {
	case protocol.Error:
		slog.Error("LSP Server", "message", msgParams.Message)
	case protocol.Warning:
		slog.Warn("LSP Server", "message", msgParams.Message)
	case protocol.Info:
		slog.Info("LSP Server", "message", msgParams.Message)
	case protocol.Log:
		slog.Debug("LSP Server", "message", msgParams.Message)
	}
}

// HandleWorkspaceConfiguration handles workspace configuration requests
func HandleWorkspaceConfiguration(params json.RawMessage) (any, error) {
	return []map[string]any{{}}, nil
}

// HandleRegisterCapability handles capability registration requests
func HandleRegisterCapability(params json.RawMessage) (any, error) {
	return nil, nil
}

// HandleApplyEdit handles workspace edit requests
func HandleApplyEdit(params json.RawMessage) (any, error) {
	return map[string]bool{"applied": false}, nil
}
