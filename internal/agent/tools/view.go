package tools

import (
	"bufio"
	"cmp"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/imageutil"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/skills"
)

//go:embed view.md
var viewDescription []byte

type ViewParams struct {
	FilePath string `json:"file_path" description:"The path to the file to read"`
	Offset   int    `json:"offset,omitempty" description:"The line number to start reading from (0-based)"`
	Limit    int    `json:"limit,omitempty" description:"The number of lines to read (defaults to 2000)"`
	Hashline bool   `json:"hashline,omitempty" description:"If true, include hashline anchors in the output for line-addressable editing"`
}

type ViewPermissionsParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
	Hashline bool   `json:"hashline,omitempty"`
}

type ViewResourceType string

const (
	ViewResourceUnset ViewResourceType = ""
	ViewResourceSkill ViewResourceType = "skill"
)

type ViewResponseMetadata struct {
	FilePath            string           `json:"file_path"`
	Content             string           `json:"content"`
	Hashline            bool             `json:"hashline,omitempty"`
	ResourceType        ViewResourceType `json:"resource_type,omitempty"`
	ResourceName        string           `json:"resource_name,omitempty"`
	ResourceDescription string           `json:"resource_description,omitempty"`
}

const (
	ViewToolName     = "view"
	MaxViewSize      = 1 * 1024 * 1024 // 1MB
	DefaultReadLimit = 2000
	MaxLineLength    = 2000
)

var errViewOffsetBeyondEOF = errors.New("offset is beyond end of file")

func NewViewTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	filetracker filetracker.Service,
	workingDir string,
	skillsPaths ...string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		ViewToolName,
		string(viewDescription),
		func(ctx context.Context, params ViewParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}

			// Use session-specific working directory from context if available.
			effectiveWorkingDir := cmp.Or(GetWorkingDirFromContext(ctx), workingDir)

			// Handle relative paths
			filePath := filepathext.SmartJoin(effectiveWorkingDir, params.FilePath)

			// Check if file is outside working directory and request permission if needed
			absWorkingDir, err := filepath.Abs(effectiveWorkingDir)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error resolving working directory: %w", err)
			}

			absFilePath, err := filepath.Abs(filePath)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("error resolving file path: %w", err)
			}

			relPath, err := filepath.Rel(absWorkingDir, absFilePath)
			isOutsideWorkDir := err != nil || strings.HasPrefix(relPath, "..")
			isSkillFile := isInSkillsPath(absFilePath, skillsPaths)

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for accessing files outside working directory")
			}

			// Request permission for files outside working directory, unless it's a skill file.
			if isOutsideWorkDir && !isSkillFile {
				permissionResponse, permReqErr := RequestPermission(ctx, permissions,
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
				if permReqErr != nil {
					return fantasy.ToolResponse{}, permReqErr
				}
				if permissionResponse != nil {
					return *permissionResponse, nil
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
							return fantasy.NewTextErrorResponse(fmt.Sprintf("File not found: %s\n\nDid you mean one of these?\n%s",
								filePath, strings.Join(suggestions, "\n"))), nil
						}
					}

					return fantasy.NewTextErrorResponse(fmt.Sprintf("File not found: %s", filePath)), nil
				}
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error accessing file: %v", err)), nil
			}

			// Check if it's a directory
			if fileInfo.IsDir() {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Path is a directory, not a file: %s", filePath)), nil
			}

			isSupportedImage, mimeType := getImageMimeType(filePath)

			// Based on the specifications we should not limit the skills read.
			// For images, we allow reading first so compression can be applied.
			if !isSkillFile && !isSupportedImage && fileInfo.Size() > MaxViewSize {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("File is too large (%d bytes). Maximum size is %d bytes",
					fileInfo.Size(), MaxViewSize)), nil
			}

			// Set default limit if not provided (no limit for SKILL.md files)
			if params.Limit <= 0 {
				if isSkillFile {
					params.Limit = 1000000 // Effectively no limit for skill files
				} else {
					params.Limit = DefaultReadLimit
				}
			}

			if isSupportedImage {
				if !GetSupportsImagesFromContext(ctx) {
					modelName := GetModelNameFromContext(ctx)
					return fantasy.NewTextErrorResponse(fmt.Sprintf("This model (%s) does not support image data.", modelName)), nil
				}

				imageData, readErr := os.ReadFile(filePath)
				if readErr != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("error reading image file: %w", readErr)
				}

				// Compress image if it exceeds 1MB.
				config := imageutil.DefaultCompressionConfig()
				result, compressErr := imageutil.CompressImage(imageData, mimeType, config)
				if compressErr != nil {
					slog.Warn("Failed to compress image, using original", "error", compressErr, "path", filePath)
					// Fall through with original data.
					result = &imageutil.CompressResult{
						Data:          imageData,
						MimeType:      mimeType,
						WasCompressed: false,
					}
				}

				return fantasy.NewImageResponse(result.Data, result.MimeType), nil
			}

			// Read the file content.
			content, hasMore, err := readTextFile(filePath, params.Offset, params.Limit)
			if err != nil {
				if errors.Is(err, errViewOffsetBeyondEOF) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Offset %d is beyond end of file", params.Offset)), nil
				}
				return fantasy.ToolResponse{}, fmt.Errorf("error reading file: %w", err)
			}
			if !utf8.ValidString(content) {
				return fantasy.NewTextErrorResponse("File content is not valid UTF-8"), nil
			}

			openInLSPs(ctx, lspManager, filePath)
			waitForLSPDiagnostics(ctx, lspManager, filePath, 300*time.Millisecond)
			output := "<file>\n"
			if params.Hashline {
				output += addHashlineLineNumbers(content, params.Offset+1)
			} else {
				output += addLineNumbers(content, params.Offset+1)
			}

			if hasMore {
				output += fmt.Sprintf("\n\n(File has more lines. Use 'offset' parameter to read beyond line %d)",
					params.Offset+len(strings.Split(content, "\n")))
			}
			output += "\n</file>\n"
			output += getDiagnostics(filePath, lspManager)
			filetracker.RecordRead(ctx, sessionID, filePath)

			meta := ViewResponseMetadata{
				FilePath: filePath,
				Content:  content,
				Hashline: params.Hashline,
			}
			if isSkillFile {
				if skill, err := skills.Parse(filePath); err == nil {
					meta.ResourceType = ViewResourceSkill
					meta.ResourceName = skill.Name
					meta.ResourceDescription = skill.Description
				}
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				meta,
			), nil
		})
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

func addHashlineLineNumbers(content string, startLine int) string {
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		lineNum := i + startLine
		result = append(result, fmt.Sprintf("%6s#%s|%s", fmt.Sprintf("%d", lineNum), computeHashlineID(lineNum, line), line))
	}

	return strings.Join(result, "\n")
}

func readTextFile(filePath string, offset, limit int) (string, bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	scanner := NewLineScanner(file)
	skipped := 0
	if offset > 0 {
		for skipped < offset && scanner.Scan() {
			skipped++
		}
		if err = scanner.Err(); err != nil {
			return "", false, err
		}
		if skipped < offset {
			return "", false, errViewOffsetBeyondEOF
		}
	}

	// Pre-allocate slice with expected capacity.
	lines := make([]string, 0, limit)

	for len(lines) < limit && scanner.Scan() {
		lineText := scanner.Text()
		if len(lineText) > MaxLineLength {
			lineText = lineText[:MaxLineLength] + "..."
		}
		lines = append(lines, lineText)
	}

	// Peek one more line only when we filled the limit.
	hasMore := len(lines) == limit && scanner.Scan()

	if err := scanner.Err(); err != nil {
		return "", false, err
	}

	return strings.Join(lines, "\n"), hasMore, nil
}

func getImageMimeType(filePath string) (bool, string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".jpg", ".jpeg":
		return true, "image/jpeg"
	case ".png":
		return true, "image/png"
	case ".gif":
		return true, "image/gif"
	case ".webp":
		return true, "image/webp"
	default:
		return false, ""
	}
}

type LineScanner struct {
	scanner *bufio.Scanner
}

func NewLineScanner(r io.Reader) *LineScanner {
	scanner := bufio.NewScanner(r)
	// Increase buffer size to handle large lines (e.g., minified JSON, HTML)
	// Default is 64KB, set to 1MB
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	return &LineScanner{
		scanner: scanner,
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

// isInSkillsPath checks if filePath is within any of the configured skills
// directories. Returns true for files that can be read without permission
// prompts and without size limits.
//
// Note that symlinks are resolved to prevent path traversal attacks via
// symbolic links.
func isInSkillsPath(filePath string, skillsPaths []string) bool {
	if len(skillsPaths) == 0 {
		return false
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	evalFilePath, err := filepath.EvalSymlinks(absFilePath)
	if err != nil {
		return false
	}

	for _, skillsPath := range skillsPaths {
		absSkillsPath, err := filepath.Abs(skillsPath)
		if err != nil {
			continue
		}

		evalSkillsPath, err := filepath.EvalSymlinks(absSkillsPath)
		if err != nil {
			continue
		}

		relPath, err := filepath.Rel(evalSkillsPath, evalFilePath)
		if err == nil && !strings.HasPrefix(relPath, "..") {
			return true
		}
	}

	return false
}
