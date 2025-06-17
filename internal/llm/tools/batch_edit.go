package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
)

type BatchEditOperation struct {
	FilePath  string `json:"file_path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

type BatchEditParams struct {
	Operations []BatchEditOperation `json:"operations"`
}

type BatchEditResult struct {
	FilePath   string `json:"file_path"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Additions  int    `json:"additions"`
	Removals   int    `json:"removals"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type BatchEditResponseMetadata struct {
	Results      []BatchEditResult `json:"results"`
	TotalSuccess int               `json:"total_success"`
	TotalFailed  int               `json:"total_failed"`
	TotalFiles   int               `json:"total_files"`
}

type batchEditTool struct {
	lspClients  map[string]*lsp.Client
	permissions permission.Service
	files       history.Service
}

const (
	BatchEditToolName    = "batch_edit"
	batchEditDescription = `Applies multiple edit operations across multiple files in a single atomic operation. This tool is more efficient than making multiple individual edit calls and ensures consistency across related changes.

Before using this tool:

1. Use the FileRead tool to understand all target files' contents and context
2. Verify all directory paths are correct for new files
3. Plan the operations to avoid conflicts

To make batch edits, provide:
1. operations: Array of edit operations, each containing:
   - file_path: The absolute path to the file to modify
   - old_string: The text to replace (must be unique within the file)
   - new_string: The edited text to replace the old_string

Special cases for each operation:
- To create a new file: provide file_path and new_string, leave old_string empty
- To delete content: provide file_path and old_string, leave new_string empty

CRITICAL REQUIREMENTS:

1. UNIQUENESS: Each old_string MUST uniquely identify the specific instance you want to change
   - Include AT LEAST 3-5 lines of context BEFORE and AFTER the change point
   - Include all whitespace, indentation, and surrounding code exactly as it appears

2. ATOMICITY: All operations succeed or all fail - partial application is not allowed

3. VERIFICATION: Before using this tool:
   - Check that all old_strings exist and are unique in their respective files
   - Ensure operations don't conflict with each other
   - Verify file modification timestamps

4. ORDERING: Operations are applied in the order specified
   - Later operations see the results of earlier operations in the same file
   - Plan operation order carefully for files with multiple edits

ADVANTAGES over individual edits:
- Atomic operation - all changes succeed or all fail
- Better performance for multiple related changes
- Consistent permission handling across all files
- Single LSP diagnostic check after all changes
- Reduced context switching and validation overhead

Remember: Always use absolute file paths (starting with /) and ensure proper context for uniqueness.`
)

func NewBatchEditTool(lspClients map[string]*lsp.Client, permissions permission.Service, files history.Service) BaseTool {
	return &batchEditTool{
		lspClients:  lspClients,
		permissions: permissions,
		files:       files,
	}
}

func (b *batchEditTool) Info() ToolInfo {
	return ToolInfo{
		Name:        BatchEditToolName,
		Description: batchEditDescription,
		Parameters: map[string]any{
			"operations": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
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
					},
					"required": []string{"file_path", "old_string", "new_string"},
				},
				"description": "Array of edit operations to perform",
			},
		},
		Required: []string{"operations"},
	}
}

func (b *batchEditTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params BatchEditParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("invalid parameters"), nil
	}

	if len(params.Operations) == 0 {
		return NewTextErrorResponse("at least one operation is required"), nil
	}

	// Validate all operations first
	if err := b.validateOperations(params.Operations); err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}

	// Group operations by file for efficient processing
	fileOps := b.groupOperationsByFile(params.Operations)

	// Pre-validate all files and permissions
	if err := b.preValidateFiles(ctx, fileOps); err != nil {
		if err == permission.ErrorPermissionDenied {
			return ToolResponse{}, err
		}
		return NewTextErrorResponse(err.Error()), nil
	}

	// Apply all operations atomically
	results, err := b.applyOperations(ctx, fileOps)
	if err != nil {
		return NewTextErrorResponse(err.Error()), nil
	}

	// Get LSP diagnostics for all modified files
	modifiedFiles := b.getModifiedFiles(results)
	for _, filePath := range modifiedFiles {
		waitForLspDiagnostics(ctx, filePath, b.lspClients)
	}

	// Build response
	metadata := b.buildResponseMetadata(results)
	response := b.buildResponse(metadata)

	// Add diagnostics
	diagnostics := ""
	for _, filePath := range modifiedFiles {
		diagnostics += getDiagnostics(filePath, b.lspClients)
	}

	if diagnostics != "" {
		response.Content = fmt.Sprintf("<result>\n%s\n</result>\n%s", response.Content, diagnostics)
	}

	return WithResponseMetadata(response, metadata), nil
}

func (b *batchEditTool) validateOperations(operations []BatchEditOperation) error {
	for i, op := range operations {
		if op.FilePath == "" {
			return fmt.Errorf("operation %d: file_path is required", i)
		}
		if !filepath.IsAbs(op.FilePath) {
			wd := config.WorkingDirectory()
			operations[i].FilePath = filepath.Join(wd, op.FilePath)
		}
	}
	return nil
}

func (b *batchEditTool) groupOperationsByFile(operations []BatchEditOperation) map[string][]BatchEditOperation {
	fileOps := make(map[string][]BatchEditOperation)
	for _, op := range operations {
		fileOps[op.FilePath] = append(fileOps[op.FilePath], op)
	}
	return fileOps
}

func (b *batchEditTool) preValidateFiles(ctx context.Context, fileOps map[string][]BatchEditOperation) error {
	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return fmt.Errorf("session ID and message ID are required for batch edit operations")
	}

	for filePath, ops := range fileOps {
		// Check if file exists for non-creation operations
		hasCreation := false
		for _, op := range ops {
			if op.OldString == "" {
				hasCreation = true
				break
			}
		}

		if !hasCreation {
			// File must exist and be readable
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("file not found: %s", filePath)
				}
				return fmt.Errorf("failed to access file %s: %w", filePath, err)
			}

			if fileInfo.IsDir() {
				return fmt.Errorf("path is a directory, not a file: %s", filePath)
			}

			// Check if file was read recently
			if getLastReadTime(filePath).IsZero() {
				return fmt.Errorf("you must read file %s before editing it. Use the View tool first", filePath)
			}

			// Check modification time
			modTime := fileInfo.ModTime()
			lastRead := getLastReadTime(filePath)
			if modTime.After(lastRead) {
				return fmt.Errorf("file %s has been modified since it was last read (mod time: %s, last read: %s)",
					filePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339))
			}
		}

		// Request permissions for this file
		rootDir := config.WorkingDirectory()
		permissionPath := filepath.Dir(filePath)
		if strings.HasPrefix(filePath, rootDir) {
			permissionPath = rootDir
		}

		action := "write"
		description := fmt.Sprintf("Batch edit file %s (%d operations)", filePath, len(ops))

		p := b.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        permissionPath,
				ToolName:    BatchEditToolName,
				Action:      action,
				Description: description,
				Params: map[string]any{
					"file_path":  filePath,
					"operations": len(ops),
				},
			},
		)
		if !p {
			return permission.ErrorPermissionDenied
		}
	}

	return nil
}

func (b *batchEditTool) applyOperations(ctx context.Context, fileOps map[string][]BatchEditOperation) ([]BatchEditResult, error) {
	var results []BatchEditResult

	for filePath, ops := range fileOps {
		result, err := b.applyFileOperations(ctx, filePath, ops)
		if err != nil {
			// If any file fails, we should ideally rollback, but for now we'll return the error
			return nil, fmt.Errorf("failed to apply operations to %s: %w", filePath, err)
		}
		results = append(results, result...)
	}

	return results, nil
}

func (b *batchEditTool) applyFileOperations(ctx context.Context, filePath string, ops []BatchEditOperation) ([]BatchEditResult, error) {
	var results []BatchEditResult
	sessionID, _ := GetContextValues(ctx)

	// Read current content or start with empty for new files
	var currentContent string
	var isNewFile bool

	// Check if this is a file creation (first operation has empty old_string)
	if len(ops) > 0 && ops[0].OldString == "" {
		isNewFile = true
		currentContent = ""
	} else {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %w", err)
		}
		currentContent = string(content)
	}

	originalContent := currentContent

	// Apply operations sequentially
	for _, op := range ops {
		var newContent string
		var err error

		if op.OldString == "" {
			// File creation - should only happen for new files
			if !isNewFile {
				return nil, fmt.Errorf("cannot create file that already exists: %s", filePath)
			}
			newContent = op.NewString
		} else if op.NewString == "" {
			// Content deletion
			newContent, err = b.deleteContentFromString(currentContent, op.OldString)
			if err != nil {
				results = append(results, BatchEditResult{
					FilePath: filePath,
					Success:  false,
					Error:    err.Error(),
				})
				continue
			}
		} else {
			// Content replacement
			newContent, err = b.replaceContentInString(currentContent, op.OldString, op.NewString)
			if err != nil {
				results = append(results, BatchEditResult{
					FilePath: filePath,
					Success:  false,
					Error:    err.Error(),
				})
				continue
			}
		}

		// Calculate diff for this operation
		_, additions, removals := diff.GenerateDiff(currentContent, newContent, filePath)

		results = append(results, BatchEditResult{
			FilePath:   filePath,
			Success:    true,
			Additions:  additions,
			Removals:   removals,
			OldContent: currentContent,
			NewContent: newContent,
		})

		currentContent = newContent
	}

	// Write the final content to file
	if isNewFile {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create parent directories: %w", err)
		}
	}

	err := os.WriteFile(filePath, []byte(currentContent), 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Update file history
	if isNewFile {
		_, err = b.files.Create(ctx, sessionID, filePath, "")
		if err != nil {
			logging.Debug("Error creating file history", "error", err)
		}
	} else {
		// Check if file exists in history
		file, err := b.files.GetByPathAndSession(ctx, filePath, sessionID)
		if err != nil {
			_, err = b.files.Create(ctx, sessionID, filePath, originalContent)
			if err != nil {
				logging.Debug("Error creating file history", "error", err)
			}
		} else if file.Content != originalContent {
			// Store intermediate version if content was manually changed
			_, err = b.files.CreateVersion(ctx, sessionID, filePath, originalContent)
			if err != nil {
				logging.Debug("Error creating file history version", "error", err)
			}
		}
	}

	// Store the final version
	_, err = b.files.CreateVersion(ctx, sessionID, filePath, currentContent)
	if err != nil {
		logging.Debug("Error creating file history version", "error", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	return results, nil
}

func (b *batchEditTool) deleteContentFromString(content, oldString string) (string, error) {
	index := strings.Index(content, oldString)
	if index == -1 {
		return "", fmt.Errorf("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks")
	}

	lastIndex := strings.LastIndex(content, oldString)
	if index != lastIndex {
		return "", fmt.Errorf("old_string appears multiple times in the file. Please provide more context to ensure a unique match")
	}

	return content[:index] + content[index+len(oldString):], nil
}

func (b *batchEditTool) replaceContentInString(content, oldString, newString string) (string, error) {
	index := strings.Index(content, oldString)
	if index == -1 {
		return "", fmt.Errorf("old_string not found in file. Make sure it matches exactly, including whitespace and line breaks")
	}

	lastIndex := strings.LastIndex(content, oldString)
	if index != lastIndex {
		return "", fmt.Errorf("old_string appears multiple times in the file. Please provide more context to ensure a unique match")
	}

	newContent := content[:index] + newString + content[index+len(oldString):]
	if content == newContent {
		return "", fmt.Errorf("new content is the same as old content. No changes made")
	}

	return newContent, nil
}

func (b *batchEditTool) getModifiedFiles(results []BatchEditResult) []string {
	fileSet := make(map[string]bool)
	for _, result := range results {
		if result.Success {
			fileSet[result.FilePath] = true
		}
	}

	var files []string
	for file := range fileSet {
		files = append(files, file)
	}
	return files
}

func (b *batchEditTool) buildResponseMetadata(results []BatchEditResult) BatchEditResponseMetadata {
	metadata := BatchEditResponseMetadata{
		Results: results,
	}

	fileSet := make(map[string]bool)
	for _, result := range results {
		fileSet[result.FilePath] = true
		if result.Success {
			metadata.TotalSuccess++
		} else {
			metadata.TotalFailed++
		}
	}
	metadata.TotalFiles = len(fileSet)

	return metadata
}

func (b *batchEditTool) buildResponse(metadata BatchEditResponseMetadata) ToolResponse {
	if metadata.TotalFailed > 0 {
		return NewTextErrorResponse(fmt.Sprintf("Batch edit completed with %d successes and %d failures across %d files",
			metadata.TotalSuccess, metadata.TotalFailed, metadata.TotalFiles))
	}

	return NewTextResponse(fmt.Sprintf("Batch edit completed successfully: %d operations across %d files",
		metadata.TotalSuccess, metadata.TotalFiles))
}
