package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
)

type MultiEditOperation struct {
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type MultiEditParams struct {
	FilePath string               `json:"file_path"`
	Edits    []MultiEditOperation `json:"edits"`
}

type MultiEditPermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type MultiEditResponseMetadata struct {
	Additions    int    `json:"additions"`
	Removals     int    `json:"removals"`
	OldContent   string `json:"old_content,omitempty"`
	NewContent   string `json:"new_content,omitempty"`
	EditsApplied int    `json:"edits_applied"`
}

type multiEditTool struct {
	lspClients  *csync.Map[string, *lsp.Client]
	permissions permission.Service
	files       history.Service
	workingDir  string
}

const MultiEditToolName = "multiedit"

//go:embed multiedit.md
var multieditDescription []byte

func NewMultiEditTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, files history.Service, workingDir string) BaseTool {
	return &multiEditTool{
		lspClients:  lspClients,
		permissions: permissions,
		files:       files,
		workingDir:  workingDir,
	}
}

func (m *multiEditTool) Name() string {
	return MultiEditToolName
}

func (m *multiEditTool) Info() ToolInfo {
	return ToolInfo{
		Name:        MultiEditToolName,
		Description: string(multieditDescription),
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to modify",
			},
			"edits": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"old_string": map[string]any{
							"type":        "string",
							"description": "The text to replace",
						},
						"new_string": map[string]any{
							"type":        "string",
							"description": "The text to replace it with",
						},
						"replace_all": map[string]any{
							"type":        "boolean",
							"default":     false,
							"description": "Replace all occurrences of old_string (default false).",
						},
					},
					"required":             []string{"old_string", "new_string"},
					"additionalProperties": false,
				},
				"minItems":    1,
				"description": "Array of edit operations to perform sequentially on the file",
			},
		},
		Required: []string{"file_path", "edits"},
	}
}

func (m *multiEditTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params MultiEditParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	if len(params.Edits) == 0 {
		return NewTextErrorResponse("at least one edit operation is required"), nil
	}

	if !filepath.IsAbs(params.FilePath) {
		params.FilePath = filepath.Join(m.workingDir, params.FilePath)
	}

	// Validate all edits before applying any
	if err := m.validateEdits(params.Edits); err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}

	var response ToolResponse
	var err error

	// Handle file creation case (first edit has empty old_string)
	if len(params.Edits) > 0 && params.Edits[0].OldString == "" {
		response, err = m.processMultiEditWithCreation(ctx, params, call)
	} else {
		response, err = m.processMultiEditExistingFile(ctx, params, call)
	}

	if err != nil {
		return response, err
	}

	if response.IsError {
		return response, nil
	}

	// Notify LSP clients about the change
	notifyLSPs(ctx, m.lspClients, params.FilePath)

	// Wait for LSP diagnostics and add them to the response
	text := fmt.Sprintf("<result>\n%s\n</result>\n", response.Content)
	text += getDiagnostics(params.FilePath, m.lspClients)
	response.Content = text
	return response, nil
}

func (m *multiEditTool) validateEdits(edits []MultiEditOperation) error {
	for i, edit := range edits {
		if edit.OldString == edit.NewString {
			return fmt.Errorf("edit %d: old_string and new_string are identical", i+1)
		}
		// Only the first edit can have empty old_string (for file creation)
		if i > 0 && edit.OldString == "" {
			return fmt.Errorf("edit %d: only the first edit can have empty old_string (for file creation)", i+1)
		}
	}
	return nil
}

func (m *multiEditTool) processMultiEditWithCreation(ctx context.Context, params MultiEditParams, call ToolCall) (ToolResponse, error) {
	// First edit creates the file
	firstEdit := params.Edits[0]
	if firstEdit.OldString != "" {
		return NewTextErrorResponse("first edit must have empty old_string for file creation"), nil
	}

	// Check if file already exists
	if _, err := os.Stat(params.FilePath); err == nil {
		return NewTextErrorResponse(fmt.Sprintf("file already exists: %s", params.FilePath)), nil
	} else if !os.IsNotExist(err) {
		return ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
	}

	// Create parent directories
	dir := filepath.Dir(params.FilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ToolResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Start with the content from the first edit
	currentContent := firstEdit.NewString

	// Apply remaining edits to the content
	for i := 1; i < len(params.Edits); i++ {
		edit := params.Edits[i]
		newContent, err := m.applyEditToContent(currentContent, edit)
		if err != nil {
			return NewTextErrorResponse(fmt.Sprintf("edit %d failed: %s", i+1, err.Error())), nil
		}
		currentContent = newContent
	}

	// Get session and message IDs
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}

	// Check permissions
	_, additions, removals := diff.GenerateDiff("", currentContent, strings.TrimPrefix(params.FilePath, m.workingDir))

	p := m.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        fsext.PathOrPrefix(params.FilePath, m.workingDir),
		ToolCallID:  call.ID,
		ToolName:    MultiEditToolName,
		Action:      "write",
		Description: fmt.Sprintf("Create file %s with %d edits", params.FilePath, len(params.Edits)),
		Params: MultiEditPermissionsParams{
			FilePath:   params.FilePath,
			OldContent: "",
			NewContent: currentContent,
		},
	})
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	// Write the file
	err := os.WriteFile(params.FilePath, []byte(currentContent), 0o644)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}

	// Update file history
	_, err = m.files.Create(ctx, sessionID, params.FilePath, "")
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
	}

	_, err = m.files.CreateVersion(ctx, sessionID, params.FilePath, currentContent)
	if err != nil {
		slog.Debug("Error creating file history version", "error", err)
	}

	recordFileWrite(params.FilePath)
	recordFileRead(params.FilePath)

	return WithResponseMetadata(
		NewTextResponse(fmt.Sprintf("File created with %d edits: %s", len(params.Edits), params.FilePath)),
		MultiEditResponseMetadata{
			OldContent:   "",
			NewContent:   currentContent,
			Additions:    additions,
			Removals:     removals,
			EditsApplied: len(params.Edits),
		},
	), nil
}

func (m *multiEditTool) processMultiEditExistingFile(ctx context.Context, params MultiEditParams, call ToolCall) (ToolResponse, error) {
	// Validate file exists and is readable
	fileInfo, err := os.Stat(params.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewTextErrorResponse(fmt.Sprintf("file not found: %s", params.FilePath)), nil
		}
		return ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
	}

	if fileInfo.IsDir() {
		return NewTextErrorResponse(fmt.Sprintf("path is a directory, not a file: %s", params.FilePath)), nil
	}

	// Check if file was read before editing
	if getLastReadTime(params.FilePath).IsZero() {
		return NewTextErrorResponse("you must read the file before editing it. Use the View tool first"), nil
	}

	// Check if file was modified since last read
	modTime := fileInfo.ModTime()
	lastRead := getLastReadTime(params.FilePath)
	if modTime.After(lastRead) {
		return NewTextErrorResponse(
			fmt.Sprintf("file %s has been modified since it was last read (mod time: %s, last read: %s)",
				params.FilePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339),
			)), nil
	}

	// Read current file content
	content, err := os.ReadFile(params.FilePath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to read file: %w", err)
	}

	oldContent, isCrlf := fsext.ToUnixLineEndings(string(content))
	currentContent := oldContent

	// Apply all edits sequentially
	for i, edit := range params.Edits {
		newContent, err := m.applyEditToContent(currentContent, edit)
		if err != nil {
			return NewTextErrorResponse(fmt.Sprintf("edit %d failed: %s", i+1, err.Error())), nil
		}
		currentContent = newContent
	}

	// Check if content actually changed
	if oldContent == currentContent {
		return NewTextErrorResponse("no changes made - all edits resulted in identical content"), nil
	}

	// Get session and message IDs
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for editing file")
	}

	// Generate diff and check permissions
	_, additions, removals := diff.GenerateDiff(oldContent, currentContent, strings.TrimPrefix(params.FilePath, m.workingDir))
	p := m.permissions.Request(permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        fsext.PathOrPrefix(params.FilePath, m.workingDir),
		ToolCallID:  call.ID,
		ToolName:    MultiEditToolName,
		Action:      "write",
		Description: fmt.Sprintf("Apply %d edits to file %s", len(params.Edits), params.FilePath),
		Params: MultiEditPermissionsParams{
			FilePath:   params.FilePath,
			OldContent: oldContent,
			NewContent: currentContent,
		},
	})
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	if isCrlf {
		currentContent, _ = fsext.ToWindowsLineEndings(currentContent)
	}

	// Write the updated content
	err = os.WriteFile(params.FilePath, []byte(currentContent), 0o644)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}

	// Update file history
	file, err := m.files.GetByPathAndSession(ctx, params.FilePath, sessionID)
	if err != nil {
		_, err = m.files.Create(ctx, sessionID, params.FilePath, oldContent)
		if err != nil {
			return ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
		}
	}
	if file.Content != oldContent {
		// User manually changed the content, store an intermediate version
		_, err = m.files.CreateVersion(ctx, sessionID, params.FilePath, oldContent)
		if err != nil {
			slog.Debug("Error creating file history version", "error", err)
		}
	}

	// Store the new version
	_, err = m.files.CreateVersion(ctx, sessionID, params.FilePath, currentContent)
	if err != nil {
		slog.Debug("Error creating file history version", "error", err)
	}

	recordFileWrite(params.FilePath)
	recordFileRead(params.FilePath)

	return WithResponseMetadata(
		NewTextResponse(fmt.Sprintf("Applied %d edits to file: %s", len(params.Edits), params.FilePath)),
		MultiEditResponseMetadata{
			OldContent:   oldContent,
			NewContent:   currentContent,
			Additions:    additions,
			Removals:     removals,
			EditsApplied: len(params.Edits),
		},
	), nil
}

func (m *multiEditTool) applyEditToContent(content string, edit MultiEditOperation) (string, error) {
	if edit.OldString == "" && edit.NewString == "" {
		return content, nil
	}

	if edit.OldString == "" {
		return "", fmt.Errorf("old_string cannot be empty for content replacement")
	}

	var newContent string
	var replacementCount int

	if edit.ReplaceAll {
		newContent = strings.ReplaceAll(content, edit.OldString, edit.NewString)
		replacementCount = strings.Count(content, edit.OldString)
		if replacementCount == 0 {
			return "", fmt.Errorf("old_string not found in content. Make sure it matches exactly, including whitespace and line breaks")
		}
	} else {
		index := strings.Index(content, edit.OldString)
		if index == -1 {
			return "", fmt.Errorf("old_string not found in content. Make sure it matches exactly, including whitespace and line breaks")
		}

		lastIndex := strings.LastIndex(content, edit.OldString)
		if index != lastIndex {
			return "", fmt.Errorf("old_string appears multiple times in the content. Please provide more context to ensure a unique match, or set replace_all to true")
		}

		newContent = content[:index] + edit.NewString + content[index+len(edit.OldString):]
		replacementCount = 1
	}

	return newContent, nil
}
