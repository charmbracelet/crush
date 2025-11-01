package commands

import (
	"path/filepath"
	"strings"
)

// deriveCommandName derives the command name from a file path, handling subdirectories as namespaces.
// Examples:
//   - `review-pr.md` → `review-pr` (no namespace)
//   - `frontend/review-pr.md` → `frontend:review-pr`
//   - `frontend/components/button.md` → `frontend:components:button`
//
// The function handles cross-platform path separators and removes the `.md` extension.
func deriveCommandName(path string, baseDir string) (name string, namespace string) {
	// Make path relative to base directory
	relPath, err := filepath.Rel(baseDir, path)
	if err != nil {
		// If relative path fails, use original path
		relPath = path
	}

	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	// Remove .md extension
	relPath = strings.TrimSuffix(relPath, ".md")
	relPath = strings.TrimSuffix(relPath, ".MD")

	// Split into directory and filename parts
	dir := filepath.Dir(relPath)
	filename := filepath.Base(relPath)

	// If dir is "." or empty, this is a root-level command
	if dir == "." || dir == "" || dir == "/" {
		return filename, ""
	}

	// Convert directory path to namespace (replace / with :)
	namespace = strings.ReplaceAll(dir, "/", ":")
	namespace = strings.ReplaceAll(namespace, "\\", ":") // Handle Windows paths

	// Full name is namespace:filename
	name = namespace + ":" + filename

	return name, namespace
}

