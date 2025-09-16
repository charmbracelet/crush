package files

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/nom-nom-hub/blush/internal/config"
	"github.com/nom-nom-hub/blush/internal/fsext"
	"github.com/nom-nom-hub/blush/internal/history"
	"github.com/nom-nom-hub/blush/internal/tui/styles"
)

// FileHistory represents a file history with initial and latest versions.
type FileHistory struct {
	InitialVersion history.File
	LatestVersion  history.File
}

// SessionFile represents a file with its history information.
type SessionFile struct {
	History   FileHistory
	FilePath  string
	Additions int
	Deletions int
}

// RenderOptions contains options for rendering file lists.
type RenderOptions struct {
	MaxWidth    int
	MaxItems    int
	ShowSection bool
	SectionName string
}

// RenderFileList renders a list of file status items with the given options.
func RenderFileList(fileSlice []SessionFile, opts RenderOptions) []string {
	t := styles.CurrentTheme()
	fileList := []string{}

	if opts.ShowSection {
		sectionName := opts.SectionName
		if sectionName == "" {
			sectionName = "Modified Files"
		}
		// Create a beautiful section header with decorative elements
		sectionStyle := t.S().Subtle.Bold(true).Foreground(t.Secondary)
		section := sectionStyle.Render(sectionName)
		fileList = append(fileList, section, "")
	}

	if len(fileSlice) == 0 {
		// Beautiful empty state with decorative elements
		emptyStyle := t.S().Base.Foreground(t.FgSubtle).Italic(true)
		emptyIcon := t.S().Base.Foreground(t.Border).Render("📂")
		emptyMessage := emptyStyle.Render("No files modified in this session")
		fileList = append(fileList, fmt.Sprintf("%s %s", emptyIcon, emptyMessage))
		return fileList
	}

	// Sort files by the latest version's created time
	sort.Slice(fileSlice, func(i, j int) bool {
		if fileSlice[i].History.LatestVersion.CreatedAt == fileSlice[j].History.LatestVersion.CreatedAt {
			return strings.Compare(fileSlice[i].FilePath, fileSlice[j].FilePath) < 0
		}
		return fileSlice[i].History.LatestVersion.CreatedAt > fileSlice[j].History.LatestVersion.CreatedAt
	})

	// Determine how many items to show
	maxItems := len(fileSlice)
	if opts.MaxItems > 0 {
		maxItems = min(opts.MaxItems, len(fileSlice))
	}

	filesShown := 0
	for _, file := range fileSlice {
		if file.Additions == 0 && file.Deletions == 0 {
			continue // skip files with no changes
		}
		if filesShown >= maxItems {
			break
		}

		// Beautiful status indicators with enhanced styling
		var statusParts []string
		if file.Additions > 0 {
			addStyle := t.S().Base.Foreground(t.Success).Bold(true)
			statusParts = append(statusParts, addStyle.Render(fmt.Sprintf("▲ +%d", file.Additions)))
		}
		if file.Deletions > 0 {
			delStyle := t.S().Base.Foreground(t.Error).Bold(true)
			statusParts = append(statusParts, delStyle.Render(fmt.Sprintf("▼ -%d", file.Deletions)))
		}

		extraContent := strings.Join(statusParts, " ")
		cwd := config.Get().WorkingDir() + string(os.PathSeparator)
		filePath := file.FilePath
		if rel, err := filepath.Rel(cwd, filePath); err == nil {
			filePath = rel
		}
		filePath = fsext.DirTrim(fsext.PrettyPath(filePath), 2)
		filePath = ansi.Truncate(filePath, opts.MaxWidth-lipgloss.Width(extraContent)-2, "…")

		// Enhanced file icons based on file extension with beautiful styling
		icon := "📄" // Default file icon
		iconStyle := t.S().Base.Foreground(t.FgSubtle)
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".go":
			icon = "🐹"
			iconStyle = t.S().Base.Foreground(t.Primary)
		case ".js", ".jsx":
			icon = "📜"
			iconStyle = t.S().Base.Foreground(t.Secondary)
		case ".ts", ".tsx":
			icon = "📘"
			iconStyle = t.S().Base.Foreground(t.Info)
		case ".py":
			icon = "🐍"
			iconStyle = t.S().Base.Foreground(t.Warning)
		case ".md":
			icon = "📝"
			iconStyle = t.S().Base.Foreground(t.Success)
		case ".json":
			icon = "📊"
			iconStyle = t.S().Base.Foreground(t.Info)
		case ".html":
			icon = "🌐"
			iconStyle = t.S().Base.Foreground(t.Error)
		case ".css", ".scss", ".sass":
			icon = "🎨"
			iconStyle = t.S().Base.Foreground(t.Primary)
		case ".sql":
			icon = "🗄️"
			iconStyle = t.S().Base.Foreground(t.Secondary)
		case ".yml", ".yaml":
			icon = "🔧"
			iconStyle = t.S().Base.Foreground(t.FgHalfMuted)
		case ".gitignore", ".env":
			icon = "⚙️"
			iconStyle = t.S().Base.Foreground(t.FgMuted)
		}

		// Create a beautiful file entry with enhanced visual styling
		iconPart := iconStyle.Render(icon)
		filePathStyle := t.S().Base.Foreground(t.FgBase)
		fileEntry := fmt.Sprintf("%s %s", iconPart, filePathStyle.Render(filePath))
		
		// Combine all parts in a beautiful layout
		if extraContent != "" {
			fileEntry = lipgloss.JoinHorizontal(
				lipgloss.Left,
				fileEntry,
				strings.Repeat(" ", max(0, opts.MaxWidth-lipgloss.Width(fileEntry)-lipgloss.Width(extraContent))),
				extraContent,
			)
		}
		
		fileList = append(fileList, fileEntry)
		filesShown++
	}

	return fileList
}

// RenderFileBlock renders a complete file block with optional truncation indicator.
func RenderFileBlock(fileSlice []SessionFile, opts RenderOptions, showTruncationIndicator bool) string {
	t := styles.CurrentTheme()
	fileList := RenderFileList(fileSlice, opts)

	// Add truncation indicator if needed
	if showTruncationIndicator && opts.MaxItems > 0 {
		totalFilesWithChanges := 0
		for _, file := range fileSlice {
			if file.Additions > 0 || file.Deletions > 0 {
				totalFilesWithChanges++
			}
		}
		if totalFilesWithChanges > opts.MaxItems {
			remaining := totalFilesWithChanges - opts.MaxItems
			if remaining == 1 {
				fileList = append(fileList, t.S().Base.Foreground(t.FgMuted).Render("…"))
			} else {
				fileList = append(fileList,
					t.S().Base.Foreground(t.FgSubtle).Render(fmt.Sprintf("…and %d more", remaining)),
				)
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, fileList...)
	if opts.MaxWidth > 0 {
		return lipgloss.NewStyle().Width(opts.MaxWidth).Render(content)
	}
	return content
}
