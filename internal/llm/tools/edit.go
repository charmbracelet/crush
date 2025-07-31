package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
)

type EditParams struct {
	FilePath   string `json:"file_path"`
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all,omitempty"`
}

type EditPermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type EditResponseMetadata struct {
	Additions  int    `json:"additions"`
	Removals   int    `json:"removals"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type editTool struct {
	lspClients  map[string]*lsp.Client
	permissions permission.Service
	files       history.Service
	workingDir  string
}

const (
	EditToolName    = "edit"
	editDescription = `Edits files by replacing text, creating new files, or deleting content. For moving or renaming files, use the Bash tool with the 'mv' command instead. For larger file edits, use the FileWrite tool to overwrite files.

Before using this tool:

1. Use the FileRead tool to understand the file's contents and context

2. Verify the directory path is correct (only applicable when creating new files):
   - Use the LS tool to verify the parent directory exists and is the correct location

To make a file edit, provide the following:
1. file_path: The absolute path to the file to modify (must be absolute, not relative)
2. old_string: The text to replace (must be unique within the file, and must match the file contents exactly, including all whitespace and indentation)
3. new_string: The edited text to replace the old_string
4. replace_all: Replace all occurrences of old_string (default false)

Special cases:
- To create a new file: provide file_path and new_string, leave old_string empty
- To delete content: provide file_path and old_string, leave new_string empty

The tool will replace ONE occurrence of old_string with new_string in the specified file by default. Set replace_all to true to replace all occurrences.

CRITICAL REQUIREMENTS FOR USING THIS TOOL:

1. UNIQUENESS: When replace_all is false (default), the old_string MUST uniquely identify the specific instance you want to change. This means:
   - Include AT LEAST 3-5 lines of context BEFORE the change point
   - Include AT LEAST 3-5 lines of context AFTER the change point
   - Include all whitespace, indentation, and surrounding code exactly as it appears in the file

2. SINGLE INSTANCE: When replace_all is false, this tool can only change ONE instance at a time. If you need to change multiple instances:
   - Set replace_all to true to replace all occurrences at once
   - Or make separate calls to this tool for each instance
   - Each call must uniquely identify its specific instance using extensive context

3. VERIFICATION: Before using this tool:
   - Check how many instances of the target text exist in the file
   - If multiple instances exist and replace_all is false, gather enough context to uniquely identify each one
   - Plan separate tool calls for each instance or use replace_all

WARNING: If you do not follow these requirements:
   - The tool will fail if old_string matches multiple locations and replace_all is false
   - The tool will fail if old_string doesn't match exactly (including whitespace)
   - You may change the wrong instance if you don't include enough context

When making edits:
   - Ensure the edit results in idiomatic, correct code
   - Do not leave the code in a broken state
   - Always use absolute file paths (starting with /)

WINDOWS NOTES:
- File paths should use forward slashes (/) for cross-platform compatibility
- On Windows, absolute paths start with drive letters (C:/) but forward slashes work throughout
- File permissions are handled automatically by the Go runtime

Remember: when making multiple file edits in a row to the same file, you should prefer to send all edits in a single message with multiple calls to this tool, rather than multiple messages with a single call each.`
)

func NewEditTool(lspClients map[string]*lsp.Client, permissions permission.Service, files history.Service, workingDir string) BaseTool {
	return &editTool{
		lspClients:  lspClients,
		permissions: permissions,
		files:       files,
		workingDir:  workingDir,
	}
}

func (e *editTool) Name() string {
	return EditToolName
}

func (e *editTool) Info() ToolInfo {
	return ToolInfo{
		Name:        EditToolName,
		Description: editDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to modify",
			},
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
				"description": "Replace all occurrences of old_string (default false)",
			},
		},
		Required: []string{"file_path", "old_string", "new_string"},
	}
}

func (e *editTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params EditParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	if !filepath.IsAbs(params.FilePath) {
		params.FilePath = filepath.Join(e.workingDir, params.FilePath)
	}

	var response ToolResponse
	var err error

	if params.OldString == "" {
		response, err = e.createNewFile(ctx, params.FilePath, params.NewString, call)
		if err != nil {
			return response, err
		}
	}

	if params.NewString == "" {
		response, err = e.deleteContent(ctx, params.FilePath, params.OldString, params.ReplaceAll, call)
		if err != nil {
			return response, err
		}
	}

	response, err = e.replaceContent(ctx, params.FilePath, params.OldString, params.NewString, params.ReplaceAll, call)
	if err != nil {
		return response, err
	}
	if response.IsError {
		// Return early if there was an error during content replacement
		// This prevents unnecessary LSP diagnostics processing
		return response, nil
	}

	waitForLspDiagnostics(ctx, params.FilePath, e.lspClients)
	text := fmt.Sprintf("<result>\n%s\n</result>\n", response.Content)
	text += getDiagnostics(params.FilePath, e.lspClients)
	response.Content = text
	return response, nil
}

func (e *editTool) createNewFile(ctx context.Context, filePath, content string, call ToolCall) (ToolResponse, error) {
	fileInfo, err := os.Stat(filePath)
	if err == nil {
		if fileInfo.IsDir() {
			return NewTextErrorResponse(fmt.Sprintf("path is a directory, not a file: %s", filePath)), nil
		}
		return NewTextErrorResponse(fmt.Sprintf("file already exists: %s", filePath)), nil
	} else if !os.IsNotExist(err) {
		return ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
	}

	dir := filepath.Dir(filePath)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return ToolResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}

	_, additions, removals := diff.GenerateDiff(
		"",
		content,
		strings.TrimPrefix(filePath, e.workingDir),
	)
	p := e.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        fsext.PathOrPrefix(filePath, e.workingDir),
			ToolCallID:  call.ID,
			ToolName:    EditToolName,
			Action:      "write",
			Description: fmt.Sprintf("Create file %s", filePath),
			Params: EditPermissionsParams{
				FilePath:   filePath,
				OldContent: "",
				NewContent: content,
			},
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	err = os.WriteFile(filePath, []byte(content), 0o644)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}

	// File can't be in the history so we create a new file history
	_, err = e.files.Create(ctx, sessionID, filePath, "")
	if err != nil {
		// Log error but don't fail the operation
		return ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
	}

	// Add the new content to the file history
	_, err = e.files.CreateVersion(ctx, sessionID, filePath, content)
	if err != nil {
		// Log error but don't fail the operation
		slog.Debug("Error creating file history version", "error", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	return WithResponseMetadata(
		NewTextResponse("File created: "+filePath),
		EditResponseMetadata{
			OldContent: "",
			NewContent: content,
			Additions:  additions,
			Removals:   removals,
		},
	), nil
}

func (e *editTool) deleteContent(ctx context.Context, filePath, oldString string, replaceAll bool, call ToolCall) (ToolResponse, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewTextErrorResponse(fmt.Sprintf("file not found: %s", filePath)), nil
		}
		return ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
	}

	if fileInfo.IsDir() {
		return NewTextErrorResponse(fmt.Sprintf("path is a directory, not a file: %s", filePath)), nil
	}

	if getLastReadTime(filePath).IsZero() {
		return NewTextErrorResponse("you must read the file before editing it. Use the View tool first"), nil
	}

	modTime := fileInfo.ModTime()
	lastRead := getLastReadTime(filePath)
	if modTime.After(lastRead) {
		return NewTextErrorResponse(
			fmt.Sprintf("file %s has been modified since it was last read (mod time: %s, last read: %s)",
				filePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339),
			)), nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to read file: %w", err)
	}

	oldContent := string(content)

	var newContent string
	var deletionCount int

	if replaceAll {
		newContent = strings.ReplaceAll(oldContent, oldString, "")
		deletionCount = strings.Count(oldContent, oldString)
		if deletionCount == 0 {
			return NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}
	} else {
		index := strings.Index(oldContent, oldString)
		if index == -1 {
			return NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}

		lastIndex := strings.LastIndex(oldContent, oldString)
		if index != lastIndex {
			return NewTextErrorResponse("old_string appears multiple times in the file. Please provide more context to ensure a unique match, or set replace_all to true"), nil
		}

		newContent = oldContent[:index] + oldContent[index+len(oldString):]
		deletionCount = 1
	}

	sessionID, messageID := GetContextValues(ctx)

	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}

	_, additions, removals := diff.GenerateDiff(
		oldContent,
		newContent,
		strings.TrimPrefix(filePath, e.workingDir),
	)

	p := e.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        fsext.PathOrPrefix(filePath, e.workingDir),
			ToolCallID:  call.ID,
			ToolName:    EditToolName,
			Action:      "write",
			Description: fmt.Sprintf("Delete content from file %s", filePath),
			Params: EditPermissionsParams{
				FilePath:   filePath,
				OldContent: oldContent,
				NewContent: newContent,
			},
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	err = os.WriteFile(filePath, []byte(newContent), 0o644)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}

	// Check if file exists in history
	file, err := e.files.GetByPathAndSession(ctx, filePath, sessionID)
	if err != nil {
		_, err = e.files.Create(ctx, sessionID, filePath, oldContent)
		if err != nil {
			// Log error but don't fail the operation
			return ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
		}
	}
	if file.Content != oldContent {
		// User Manually changed the content store an intermediate version
		_, err = e.files.CreateVersion(ctx, sessionID, filePath, oldContent)
		if err != nil {
			slog.Debug("Error creating file history version", "error", err)
		}
	}
	// Store the new version
	_, err = e.files.CreateVersion(ctx, sessionID, filePath, "")
	if err != nil {
		slog.Debug("Error creating file history version", "error", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	return WithResponseMetadata(
		NewTextResponse("Content deleted from file: "+filePath),
		EditResponseMetadata{
			OldContent: oldContent,
			NewContent: newContent,
			Additions:  additions,
			Removals:   removals,
		},
	), nil
}

func (e *editTool) replaceContent(ctx context.Context, filePath, oldString, newString string, replaceAll bool, call ToolCall) (ToolResponse, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewTextErrorResponse(fmt.Sprintf("file not found: %s", filePath)), nil
		}
		return ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
	}

	if fileInfo.IsDir() {
		return NewTextErrorResponse(fmt.Sprintf("path is a directory, not a file: %s", filePath)), nil
	}

	if getLastReadTime(filePath).IsZero() {
		return NewTextErrorResponse("you must read the file before editing it. Use the View tool first"), nil
	}

	modTime := fileInfo.ModTime()
	lastRead := getLastReadTime(filePath)
	if modTime.After(lastRead) {
		return NewTextErrorResponse(
			fmt.Sprintf("file %s has been modified since it was last read (mod time: %s, last read: %s)",
				filePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339),
			)), nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to read file: %w", err)
	}

	oldContent := string(content)

	var newContent string
	var replacementCount int

	if replaceAll {
		newContent = strings.ReplaceAll(oldContent, oldString, newString)
		replacementCount = strings.Count(oldContent, oldString)
		if replacementCount == 0 {
			return NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}
	} else {
		index := strings.Index(oldContent, oldString)
		if index == -1 {
			return NewTextErrorResponse("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks"), nil
		}

		lastIndex := strings.LastIndex(oldContent, oldString)
		if index != lastIndex {
			return NewTextErrorResponse("old_string appears multiple times in the file. Please provide more context to ensure a unique match, or set replace_all to true"), nil
		}

		newContent = oldContent[:index] + newString + oldContent[index+len(oldString):]
		replacementCount = 1
	}

	if oldContent == newContent {
		return NewTextErrorResponse("new content is the same as old content. No changes made."), nil
	}
	sessionID, messageID := GetContextValues(ctx)

	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for creating a new file")
	}
	_, additions, removals := diff.GenerateDiff(
		oldContent,
		newContent,
		strings.TrimPrefix(filePath, e.workingDir),
	)

	p := e.permissions.Request(
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        fsext.PathOrPrefix(filePath, e.workingDir),
			ToolCallID:  call.ID,
			ToolName:    EditToolName,
			Action:      "write",
			Description: fmt.Sprintf("Replace content in file %s", filePath),
			Params: EditPermissionsParams{
				FilePath:   filePath,
				OldContent: oldContent,
				NewContent: newContent,
			},
		},
	)
	if !p {
		return ToolResponse{}, permission.ErrorPermissionDenied
	}

	err = os.WriteFile(filePath, []byte(newContent), 0o644)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}

	// Check if file exists in history
	file, err := e.files.GetByPathAndSession(ctx, filePath, sessionID)
	if err != nil {
		_, err = e.files.Create(ctx, sessionID, filePath, oldContent)
		if err != nil {
			// Log error but don't fail the operation
			return ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
		}
	}
	if file.Content != oldContent {
		// User Manually changed the content store an intermediate version
		_, err = e.files.CreateVersion(ctx, sessionID, filePath, oldContent)
		if err != nil {
			slog.Debug("Error creating file history version", "error", err)
		}
	}
	// Store the new version
	_, err = e.files.CreateVersion(ctx, sessionID, filePath, newContent)
	if err != nil {
		slog.Debug("Error creating file history version", "error", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	return WithResponseMetadata(
		NewTextResponse("Content replaced in file: "+filePath),
		EditResponseMetadata{
			OldContent: oldContent,
			NewContent: newContent,
			Additions:  additions,
			Removals:   removals,
		}), nil
}
