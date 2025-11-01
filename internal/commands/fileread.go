package commands

import (
	"log/slog"
	"os"
)

// FileContent represents a file with its content ready to be attached.
type FileContent struct {
	// Path is the resolved absolute path of the file.
	Path string

	// Content is the file content as a string.
	// Empty if file could not be read.
	Content string
}

// readFileContents reads file contents from resolved paths.
//
// For each file path:
//   - Attempts to read the file content
//   - Returns FileContent with path and content
//   - If file cannot be read (not found, permission denied, etc.), logs error and returns empty content
//   - Errors are logged but don't stop processing of other files
//
// Parameters:
//   - filePaths: Slice of resolved absolute file paths
//
// Returns a slice of FileContent structs, one per file path.
// Files that couldn't be read will have empty Content but will still be included with their Path.
func readFileContents(filePaths []string) []FileContent {
	if len(filePaths) == 0 {
		return []FileContent{}
	}

	results := make([]FileContent, 0, len(filePaths))
	for _, filePath := range filePaths {
		content, err := readSingleFile(filePath)
		if err != nil {
			// Log error but continue processing other files
			slog.Warn("Failed to read file for command attachment",
				"file_path", filePath,
				"error", err,
			)
			// Include file with empty content so caller knows it was attempted
			results = append(results, FileContent{
				Path:    filePath,
				Content: "",
			})
		} else {
			results = append(results, FileContent{
				Path:    filePath,
				Content: content,
			})
		}
	}

	return results
}

// readSingleFile reads a single file and returns its content.
// Handles various error conditions and logs them appropriately.
func readSingleFile(filePath string) (string, error) {
	// Check if file exists and get info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", &FileReadError{
				Path:  filePath,
				Type:  ErrorTypeNotFound,
				Cause: err,
			}
		}
		// Permission denied or other stat error
		return "", &FileReadError{
			Path:  filePath,
			Type:  ErrorTypeAccess,
			Cause: err,
		}
	}

	// Check if it's a directory
	if fileInfo.IsDir() {
		return "", &FileReadError{
			Path:  filePath,
			Type:  ErrorTypeIsDirectory,
			Cause: nil,
		}
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsPermission(err) {
			return "", &FileReadError{
				Path:  filePath,
				Type:  ErrorTypePermissionDenied,
				Cause: err,
			}
		}
		// Other read errors
		return "", &FileReadError{
			Path:  filePath,
			Type:  ErrorTypeRead,
			Cause: err,
		}
	}

	return string(content), nil
}

// FileReadError represents an error encountered while reading a file.
type FileReadError struct {
	Path  string
	Type  ErrorType
	Cause error
}

func (e *FileReadError) Error() string {
	switch e.Type {
	case ErrorTypeNotFound:
		return "file not found: " + e.Path
	case ErrorTypePermissionDenied:
		return "permission denied reading file: " + e.Path
	case ErrorTypeIsDirectory:
		return "path is a directory, not a file: " + e.Path
	case ErrorTypeAccess:
		return "cannot access file: " + e.Path
	case ErrorTypeRead:
		return "error reading file: " + e.Path
	default:
		return "unknown error reading file: " + e.Path
	}
}

func (e *FileReadError) Unwrap() error {
	return e.Cause
}

// ErrorType categorizes file read errors.
type ErrorType string

const (
	ErrorTypeNotFound        ErrorType = "not_found"
	ErrorTypePermissionDenied ErrorType = "permission_denied"
	ErrorTypeIsDirectory     ErrorType = "is_directory"
	ErrorTypeAccess          ErrorType = "access"
	ErrorTypeRead            ErrorType = "read"
)

