package lsp

import (
	"encoding/json"
	"log/slog"

	"github.com/charmbracelet/crush/internal/config"

	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/charmbracelet/crush/internal/lsp/util"
)

// Requests

func HandleWorkspaceConfiguration(params json.RawMessage) (any, error) {
	return []map[string]any{{}}, nil
}

func HandleRegisterCapability(params json.RawMessage) (any, error) {
	var registerParams protocol.RegistrationParams
	if err := json.Unmarshal(params, &registerParams); err != nil {
		slog.Error("Error unmarshaling registration params", "error", err)
		return nil, err
	}

	for _, reg := range registerParams.Registrations {
		switch reg.Method {
		case "workspace/didChangeWatchedFiles":
			// Parse the registration options
			optionsJSON, err := json.Marshal(reg.RegisterOptions)
			if err != nil {
				slog.Error("Error marshaling registration options", "error", err)
				continue
			}

			var options protocol.DidChangeWatchedFilesRegistrationOptions
			if err := json.Unmarshal(optionsJSON, &options); err != nil {
				slog.Error("Error unmarshaling registration options", "error", err)
				continue
			}

			// Store the file watchers registrations
			notifyFileWatchRegistration(reg.ID, options.Watchers)
		}
	}

	return nil, nil
}

func HandleApplyEdit(params json.RawMessage) (any, error) {
	var edit protocol.ApplyWorkspaceEditParams
	if err := json.Unmarshal(params, &edit); err != nil {
		return nil, err
	}

	err := util.ApplyWorkspaceEdit(edit.Edit)
	if err != nil {
		slog.Error("Error applying workspace edit", "error", err)
		return protocol.ApplyWorkspaceEditResult{Applied: false, FailureReason: err.Error()}, nil
	}

	return protocol.ApplyWorkspaceEditResult{Applied: true}, nil
}

// FileWatchRegistrationHandler is a function that will be called when file watch registrations are received
type FileWatchRegistrationHandler func(id string, watchers []protocol.FileSystemWatcher)

// fileWatchHandler holds the current handler for file watch registrations
var fileWatchHandler FileWatchRegistrationHandler

// RegisterFileWatchHandler sets the handler for file watch registrations
func RegisterFileWatchHandler(handler FileWatchRegistrationHandler) {
	fileWatchHandler = handler
}

// notifyFileWatchRegistration notifies the handler about new file watch registrations
func notifyFileWatchRegistration(id string, watchers []protocol.FileSystemWatcher) {
	if fileWatchHandler != nil {
		fileWatchHandler(id, watchers)
	}
}

// Notifications

func HandleServerMessage(params json.RawMessage) {
	cfg := config.Get()
	var msg struct {
		Type    int    `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(params, &msg); err == nil {
		if cfg.Options.DebugLSP {
			slog.Debug("Server message", "type", msg.Type, "message", msg.Message)
		}
	}
}

func HandleDiagnostics(client *Client, params json.RawMessage) {
	var diagParams protocol.PublishDiagnosticsParams
	if err := json.Unmarshal(params, &diagParams); err != nil {
		slog.Error("Error unmarshaling diagnostics params", "error", err)
		return
	}

	client.diagnosticsMu.Lock()
	client.diagnostics[diagParams.URI] = diagParams.Diagnostics

	// Calculate total diagnostic count
	totalCount := 0
	for _, diagnostics := range client.diagnostics {
		totalCount += len(diagnostics)
	}
	client.diagnosticsMu.Unlock()

	// Trigger callback if set
	if client.onDiagnosticsChanged != nil {
		client.onDiagnosticsChanged(client.name, totalCount)
	}
}
