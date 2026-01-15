package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

//go:embed delete.md
var deleteDescription []byte

// DeleteParams defines the parameters for the delete tool.
type DeleteParams struct {
	FilePath  string `json:"file_path" description:"The path to the file or directory to delete"`
	Recursive bool   `json:"recursive,omitempty" description:"If true, recursively delete directory contents (default false)"`
}

// DeletePermissionsParams defines the parameters shown in permission requests.
type DeletePermissionsParams struct {
	FilePath  string `json:"file_path"`
	Recursive bool   `json:"recursive,omitempty"`
	IsDir     bool   `json:"is_dir,omitempty"`
}

// DeleteToolName is the name of the delete tool.
const DeleteToolName = "delete"

// NewDeleteTool creates a new delete tool.
func NewDeleteTool(lspClients *csync.Map[string, *lsp.Client], permissions permission.Service, files history.Service, workingDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		DeleteToolName,
		string(deleteDescription),
		func(ctx context.Context, params DeleteParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}

			filePath := filepathext.SmartJoin(workingDir, params.FilePath)

			fileInfo, err := os.Stat(filePath)
			if os.IsNotExist(err) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Path does not exist: %s", filePath)), nil
			}
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error checking path: %w", err)
			}

			isDir := fileInfo.IsDir()

			if isDir && !params.Recursive {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Cannot delete directory %s. Set recursive=true to delete directory and its contents.", filePath)), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session_id is required")
			}

			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        fsext.PathOrPrefix(filePath, workingDir),
					ToolCallID:  call.ID,
					ToolName:    DeleteToolName,
					Action:      "delete",
					Description: buildDeleteDescription(filePath, isDir),
					Params: DeletePermissionsParams{
						FilePath:  filePath,
						Recursive: params.Recursive,
						IsDir:     isDir,
					},
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			// Save file content to history BEFORE deleting.
			saveDeletedFileHistory(ctx, files, sessionID, filePath, isDir)

			if err := os.RemoveAll(filePath); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error deleting path: %w", err)
			}

			lspCloseAndDeleteFiles(ctx, lspClients, filePath, isDir)
			return fantasy.NewTextResponse(fmt.Sprintf("Successfully deleted: %s", filePath)), nil
		})
}

func buildDeleteDescription(filePath string, isDir bool) string {
	if !isDir {
		return fmt.Sprintf("Delete file %s", filePath)
	}
	return fmt.Sprintf("Delete directory %s and all its contents", filePath)
}

// shouldDeletePath checks if a path matches the deletion target.
// For files, it matches exact paths. For directories, it matches the directory
// and all paths within it.
func shouldDeletePath(path, targetPath string, isDir bool) bool {
	cleanPath := filepath.Clean(path)
	cleanTarget := filepath.Clean(targetPath)

	if cleanPath == cleanTarget {
		return true
	}

	return isDir && strings.HasPrefix(cleanPath, cleanTarget+string(filepath.Separator))
}

func lspCloseAndDeleteFiles(ctx context.Context, lsps *csync.Map[string, *lsp.Client], filePath string, isDir bool) {
	for client := range lsps.Seq() {
		for uri := range client.OpenFiles() {
			path, err := protocol.DocumentURI(uri).Path()
			if err != nil {
				continue
			}
			if !shouldDeletePath(path, filePath, isDir) {
				continue
			}
			_ = client.DeleteFile(ctx, path)
		}
	}
}

func saveDeletedFileHistory(ctx context.Context, files history.Service, sessionID, filePath string, isDir bool) {
	if isDir {
		// For directories, walk through and save all files.
		_ = filepath.Walk(filePath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			saveFileBeforeDeletion(ctx, files, sessionID, path)
			return nil
		})
	} else {
		// For single file.
		saveFileBeforeDeletion(ctx, files, sessionID, filePath)
	}
}

func saveFileBeforeDeletion(ctx context.Context, files history.Service, sessionID, filePath string) {
	// Check if file already exists in history.
	existing, err := files.GetByPathAndSession(ctx, filePath, sessionID)
	if err == nil && existing.Path != "" {
		// File exists in history, create empty version to show deletion.
		_, _ = files.CreateVersion(ctx, sessionID, filePath, "")
		return
	}

	// File not in history, read content and create initial + empty version.
	content, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	// Create initial version with current content.
	_, err = files.Create(ctx, sessionID, filePath, string(content))
	if err != nil {
		return
	}

	// Create empty version to show deletion.
	_, _ = files.CreateVersion(ctx, sessionID, filePath, "")
}
