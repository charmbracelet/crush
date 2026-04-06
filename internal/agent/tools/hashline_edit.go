package tools

import (
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/permission"
)

//go:embed hashline_edit.md
var hashlineEditDescription []byte

type parsedHashlineOperation struct {
	Operation    string
	Line         hashlineRef
	Start        hashlineRef
	End          hashlineRef
	ContentLines []string
}

func NewHashlineEditTool(
	lspManager *lsp.Manager,
	permissions permission.Service,
	files history.Service,
	filetracker filetracker.Service,
	workingDir string,
) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		HashlineEditToolName,
		string(hashlineEditDescription),
		func(ctx context.Context, params HashlineEditParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.FilePath == "" {
				return fantasy.NewTextErrorResponse("file_path is required"), nil
			}
			if len(params.Operations) == 0 {
				return fantasy.NewTextErrorResponse("at least one operation is required"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for hashline edit")
			}

			effectiveWorkingDir := cmp.Or(GetWorkingDirFromContext(ctx), workingDir)
			params.FilePath = filepathext.SmartJoin(effectiveWorkingDir, params.FilePath)

			fileInfo, err := os.Stat(params.FilePath)
			if err != nil {
				if os.IsNotExist(err) {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("file not found: %s", params.FilePath)), nil
				}
				return fantasy.ToolResponse{}, fmt.Errorf("failed to access file: %w", err)
			}
			if fileInfo.IsDir() {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("path is a directory, not a file: %s", params.FilePath)), nil
			}

			lastRead := filetracker.LastReadTime(ctx, sessionID, params.FilePath)
			if lastRead.IsZero() {
				return fantasy.NewTextErrorResponse("you must read the file before editing it. Use the View tool first"), nil
			}

			modTime := fileInfo.ModTime().Truncate(time.Second)
			if modTime.After(lastRead) {
				return fantasy.NewTextErrorResponse(
					fmt.Sprintf("file %s has been modified since it was last read (mod time: %s, last read: %s)",
						params.FilePath, modTime.Format(time.RFC3339), lastRead.Format(time.RFC3339),
					)), nil
			}

			content, err := os.ReadFile(params.FilePath)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to read file: %w", err)
			}

			oldContent, isCrlf := fsext.ToUnixLineEndings(string(content))
			oldLines, hadTrailingNewline := splitHashlineFileLines(oldContent)

			operations, err := parseHashlineOperations(params.Operations, oldLines)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			newLines, err := applyHashlineOperations(oldLines, operations)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}

			newContent := joinHashlineFileLines(newLines, hadTrailingNewline)
			if newContent == oldContent {
				return fantasy.NewTextErrorResponse("new content is the same as old content. No changes made."), nil
			}

			_, additions, removals := diff.GenerateDiff(
				oldContent,
				newContent,
				strings.TrimPrefix(params.FilePath, effectiveWorkingDir),
			)

			permissionResponse, err := RequestPermission(ctx, permissions,
				permission.CreatePermissionRequest{
					SessionID:          sessionID,
					AuthoritySessionID: ResolveAuthoritySessionID(ctx, sessionID),
					Path:               fsext.PathOrPrefix(params.FilePath, effectiveWorkingDir),
					ToolCallID:         call.ID,
					ToolName:           HashlineEditToolName,
					Action:             "write",
					Description:        fmt.Sprintf("Apply %d hashline operations to file %s", len(operations), params.FilePath),
					Params: HashlineEditPermissionsParams{
						FilePath:   params.FilePath,
						OldContent: oldContent,
						NewContent: newContent,
					},
				},
			)
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if permissionResponse != nil {
				return *permissionResponse, nil
			}

			if isCrlf {
				newContent, _ = fsext.ToWindowsLineEndings(newContent)
			}

			err = os.WriteFile(params.FilePath, []byte(newContent), 0o644)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to write file: %w", err)
			}

			file, err := files.GetByPathAndSession(ctx, params.FilePath, sessionID)
			if err != nil {
				_, err = files.Create(ctx, sessionID, params.FilePath, oldContent)
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("error creating file history: %w", err)
				}
			}
			if file.Content != oldContent {
				_, err = files.CreateVersion(ctx, sessionID, params.FilePath, oldContent)
				if err != nil {
					slog.Error("Error creating file history version", "error", err)
				}
			}
			_, err = files.CreateVersion(ctx, sessionID, params.FilePath, newContent)
			if err != nil {
				slog.Error("Error creating file history version", "error", err)
			}

			filetracker.RecordRead(ctx, sessionID, params.FilePath)
			notifyLSPs(ctx, lspManager, params.FilePath)

			response := fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(fmt.Sprintf("Applied %d hashline operation(s) to file: %s", len(operations), params.FilePath)),
				HashlineEditResponseMetadata{
					FilePath:   params.FilePath,
					OldContent: oldContent,
					NewContent: newContent,
					Additions:  additions,
					Removals:   removals,
				},
			)

			text := fmt.Sprintf("<result>\n%s\n</result>\n", response.Content)
			text += getDiagnostics(params.FilePath, lspManager)
			response.Content = text
			return response, nil
		},
	)
}

func parseHashlineOperations(operations []HashlineEditOperation, originalLines []string) ([]parsedHashlineOperation, error) {
	parsed := make([]parsedHashlineOperation, 0, len(operations))
	for i, operation := range operations {
		opIndex := i + 1
		contentLines := splitHashlineContent(operation.Content)

		switch operation.Operation {
		case hashlineEditOpReplaceLine:
			lineRef, err := parseAndValidateHashlineReference(operation.Line, originalLines)
			if err != nil {
				return nil, fmt.Errorf("operation %d (%s): %w", opIndex, operation.Operation, err)
			}
			parsed = append(parsed, parsedHashlineOperation{
				Operation:    operation.Operation,
				Line:         lineRef,
				ContentLines: contentLines,
			})
		case hashlineEditOpReplaceRange:
			startRef, err := parseAndValidateHashlineReference(operation.Start, originalLines)
			if err != nil {
				return nil, fmt.Errorf("operation %d (%s): %w", opIndex, operation.Operation, err)
			}
			endRef, err := parseAndValidateHashlineReference(operation.End, originalLines)
			if err != nil {
				return nil, fmt.Errorf("operation %d (%s): %w", opIndex, operation.Operation, err)
			}
			if startRef.Line > endRef.Line {
				return nil, fmt.Errorf("operation %d (%s): start line must be less than or equal to end line", opIndex, operation.Operation)
			}
			parsed = append(parsed, parsedHashlineOperation{
				Operation:    operation.Operation,
				Start:        startRef,
				End:          endRef,
				ContentLines: contentLines,
			})
		case hashlineEditOpPrepend, hashlineEditOpAppend:
			if len(contentLines) == 0 {
				return nil, fmt.Errorf("operation %d (%s): content cannot be empty", opIndex, operation.Operation)
			}
			lineRef, err := parseAndValidateHashlineReference(operation.Line, originalLines)
			if err != nil {
				return nil, fmt.Errorf("operation %d (%s): %w", opIndex, operation.Operation, err)
			}
			parsed = append(parsed, parsedHashlineOperation{
				Operation:    operation.Operation,
				Line:         lineRef,
				ContentLines: contentLines,
			})
		default:
			return nil, fmt.Errorf("operation %d: unsupported operation %q. Use replace_line, replace_range, prepend, or append", opIndex, operation.Operation)
		}
	}

	return parsed, nil
}

func parseAndValidateHashlineReference(reference string, lines []string) (hashlineRef, error) {
	parsedRef, err := parseHashlineReference(reference)
	if err != nil {
		return hashlineRef{}, err
	}

	currentHash, err := validateHashlineReference(parsedRef, lines)
	if err != nil {
		if currentHash != "" {
			return hashlineRef{}, fmt.Errorf("%w (current hash is %s). Re-run view with hashline=true and retry", err, currentHash)
		}
		return hashlineRef{}, err
	}

	return parsedRef, nil
}

func splitHashlineFileLines(content string) ([]string, bool) {
	if content == "" {
		return []string{}, false
	}

	hasTrailingNewline := strings.HasSuffix(content, "\n")
	lines := strings.Split(content, "\n")
	if hasTrailingNewline {
		lines = lines[:len(lines)-1]
	}
	return lines, hasTrailingNewline
}

func joinHashlineFileLines(lines []string, trailingNewline bool) string {
	if len(lines) == 0 {
		return ""
	}

	content := strings.Join(lines, "\n")
	if trailingNewline {
		content += "\n"
	}
	return content
}

func splitHashlineContent(content string) []string {
	normalized, _ := fsext.ToUnixLineEndings(content)
	if normalized == "" {
		return nil
	}
	lines := strings.Split(normalized, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func applyHashlineOperations(originalLines []string, operations []parsedHashlineOperation) ([]string, error) {
	currentLines := append([]string(nil), originalLines...)
	mapping := make([]int, len(originalLines)+1)
	for line := 1; line <= len(originalLines); line++ {
		mapping[line] = line
	}

	// Track cumulative insert offsets for each original line.
	// prependOffsets[line] = total lines prepended before line
	// appendOffsets[line] = total lines appended after line
	prependOffsets := make([]int, len(originalLines)+1)
	appendOffsets := make([]int, len(originalLines)+1)

	for i, operation := range operations {
		var err error
		switch operation.Operation {
		case hashlineEditOpReplaceLine:
			currentLines, mapping, err = replaceOriginalRange(currentLines, mapping, operation.Line.Line, operation.Line.Line, operation.ContentLines)
		case hashlineEditOpReplaceRange:
			currentLines, mapping, err = replaceOriginalRange(currentLines, mapping, operation.Start.Line, operation.End.Line, operation.ContentLines)
		case hashlineEditOpPrepend:
			currentLines, mapping, prependOffsets, err = insertRelativeToOriginalLine(currentLines, mapping, prependOffsets, appendOffsets, operation.Line.Line, true, operation.ContentLines)
		case hashlineEditOpAppend:
			currentLines, mapping, appendOffsets, err = insertRelativeToOriginalLine(currentLines, mapping, prependOffsets, appendOffsets, operation.Line.Line, false, operation.ContentLines)
		default:
			err = fmt.Errorf("unsupported operation %q", operation.Operation)
		}
		if err != nil {
			return nil, fmt.Errorf("operation %d (%s): %w", i+1, operation.Operation, err)
		}
	}

	return currentLines, nil
}

func replaceOriginalRange(lines []string, mapping []int, startOriginal, endOriginal int, contentLines []string) ([]string, []int, error) {
	startCurrent, err := resolveCurrentLine(mapping, startOriginal)
	if err != nil {
		return nil, nil, fmt.Errorf("start line %d: %w", startOriginal, err)
	}
	endCurrent, err := resolveCurrentLine(mapping, endOriginal)
	if err != nil {
		return nil, nil, fmt.Errorf("end line %d: %w", endOriginal, err)
	}
	if startCurrent > endCurrent {
		return nil, nil, fmt.Errorf("resolved range is invalid (%d > %d)", startCurrent, endCurrent)
	}

	replacedLength := endCurrent - startCurrent + 1
	updatedLines := make([]string, 0, len(lines)-replacedLength+len(contentLines))
	updatedLines = append(updatedLines, lines[:startCurrent-1]...)
	updatedLines = append(updatedLines, contentLines...)
	updatedLines = append(updatedLines, lines[endCurrent:]...)

	delta := len(contentLines) - replacedLength
	updatedMapping := append([]int(nil), mapping...)
	for originalLine := 1; originalLine < len(updatedMapping); originalLine++ {
		position := mapping[originalLine]
		if position == 0 {
			continue
		}

		switch {
		case position < startCurrent:
			continue
		case position > endCurrent:
			updatedMapping[originalLine] = position + delta
		default:
			relative := position - startCurrent
			if relative < len(contentLines) {
				updatedMapping[originalLine] = startCurrent + relative
			} else {
				updatedMapping[originalLine] = 0
			}
		}
	}

	return updatedLines, updatedMapping, nil
}

func insertRelativeToOriginalLine(lines []string, mapping []int, prependOffsets []int, appendOffsets []int, originalLine int, before bool, contentLines []string) ([]string, []int, []int, error) {
	if len(contentLines) == 0 {
		if before {
			return lines, mapping, prependOffsets, nil
		}
		return lines, mapping, appendOffsets, nil
	}

	// Check if the anchor line was deleted
	if originalLine >= len(mapping) || mapping[originalLine] == 0 {
		return nil, nil, nil, fmt.Errorf("line %d no longer exists after previous operations", originalLine)
	}

	// Calculate current position: original position + all inserts before this line + own prepends
	lineCurrent := originalLine
	for prevLine := 1; prevLine < originalLine; prevLine++ {
		if mapping[prevLine] > 0 {
			lineCurrent += prependOffsets[prevLine] + appendOffsets[prevLine]
		}
	}
	lineCurrent += prependOffsets[originalLine]

	// For prepend: insert before the line (at its current position)
	// For append: insert after the line (after existing appends)
	insertAt := lineCurrent
	if !before {
		insertAt = lineCurrent + 1 + appendOffsets[originalLine]
	}

	updatedLines := make([]string, 0, len(lines)+len(contentLines))
	updatedLines = append(updatedLines, lines[:insertAt-1]...)
	updatedLines = append(updatedLines, contentLines...)
	updatedLines = append(updatedLines, lines[insertAt-1:]...)

	// Update the appropriate offset
	updatedPrepend := append([]int(nil), prependOffsets...)
	updatedAppend := append([]int(nil), appendOffsets...)
	if before {
		updatedPrepend[originalLine] += len(contentLines)
	} else {
		updatedAppend[originalLine] += len(contentLines)
	}

	// Recalculate mapping based on new offsets (mapping tracks where each original line currently is)
	updatedMapping := make([]int, len(mapping))
	for orig := 1; orig < len(mapping); orig++ {
		if mapping[orig] == 0 {
			continue // Line was deleted
		}
		// Position = original line number + inserts before it + own prepends
		pos := orig
		for prev := 1; prev < orig; prev++ {
			if mapping[prev] > 0 {
				pos += updatedPrepend[prev] + updatedAppend[prev]
			}
		}
		pos += updatedPrepend[orig]
		updatedMapping[orig] = pos
	}

	if before {
		return updatedLines, updatedMapping, updatedPrepend, nil
	}
	return updatedLines, updatedMapping, updatedAppend, nil
}

func resolveCurrentLine(mapping []int, originalLine int) (int, error) {
	if originalLine < 1 || originalLine >= len(mapping) {
		return 0, fmt.Errorf("line %d is outside original file range", originalLine)
	}
	if mapping[originalLine] == 0 {
		return 0, fmt.Errorf("line %d no longer exists after previous operations", originalLine)
	}
	return mapping[originalLine], nil
}
