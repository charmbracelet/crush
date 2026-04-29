package tools

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

			absWorkingDir, err := filepath.Abs(workingDir)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error resolving working directory: %w", err)
			}
			absFilePath, err := filepath.Abs(filePath)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error resolving file path: %w", err)
			}
			relPath, relErr := filepath.Rel(absWorkingDir, absFilePath)
			isOutsideWorkDir := relErr != nil || strings.HasPrefix(relPath, "..")

			if isOutsideWorkDir {
				granted, permReqErr := permissions.Request(ctx,
					permission.CreatePermissionRequest{
						SessionID:   sessionID,
						Path:        absFilePath,
						ToolCallID:  call.ID,
						ToolName:    TouchToolName,
						Action:      "write",
						Description: fmt.Sprintf("Create empty file outside working directory: %s", absFilePath),
						Params: TouchPermissionsParams{
							FilePath: absFilePath,
						},
					},
				)
				if permReqErr != nil {
					return fantasy.ToolResponse{}, permReqErr
				}
				if !granted {
					return NewPermissionDeniedResponse(), nil
				}
			}

			fileInfo, err := os.Stat(absFilePath)
			if err == nil {
				if fileInfo.IsDir() {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Path is a directory, not a file: %s", absFilePath)), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("File already exists: %s", absFilePath)), nil
			} else if !os.IsNotExist(err) {
				return fantasy.ToolResponse{}, fmt.Errorf("error checking file: %w", err)
			}

			dir := filepath.Dir(absFilePath)
			if err = os.MkdirAll(dir, 0o755); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating directory: %w", err)
			}

			p, err := permissions.Request(ctx,
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        fsext.PathOrPrefix(absFilePath, absWorkingDir),
					ToolCallID:  call.ID,
					ToolName:    TouchToolName,
					Action:      "write",
					Description: fmt.Sprintf("Create empty file %s", absFilePath),
					Params: TouchPermissionsParams{
						FilePath:   absFilePath,
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

			file, err := os.OpenFile(absFilePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				if os.IsExist(err) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("File already exists: %s", absFilePath)), nil
				}
				return fantasy.ToolResponse{}, fmt.Errorf("error creating file: %w", err)
			}
			if err = file.Close(); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error closing file: %w", err)
			}

			_, err = files.Create(ctx, sessionID, absFilePath, "")
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
			}

			filetracker.RecordRead(ctx, sessionID, absFilePath)

			notifyLSPs(ctx, lspManager, absFilePath)

			result := fmt.Sprintf("Empty file successfully created: %s", absFilePath)
			result = fmt.Sprintf("<result>\n%s\n</result>", result)
			result += getDiagnostics(absFilePath, lspManager)
			return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result),
				TouchResponseMetadata{
					FilePath: absFilePath,
				},
			), nil
		})
}
