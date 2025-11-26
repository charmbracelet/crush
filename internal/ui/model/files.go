package model

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// SessionFile tracks the first and latest versions of a file in a session,
// along with the total additions and deletions.
type SessionFile struct {
	FirstVersion  history.File
	LatestVersion history.File
	Additions     int
	Deletions     int
}

// loadSessionFiles loads all files modified during a session and calculates
// their diff statistics.
func (m *UI) loadSessionFiles(sessionID string) tea.Cmd {
	return func() tea.Msg {
		files, err := m.com.App.History.ListBySession(context.Background(), sessionID)
		if err != nil {
			return err
		}
		filesByPath := make(map[string][]history.File)
		for _, f := range files {
			filesByPath[f.Path] = append(filesByPath[f.Path], f)
		}

		sessionFiles := make([]SessionFile, 0, len(filesByPath))
		for _, versions := range filesByPath {
			if len(versions) == 0 {
				continue
			}

			first := versions[0]
			last := versions[0]
			for _, v := range versions {
				if v.Version < first.Version {
					first = v
				}
				if v.Version > last.Version {
					last = v
				}
			}

			_, additions, deletions := diff.GenerateDiff(first.Content, last.Content, first.Path)

			sessionFiles = append(sessionFiles, SessionFile{
				FirstVersion:  first,
				LatestVersion: last,
				Additions:     additions,
				Deletions:     deletions,
			})
		}

		slices.SortFunc(sessionFiles, func(a, b SessionFile) int {
			if a.LatestVersion.UpdatedAt > b.LatestVersion.UpdatedAt {
				return -1
			}
			if a.LatestVersion.UpdatedAt < b.LatestVersion.UpdatedAt {
				return 1
			}
			return 0
		})

		return sessionFilesLoadedMsg{
			files: sessionFiles,
		}
	}
}

// handleFileEvent processes file change events and updates the session file
// list with new or updated file information.
func (m *UI) handleFileEvent(file history.File) tea.Cmd {
	if m.session == nil || file.SessionID != m.session.ID {
		return nil
	}

	return func() tea.Msg {
		existingIdx := -1
		for i, sf := range m.sessionFiles {
			if sf.FirstVersion.Path == file.Path {
				existingIdx = i
				break
			}
		}

		if existingIdx == -1 {
			newFiles := make([]SessionFile, 0, len(m.sessionFiles)+1)
			newFiles = append(newFiles, SessionFile{
				FirstVersion:  file,
				LatestVersion: file,
				Additions:     0,
				Deletions:     0,
			})
			newFiles = append(newFiles, m.sessionFiles...)

			return sessionFilesLoadedMsg{files: newFiles}
		}

		updated := m.sessionFiles[existingIdx]

		if file.Version < updated.FirstVersion.Version {
			updated.FirstVersion = file
		}

		if file.Version > updated.LatestVersion.Version {
			updated.LatestVersion = file
		}

		_, additions, deletions := diff.GenerateDiff(
			updated.FirstVersion.Content,
			updated.LatestVersion.Content,
			updated.FirstVersion.Path,
		)
		updated.Additions = additions
		updated.Deletions = deletions

		newFiles := make([]SessionFile, 0, len(m.sessionFiles))
		newFiles = append(newFiles, updated)
		for i, sf := range m.sessionFiles {
			if i != existingIdx {
				newFiles = append(newFiles, sf)
			}
		}

		return sessionFilesLoadedMsg{files: newFiles}
	}
}

// filesInfo renders the modified files section for the sidebar, showing files
// with their addition/deletion counts.
func (m *UI) filesInfo(cwd string, width, maxItems int) string {
	t := m.com.Styles
	title := common.Section(t, "Modified Files", width)
	list := t.Subtle.Render("None")

	if len(m.sessionFiles) > 0 {
		list = fileList(t, cwd, m.sessionFiles, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// fileList renders a list of files with their diff statistics, truncating to
// maxItems and showing a "...and N more" message if needed.
func fileList(t *styles.Styles, cwd string, files []SessionFile, width, maxItems int) string {
	var renderedFiles []string
	filesShown := 0

	var filesWithChanges []SessionFile
	for _, f := range files {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		filesWithChanges = append(filesWithChanges, f)
	}

	for _, f := range filesWithChanges {
		// Skip files with no changes
		if filesShown >= maxItems {
			break
		}

		// Build stats string with colors
		var statusParts []string
		if f.Additions > 0 {
			statusParts = append(statusParts, t.Files.Additions.Render(fmt.Sprintf("+%d", f.Additions)))
		}
		if f.Deletions > 0 {
			statusParts = append(statusParts, t.Files.Deletions.Render(fmt.Sprintf("-%d", f.Deletions)))
		}
		extraContent := strings.Join(statusParts, " ")

		// Format file path
		filePath := f.FirstVersion.Path
		if rel, err := filepath.Rel(cwd, filePath); err == nil {
			filePath = rel
		}
		filePath = fsext.DirTrim(filePath, 2)
		filePath = ansi.Truncate(filePath, width-(lipgloss.Width(extraContent)-2), "…")

		line := t.Files.Path.Render(filePath)
		if extraContent != "" {
			line = fmt.Sprintf("%s %s", line, extraContent)
		}

		renderedFiles = append(renderedFiles, line)
		filesShown++
	}

	if len(filesWithChanges) > maxItems {
		remaining := len(filesWithChanges) - maxItems
		renderedFiles = append(renderedFiles, t.Subtle.Render(fmt.Sprintf("…and %d more", remaining)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, renderedFiles...)
}
