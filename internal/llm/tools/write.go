package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwl/cliffy/internal/csync"
	"github.com/bwl/cliffy/internal/diff"
	"github.com/bwl/cliffy/internal/lsp"
)

//go:embed write.md
var writeDescription []byte

type WriteParams struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type WritePermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type writeTool struct {
	lspClients  *csync.Map[string, *lsp.Client]
	workingDir  string
}

type WriteResponseMetadata struct {
	Diff      string `json:"diff"`
	Additions int    `json:"additions"`
	Removals  int    `json:"removals"`
}

const WriteToolName = "write"

func NewWriteTool(lspClients *csync.Map[string, *lsp.Client], workingDir string) BaseTool {
	return &writeTool{
		lspClients: lspClients,
		workingDir: workingDir,
	}
}

func (w *writeTool) Name() string {
	return WriteToolName
}

func (w *writeTool) Info() ToolInfo {
	return ToolInfo{
		Name:        WriteToolName,
		Description: string(writeDescription),
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to write",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		Required: []string{"file_path", "content"},
	}
}

func (w *writeTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params WriteParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	if params.Content == "" {
		return NewTextErrorResponse("content is required"), nil
	}

	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(w.workingDir, filePath)
	}

	fileInfo, err := os.Stat(filePath)
	if err == nil {
		if fileInfo.IsDir() {
			return NewTextErrorResponse(fmt.Sprintf("Path is a directory, not a file: %s", filePath)), nil
		}

		modTime := fileInfo.ModTime()
		lastRead := getLastReadTime(filePath)
		if modTime.After(lastRead) {
			return NewTextErrorResponse(fmt.Sprintf("File %s has been modified since it was last read.\nLast modification: %s\nLast read: %s\n\nPlease read the file again before modifying it.",
				filePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339))), nil
		}

		oldContent, readErr := os.ReadFile(filePath)
		if readErr == nil && string(oldContent) == params.Content {
			return NewTextErrorResponse(fmt.Sprintf("File %s already contains the exact content. No changes made.", filePath)), nil
		}
	} else if !os.IsNotExist(err) {
		return ToolResponse{}, fmt.Errorf("error checking file: %w", err)
	}

	dir := filepath.Dir(filePath)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return ToolResponse{}, fmt.Errorf("error creating directory: %w", err)
	}

	oldContent := ""
	if fileInfo != nil && !fileInfo.IsDir() {
		oldBytes, readErr := os.ReadFile(filePath)
		if readErr == nil {
			oldContent = string(oldBytes)
		}
	}

	// In cliffy, all commands are auto-approved (user running CLI = implied consent)
	// No permission checks needed

	diff, additions, removals := diff.GenerateDiff(
		oldContent,
		params.Content,
		strings.TrimPrefix(filePath, w.workingDir),
	)

	err = os.WriteFile(filePath, []byte(params.Content), 0o644)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error writing file: %w", err)
	}

	recordFileWrite(filePath)
	recordFileRead(filePath)

	notifyLSPs(ctx, w.lspClients, params.FilePath)

	result := fmt.Sprintf("File successfully written: %s", filePath)
	result = fmt.Sprintf("<result>\n%s\n</result>", result)
	result += getDiagnostics(filePath, w.lspClients)
	return WithResponseMetadata(NewTextResponse(result),
		WriteResponseMetadata{
			Diff:      diff,
			Additions: additions,
			Removals:  removals,
		},
	), nil
}
