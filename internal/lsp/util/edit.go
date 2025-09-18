package util

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/x/powernap/pkg/lsp/protocol"
)

func applyTextEdits(uri protocol.DocumentURI, edits []protocol.TextEdit) error {
	path, err := uri.Path()
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	// Read the file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Detect line ending style
	var lineEnding string
	if bytes.Contains(content, []byte("\r\n")) {
		lineEnding = "\r\n"
	} else {
		lineEnding = "\n"
	}

	// Track if file ends with a newline
	endsWithNewline := len(content) > 0 && bytes.HasSuffix(content, []byte(lineEnding))

	// Split into lines without the endings
	lines := strings.Split(string(content), lineEnding)

	// Check for overlapping edits
	for i, edit1 := range edits {
		for j := i + 1; j < len(edits); j++ {
			if rangesOverlap(edit1.Range, edits[j].Range) {
				return fmt.Errorf("overlapping edits detected between edit %d and %d", i, j)
			}
		}
	}

	// Sort edits in reverse order
	sortedEdits := make([]protocol.TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		if sortedEdits[i].Range.Start.Line != sortedEdits[j].Range.Start.Line {
			return sortedEdits[i].Range.Start.Line > sortedEdits[j].Range.Start.Line
		}
		return sortedEdits[i].Range.Start.Character > sortedEdits[j].Range.Start.Character
	})

	// Apply each edit
	for _, edit := range sortedEdits {
		newLines, err := applyTextEdit(lines, edit)
		if err != nil {
			return fmt.Errorf("failed to apply edit: %w", err)
		}
		lines = newLines
	}

	// Join lines with proper line endings
	var newContent strings.Builder
	for i, line := range lines {
		if i > 0 {
			newContent.WriteString(lineEnding)
		}
		newContent.WriteString(line)
	}

	// Only add a newline if the original file had one and we haven't already added it
	if endsWithNewline && !strings.HasSuffix(newContent.String(), lineEnding) {
		newContent.WriteString(lineEnding)
	}

	if err := os.WriteFile(path, []byte(newContent.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func applyTextEdit(lines []string, edit protocol.TextEdit) ([]string, error) {
	startLine := int(edit.Range.Start.Line)
	endLine := int(edit.Range.End.Line)
	startChar := int(edit.Range.Start.Character)
	endChar := int(edit.Range.End.Character)

	// Validate positions
	if startLine < 0 || startLine >= len(lines) {
		return nil, fmt.Errorf("invalid start line: %d", startLine)
	}
	if endLine < 0 || endLine >= len(lines) {
		endLine = len(lines) - 1
	}

	// Create result slice with initial capacity
	result := make([]string, 0, len(lines))

	// Copy lines before edit
	result = append(result, lines[:startLine]...)

	// Get the prefix of the start line
	startLineContent := lines[startLine]
	if startChar < 0 || startChar > len(startLineContent) {
		startChar = len(startLineContent)
	}
	prefix := startLineContent[:startChar]

	// Get the suffix of the end line
	endLineContent := lines[endLine]
	if endChar < 0 || endChar > len(endLineContent) {
		endChar = len(endLineContent)
	}
	suffix := endLineContent[endChar:]

	// Handle the edit
	if edit.NewText == "" {
		if prefix+suffix != "" {
			result = append(result, prefix+suffix)
		}
	} else {
		// Split new text into lines, being careful not to add extra newlines
		// newLines := strings.Split(strings.TrimRight(edit.NewText, "\n"), "\n")
		newLines := strings.Split(edit.NewText, "\n")

		if len(newLines) == 1 {
			// Single line change
			result = append(result, prefix+newLines[0]+suffix)
		} else {
			// Multi-line change
			result = append(result, prefix+newLines[0])
			result = append(result, newLines[1:len(newLines)-1]...)
			result = append(result, newLines[len(newLines)-1]+suffix)
		}
	}

	// Add remaining lines
	if endLine+1 < len(lines) {
		result = append(result, lines[endLine+1:]...)
	}

	return result, nil
}

// applyDocumentChange applies a DocumentChange (create/rename/delete operations)
func applyDocumentChange(change protocol.DocumentChange) error {
	if change.CreateFile != nil {
		path, err := change.CreateFile.URI.Path()
		if err != nil {
			return fmt.Errorf("invalid URI: %w", err)
		}

		if change.CreateFile.Options != nil {
			if change.CreateFile.Options.Overwrite {
				// Proceed with overwrite
			} else if change.CreateFile.Options.IgnoreIfExists {
				if _, err := os.Stat(path); err == nil {
					return nil // File exists and we're ignoring it
				}
			}
		}
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}

	if change.DeleteFile != nil {
		path, err := change.DeleteFile.URI.Path()
		if err != nil {
			return fmt.Errorf("invalid URI: %w", err)
		}

		if change.DeleteFile.Options != nil && change.DeleteFile.Options.Recursive {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to delete directory recursively: %w", err)
			}
		} else {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to delete file: %w", err)
			}
		}
	}

	if change.RenameFile != nil {
		var newPath, oldPath string
		var err error

		oldPath, err = change.RenameFile.OldURI.Path()
		if err != nil {
			return err
		}

		newPath, err = change.RenameFile.NewURI.Path()
		if err != nil {
			return err
		}

		if change.RenameFile.Options != nil {
			if !change.RenameFile.Options.Overwrite {
				if _, err := os.Stat(newPath); err == nil {
					return fmt.Errorf("target file already exists and overwrite is not allowed: %s", newPath)
				}
			}
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to rename file: %w", err)
		}
	}

	if change.TextDocumentEdit != nil {
		textEdits := make([]protocol.TextEdit, len(change.TextDocumentEdit.Edits))
		for i, edit := range change.TextDocumentEdit.Edits {
			var err error
			textEdits[i], err = edit.AsTextEdit()
			if err != nil {
				return fmt.Errorf("invalid edit type: %w", err)
			}
		}
		return applyTextEdits(change.TextDocumentEdit.TextDocument.URI, textEdits)
	}

	return nil
}

// ApplyWorkspaceEdit applies the given WorkspaceEdit to the filesystem
func ApplyWorkspaceEdit(edit protocol.WorkspaceEdit) error {
	// Handle Changes field
	for uri, textEdits := range edit.Changes {
		if err := applyTextEdits(uri, textEdits); err != nil {
			return fmt.Errorf("failed to apply text edits: %w", err)
		}
	}

	// Handle DocumentChanges field
	for _, change := range edit.DocumentChanges {
		if err := applyDocumentChange(change); err != nil {
			return fmt.Errorf("failed to apply document change: %w", err)
		}
	}

	return nil
}

func rangesOverlap(r1, r2 protocol.Range) bool {
	if r1.Start.Line > r2.End.Line || r2.Start.Line > r1.End.Line {
		return false
	}
	if r1.Start.Line == r2.End.Line && r1.Start.Character > r2.End.Character {
		return false
	}
	if r2.Start.Line == r1.End.Line && r2.Start.Character > r1.End.Character {
		return false
	}
	return true
}
