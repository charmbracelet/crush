package commands

import (
	"mime"
	"net/http"
	"path/filepath"

	"github.com/charmbracelet/crush/internal/message"
)

// buildFileAttachments creates message.Attachment objects from file contents.
//
// For each FileContent:
//   - Creates a message.Attachment with file path, name, MIME type, and content
//   - Detects MIME type from file content and extension
//   - Extracts filename from file path
//   - Skips files with empty content (failed reads)
//
// Parameters:
//   - fileContents: Slice of FileContent structs from readFileContents
//
// Returns a slice of message.Attachment objects, ready to be passed to the agent coordinator.
// Files with empty content are skipped (not included in the result).
func buildFileAttachments(fileContents []FileContent) []message.Attachment {
	if len(fileContents) == 0 {
		return []message.Attachment{}
	}

	attachments := make([]message.Attachment, 0, len(fileContents))
	for _, fileContent := range fileContents {
		// Skip files with empty content (failed reads)
		if fileContent.Content == "" {
			continue
		}

		// Extract filename from path
		fileName := filepath.Base(fileContent.Path)

		// Detect MIME type
		mimeType := detectMimeType(fileContent.Path, []byte(fileContent.Content))

		// Create attachment
		attachment := message.Attachment{
			FilePath: fileContent.Path,
			FileName: fileName,
			MimeType: mimeType,
			Content:  []byte(fileContent.Content),
		}

		attachments = append(attachments, attachment)
	}

	return attachments
}

// detectMimeType detects the MIME type of a file.
// First tries to detect from content, then falls back to extension.
func detectMimeType(filePath string, content []byte) string {
	// Try content-based detection (first 512 bytes)
	mimeBufferSize := min(512, len(content))
	if mimeBufferSize > 0 {
		if detected := http.DetectContentType(content[:mimeBufferSize]); detected != "application/octet-stream" {
			return detected
		}
	}

	// Fall back to extension-based detection
	ext := filepath.Ext(filePath)
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		return mimeType
	}

	// Default to text/plain for unknown types
	return "text/plain"
}

