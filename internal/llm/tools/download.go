package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/permission"
)

type DownloadParams struct {
	URL      string `json:"url"`
	FilePath string `json:"file_path"`
	Timeout  int    `json:"timeout,omitempty"`
}

type DownloadPermissionsParams struct {
	URL      string `json:"url"`
	FilePath string `json:"file_path"`
	Timeout  int    `json:"timeout,omitempty"`
}

type downloadTool struct {
	client      *http.Client
	permissions permission.Service
	workingDir  string
}

const DownloadToolName = "download"

//go:embed download.md
var downloadDescription []byte

func NewDownloadTool(permissions permission.Service, workingDir string) BaseTool {
	return &downloadTool{
		client: &http.Client{
			Timeout: 5 * time.Minute, // Default 5 minute timeout for downloads
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (t *downloadTool) Name() string {
	return DownloadToolName
}

func (t *downloadTool) Info() ToolInfo {
	return ToolInfo{
		Name:        DownloadToolName,
		Description: string(downloadDescription),
		Parameters: map[string]any{
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to download from",
			},
			"file_path": map[string]any{
				"type":        "string",
				"description": "The local file path where the downloaded content should be saved",
			},
			"timeout": map[string]any{
				"type":        "number",
				"description": "Optional timeout in seconds (max 600)",
			},
		},
		Required: []string{"url", "file_path"},
	}
}

func (t *downloadTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params DownloadParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse("Failed to parse download parameters: " + err.Error()), nil
	}

	if params.URL == "" {
		return NewTextErrorResponse("URL parameter is required"), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path parameter is required"), nil
	}

	if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
		return NewTextErrorResponse("URL must start with http:// or https://"), nil
	}

	// Convert relative path to absolute path
	var filePath string
	if filepath.IsAbs(params.FilePath) {
		filePath = params.FilePath
	} else {
		filePath = filepath.Join(t.workingDir, params.FilePath)
	}

	sessionID, messageID := GetContextValues(ctx)
	if sessionID == "" || messageID == "" {
		return ToolResponse{}, fmt.Errorf("session ID and message ID are required for downloading files")
	}

	p := t.permissions.Request(
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
		return ToolResponse{}, permission.ErrorPermissionDenied
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
		return ToolResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "crush/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to download from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d", resp.StatusCode)), nil
	}

	// Check content length if available
	maxSize := int64(100 * 1024 * 1024) // 100MB
	if resp.ContentLength > maxSize {
		return NewTextErrorResponse(fmt.Sprintf("File too large: %d bytes (max %d bytes)", resp.ContentLength, maxSize)), nil
	}

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return ToolResponse{}, fmt.Errorf("failed to create parent directories: %w", err)
	}

	// Create the output file
	outFile, err := os.Create(filePath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Copy data with size limit
	limitedReader := io.LimitReader(resp.Body, maxSize)
	bytesWritten, err := io.Copy(outFile, limitedReader)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
	}

	// Check if we hit the size limit
	if bytesWritten == maxSize {
		// Clean up the file since it might be incomplete
		os.Remove(filePath)
		return NewTextErrorResponse(fmt.Sprintf("File too large: exceeded %d bytes limit", maxSize)), nil
	}

	contentType := resp.Header.Get("Content-Type")
	responseMsg := fmt.Sprintf("Successfully downloaded %d bytes to %s", bytesWritten, filePath)
	if contentType != "" {
		responseMsg += fmt.Sprintf(" (Content-Type: %s)", contentType)
	}

	return NewTextResponse(responseMsg), nil
}
