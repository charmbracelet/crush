package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/permission"
)

type DownloadParams struct {
	URL      string `json:"url" description:"The URL to download from"`
	FilePath string `json:"file_path" description:"The local file path where the downloaded content should be saved"`
	Timeout  int    `json:"timeout,omitempty" description:"Optional timeout in seconds (max 600)"`
}

type DownloadPermissionsParams struct {
	URL      string `json:"url"`
	FilePath string `json:"file_path"`
	Timeout  int    `json:"timeout,omitempty"`
}

const DownloadToolName = "download"

//go:embed download.md
var downloadDescription []byte

func NewDownloadTool(permissions permission.Service, workingDir string, client *http.Client) fantasy.AgentTool {
	if client == nil {
		client = &http.Client{
			Timeout: 5 * time.Minute, // Default 5 minute timeout for downloads
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}
	return fantasy.NewAgentTool(
		DownloadToolName,
		string(downloadDescription),
		func(ctx context.Context, params DownloadParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.URL == "" {
				return fantasy.NewTextErrorResponse("URL parameter is required"), nil
			}

			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path parameter is required"), nil
			}

			if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
				return fantasy.NewTextErrorResponse("URL must start with http:// or https://"), nil
			}

			filePath := filepathext.SmartJoin(workingDir, params.FilePath)
			relPath, _ := filepath.Rel(workingDir, filePath)
			relPath = filepath.ToSlash(cmp.Or(relPath, filePath))

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for downloading files")
			}

			p := permissions.Request(
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        filePath,
					ToolName:    DownloadToolName,
					Action:      "download",
					Description: fmt.Sprintf("Download file from URL: %s to %s", params.URL, filePath),
					Params:      DownloadPermissionsParams(params),
				},
			)

			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			// Handle timeout with context
			requestCtx := ctx
			if params.Timeout > 0 {
				maxTimeout := 600 // 10 minutes
				if params.Timeout > maxTimeout {
					params.Timeout = maxTimeout
				}
				var cancel context.CancelFunc
				requestCtx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
				defer cancel()
			}

			req, err := http.NewRequestWithContext(requestCtx, "GET", params.URL, nil)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("User-Agent", "crush/1.0")

			resp, err := client.Do(req)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to download from URL: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d", resp.StatusCode)), nil
			}

			// Check content length if available
			maxSize := int64(100 * 1024 * 1024) // 100MB
			if resp.ContentLength > maxSize {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("File too large: %d bytes (max %d bytes)", resp.ContentLength, maxSize)), nil
			}

			// Create parent directories if they don't exist
			if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
			}

			// Create the output file
			outFile, err := os.Create(filePath)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create output file: %w", err)
			}
			defer outFile.Close()

			// Copy data with size limit
			limitedReader := io.LimitReader(resp.Body, maxSize)
			bytesWritten, err := io.Copy(outFile, limitedReader)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
			}

			// Check if we hit the size limit
			if bytesWritten == maxSize {
				// Clean up the file since it might be incomplete
				os.Remove(filePath)
				return fantasy.NewTextErrorResponse(fmt.Sprintf("File too large: exceeded %d bytes limit", maxSize)), nil
			}

			contentType := resp.Header.Get("Content-Type")
			responseMsg := fmt.Sprintf("Successfully downloaded %d bytes to %s", bytesWritten, relPath)
			if contentType != "" {
				responseMsg += fmt.Sprintf(" (Content-Type: %s)", contentType)
			}

			return fantasy.NewTextResponse(responseMsg), nil
		})
}
