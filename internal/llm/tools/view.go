package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
)

type ViewParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type ViewPermissionsParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

type viewTool struct {
	lspClients  map[string]*lsp.Client
	workingDir  string
	permissions permission.Service
}

type ViewResponseMetadata struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

const (
	ViewToolName     = "view"
	MaxReadSize      = 250 * 1024
	DefaultReadLimit = 2000
	MaxLineLength    = 2000
	viewDescription  = `File viewing tool that reads and displays the contents of files with line numbers, allowing you to examine code, logs, or text data.

WHEN TO USE THIS TOOL:
- Use when you need to read the contents of a specific file
- Helpful for examining source code, configuration files, or log files
- Perfect for looking at text-based file formats

HOW TO USE:
- Provide the path to the file you want to view
- Optionally specify an offset to start reading from a specific line
- Optionally specify a limit to control how many lines are read
- Do not use this for directories use the ls tool instead

FEATURES:
- Displays file contents with line numbers for easy reference
- Can read from any position in a file using the offset parameter
- Handles large files by limiting the number of lines read
- Automatically truncates very long lines for better display
- Suggests similar file names when the requested file isn't found

LIMITATIONS:
- Maximum file size is 250KB
- Default reading limit is 2000 lines
- Lines longer than 2000 characters are truncated
- Cannot display binary files or images
- Images can be identified but not displayed

WINDOWS NOTES:
- Handles both Windows (CRLF) and Unix (LF) line endings automatically
- File paths work with both forward slashes (/) and backslashes (\)
- Text encoding is detected automatically for most common formats

TIPS:
- Use with Glob tool to first find files you want to view
- For code exploration, first use Grep to find relevant files, then View to examine them
- When viewing large files, use the offset parameter to read specific sections`
)

func NewViewTool(lspClients map[string]*lsp.Client, permissions permission.Service, workingDir string) BaseTool {
	return &viewTool{
		lspClients:  lspClients,
		workingDir:  workingDir,
		permissions: permissions,
	}
}

func (v *viewTool) Name() string {
	return ViewToolName
}

func (v *viewTool) Info() ToolInfo {
	return ToolInfo{
		Name:        ViewToolName,
		Description: viewDescription,
		Parameters: map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "The line number to start reading from (0-based)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "The number of lines to read (defaults to 2000)",
			},
		},
		Required: []string{"file_path"},
	}
}

// Run implements Tool.
func (v *viewTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params ViewParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.FilePath == "" {
		return NewTextErrorResponse("file_path is required"), nil
	}

	// Handle relative paths
	filePath := params.FilePath
	if !filepath.IsAbs(filePath) {
		filePath = filepath.Join(v.workingDir, filePath)
	}

	// Check if file is outside working directory and request permission if needed
	absWorkingDir, err := filepath.Abs(v.workingDir)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error resolving working directory: %w", err)
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error resolving file path: %w", err)
	}

	relPath, err := filepath.Rel(absWorkingDir, absFilePath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		// File is outside working directory, request permission
		sessionID, messageID := GetContextValues(ctx)
		if sessionID == "" || messageID == "" {
			return ToolResponse{}, fmt.Errorf("session ID and message ID are required for accessing files outside working directory")
		}

		granted := v.permissions.Request(
			permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        absFilePath,
				ToolCallID:  call.ID,
				ToolName:    ViewToolName,
				Action:      "read",
				Description: fmt.Sprintf("Read file outside working directory: %s", absFilePath),
				Params:      ViewPermissionsParams(params),
			},
		)

		if !granted {
			return ToolResponse{}, permission.ErrorPermissionDenied
		}
	}

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to offer suggestions for similarly named files
			dir := filepath.Dir(filePath)
			base := filepath.Base(filePath)

			dirEntries, dirErr := os.ReadDir(dir)
			if dirErr == nil {
				var suggestions []string
				for _, entry := range dirEntries {
					if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(base)) ||
						strings.Contains(strings.ToLower(base), strings.ToLower(entry.Name())) {
						suggestions = append(suggestions, filepath.Join(dir, entry.Name()))
						if len(suggestions) >= 3 {
							break
						}
					}
				}

				if len(suggestions) > 0 {
					return NewTextErrorResponse(fmt.Sprintf("File not found: %s\n\nDid you mean one of these?\n%s",
						filePath, strings.Join(suggestions, "\n"))), nil
				}
			}

			return NewTextErrorResponse(fmt.Sprintf("File not found: %s", filePath)), nil
		}
		return ToolResponse{}, fmt.Errorf("error accessing file: %w", err)
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return NewTextErrorResponse(fmt.Sprintf("Path is a directory, not a file: %s", filePath)), nil
	}

	// Check file size
	if fileInfo.Size() > MaxReadSize {
		return NewTextErrorResponse(fmt.Sprintf("File is too large (%d bytes). Maximum size is %d bytes",
			fileInfo.Size(), MaxReadSize)), nil
	}

	// Set default limit if not provided
	if params.Limit <= 0 {
		params.Limit = DefaultReadLimit
	}

	// Check if it's an image file
	isImage, imageType := isImageFile(filePath)
	// TODO: handle images
	if isImage {
		return NewTextErrorResponse(fmt.Sprintf("This is an image file of type: %s\n", imageType)), nil
	}

	// Read the file content
	content, lineCount, err := readTextFile(filePath, params.Offset, params.Limit)
	isValidUt8 := utf8.ValidString(content)
	if !isValidUt8 {
		return NewTextErrorResponse("File content is not valid UTF-8"), nil
	}
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error reading file: %w", err)
	}

	notifyLspOpenFile(ctx, filePath, v.lspClients)
	output := "<file>\n"
	// Format the output with line numbers
	output += addLineNumbers(content, params.Offset+1)

	// Add a note if the content was truncated
	if lineCount > params.Offset+len(strings.Split(content, "\n")) {
		output += fmt.Sprintf("\n\n(File has more lines. Use 'offset' parameter to read beyond line %d)",
			params.Offset+len(strings.Split(content, "\n")))
	}
	output += "\n</file>\n"
	output += getDiagnostics(filePath, v.lspClients)
	recordFileRead(filePath)
	return WithResponseMetadata(
		NewTextResponse(output),
		ViewResponseMetadata{
			FilePath: filePath,
			Content:  content,
		},
	), nil
}

func addLineNumbers(content string, startLine int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")

	var result []string
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")

		lineNum := i + startLine
		numStr := fmt.Sprintf("%d", lineNum)

		if len(numStr) >= 6 {
			result = append(result, fmt.Sprintf("%s|%s", numStr, line))
		} else {
			paddedNum := fmt.Sprintf("%6s", numStr)
			result = append(result, fmt.Sprintf("%s|%s", paddedNum, line))
		}
	}

	return strings.Join(result, "\n")
}

func readTextFile(filePath string, offset, limit int) (string, int, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	lineCount := 0

	scanner := NewLineScanner(file)
	if offset > 0 {
		for lineCount < offset && scanner.Scan() {
			lineCount++
		}
		if err = scanner.Err(); err != nil {
			return "", 0, err
		}
	}

	if offset == 0 {
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return "", 0, err
		}
	}

	// Pre-allocate slice with expected capacity
	lines := make([]string, 0, limit)
	lineCount = offset

	for scanner.Scan() && len(lines) < limit {
		lineCount++
		lineText := scanner.Text()
		if len(lineText) > MaxLineLength {
			lineText = lineText[:MaxLineLength] + "..."
		}
		lines = append(lines, lineText)
	}

	// Continue scanning to get total line count
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return "", 0, err
	}

	return strings.Join(lines, "\n"), lineCount, nil
}

func isImageFile(filePath string) (bool, string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return true, "JPEG"
	case ".png":
		return true, "PNG"
	case ".gif":
		return true, "GIF"
	case ".bmp":
		return true, "BMP"
	case ".svg":
		return true, "SVG"
	case ".webp":
		return true, "WebP"
	default:
		return false, ""
	}
}

type LineScanner struct {
	scanner *bufio.Scanner
}

func NewLineScanner(r io.Reader) *LineScanner {
	return &LineScanner{
		scanner: bufio.NewScanner(r),
	}
}

func (s *LineScanner) Scan() bool {
	return s.scanner.Scan()
}

func (s *LineScanner) Text() string {
	return s.scanner.Text()
}

func (s *LineScanner) Err() error {
	return s.scanner.Err()
}
