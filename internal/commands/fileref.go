package commands

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// fileRefPattern matches file references like @filename, @path/to/file, @file.txt
	// Matches @ followed by one or more word characters, dots, slashes, dashes, underscores
	// Pattern: @ followed by valid filename characters
	fileRefPattern = regexp.MustCompile(`@([\w./\\-]+)`)
)

// parseFileReferences extracts all file references from command content.
//
// File references use the syntax @filename where filename can be:
//   - Simple: @file.txt
//   - With path: @path/to/file.txt
//   - With extension: @script.sh
//   - Relative paths: @../parent/file.txt
//
// Examples:
//   - Content: "Review @file1.txt and @file2.go" → ["file1.txt", "file2.go"]
//   - Content: "Process @src/main.go" → ["src/main.go"]
//   - Content: "No references" → []
//
// Returns a slice of file paths (without the @ prefix).
// Malformed references (e.g., just @) are skipped.
func parseFileReferences(content string) []string {
	matches := fileRefPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return []string{}
	}

	fileRefs := make([]string, 0, len(matches))
	seen := make(map[string]bool) // Track duplicates

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		filePath := strings.TrimSpace(match[1])
		if filePath == "" {
			continue
		}

		// Skip duplicates
		if seen[filePath] {
			continue
		}

		seen[filePath] = true
		fileRefs = append(fileRefs, filePath)
	}

	return fileRefs
}

// resolveFilePaths resolves file paths from @filename references relative to a working directory.
//
// The function:
//   - Resolves relative paths against the working directory
//   - Preserves absolute paths as-is
//   - Normalizes path separators for cross-platform compatibility
//   - Returns absolute paths for all resolved files
//
// Examples:
//   - "file.txt" + workingDir="/project" → "/project/file.txt"
//   - "src/main.go" + workingDir="/project" → "/project/src/main.go"
//   - "/absolute/path/file.txt" + workingDir="/project" → "/absolute/path/file.txt"
//   - "../parent/file.txt" + workingDir="/project/sub" → "/project/parent/file.txt"
//
// Parameters:
//   - filePaths: Slice of file paths extracted from @filename references
//   - workingDir: The working directory to resolve relative paths against
//
// Returns a slice of resolved absolute file paths.
func resolveFilePaths(filePaths []string, workingDir string) []string {
	if len(filePaths) == 0 {
		return []string{}
	}

	resolved := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		// Normalize path separators to forward slashes first
		// This converts both Windows backslashes and Unix backslashes (if used incorrectly)
		normalized := filepath.ToSlash(filePath)
		// Also replace any remaining backslashes (literal characters) with forward slashes
		normalized = strings.ReplaceAll(normalized, "\\", "/")

		// Check if path is absolute (check original before normalization)
		if filepath.IsAbs(filePath) {
			// Absolute path - clean it (preserves as absolute)
			absPath := filepath.Clean(normalized)
			// Convert back to platform-specific separators
			resolved = append(resolved, filepath.FromSlash(absPath))
		} else {
			// Relative path - resolve against working directory
			// Clean the normalized path first, then join (this ensures proper normalization)
			cleaned := filepath.Clean(normalized)
			resolvedPath := filepath.Join(workingDir, cleaned)
			absPath := filepath.Clean(resolvedPath)
			resolved = append(resolved, absPath)
		}
	}

	return resolved
}

