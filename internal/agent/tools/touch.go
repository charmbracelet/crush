package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
)

//go:embed touch.md
var touchDescription []byte

type TouchParams struct {
	FilePath string `json:"file_path" description:"The path to the empty file to create"`
}

type TouchPermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type TouchResponseMetadata struct {
	FilePath string `json:"file_path"`
}

const TouchToolName = "touch"

func NewTouchTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	files history.Service,
	filetracker filetracker.Service,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TouchToolName,
		FirstLineDescription(touchDescription),
		func(ctx context.Context, params TouchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session_id is required")
			}

			filePath := filepathext.SmartJoin(workingDir, params.FilePath)

			fileInfo, err := os.Stat(filePath)
			if err == nil {
				if fileInfo.IsDir() {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Path is a directory, not a file: %s", filePath)), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("File already exists: %s", filePath)), nil
			} else if !os.IsNotExist(err) {
				return fantasy.ToolResponse{}, fmt.Errorf("error checking file: %w", err)
			}

			dir := filepath.Dir(filePath)
			if err = os.MkdirAll(dir, 0o755); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating directory: %w", err)
			}

			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        fsext.PathOrPrefix(filePath, workingDir),
					ToolCallID:  call.ID,
					ToolName:    TouchToolName,
					Action:      "write",
					Description: fmt.Sprintf("Create empty file %s", filePath),
					Params: TouchPermissionsParams{
						FilePath:   filePath,
						OldContent: "",
						NewContent: "",
					},
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return NewPermissionDeniedResponse(), nil
			}

			file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				if os.IsExist(err) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("File already exists: %s", filePath)), nil
				}
				return fantasy.ToolResponse{}, fmt.Errorf("error creating file: %w", err)
			}
			if err = file.Close(); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error closing file: %w", err)
			}

			_, err = files.Create(ctx, sessionID, filePath, "")
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
			}

			filetracker.RecordRead(ctx, sessionID, filePath)

			notifyLSPs(ctx, lspManager, filePath)

			result := fmt.Sprintf("Empty file successfully created: %s", filePath)
			result = fmt.Sprintf("<result>\n%s\n</result>", result)
			result += getDiagnostics(filePath, lspManager)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result),
				TouchResponseMetadata{
					FilePath: filePath,
				},
			), nil
		})
}
