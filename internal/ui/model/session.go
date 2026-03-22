package model

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent"
	agenttools "github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/chat"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/x/ansi"
)

// loadSessionMsg is a message indicating that a session and its files have
// been loaded.
type loadSessionMsg struct {
	session           *session.Session
	messages          []message.Message
	files             []SessionFile
	readFiles         []string
	selectedMessageID string
	childSessionInfo  map[string]childSessionInfo
}

// lspFilePaths returns deduplicated file paths from both modified and read
// files for starting LSP servers.
func (msg loadSessionMsg) lspFilePaths() []string {
	seen := make(map[string]struct{}, len(msg.files)+len(msg.readFiles))
	paths := make([]string, 0, len(msg.files)+len(msg.readFiles))
	for _, f := range msg.files {
		p := f.LatestVersion.Path
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	for _, p := range msg.readFiles {
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return paths
}

// SessionFile tracks the first and latest versions of a file in a session,
// along with the total additions and deletions.
type SessionFile struct {
	FirstVersion  history.File
	LatestVersion history.File
	Additions     int
	Deletions     int
}

var modifiedFilesRootMarkers = []string{
	".git",
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"AGENTS.md",
}

// loadSession loads the session along with its associated files and computes
// the diff statistics (additions and deletions) for each file in the session.
// It returns a tea.Cmd that, when executed, fetches the session data and
// returns a sessionFilesLoadedMsg containing the processed session files.
func (m *UI) loadSession(sessionID string) tea.Cmd {
	return m.loadSessionWithSelection(sessionID, "")
}

func (m *UI) loadSessionWithSelection(sessionID string, selectedMessageID string) tea.Cmd {
	return func() tea.Msg {
		session, err := m.com.App.Sessions.Get(context.Background(), sessionID)
		if err != nil {
			return util.ReportError(err)
		}

		sessionFiles, err := m.loadSessionFiles(sessionID)
		if err != nil {
			return util.ReportError(err)
		}

		readFiles, err := m.com.App.FileTracker.ListReadFiles(context.Background(), sessionID)
		if err != nil {
			slog.Error("Failed to load read files for session", "error", err)
		}

		msgs, err := m.com.App.Messages.List(context.Background(), sessionID)
		if err != nil {
			return util.ReportError(err)
		}

		childInfo := make(map[string]childSessionInfo)
		if session.ParentSessionID != "" {
			if info, ok := fetchChildSessionMetadata(m.com.App, session.ID); ok {
				childInfo[session.ID] = info
			}
		}

		return loadSessionMsg{
			session:           &session,
			messages:          msgs,
			files:             sessionFiles,
			readFiles:         readFiles,
			selectedMessageID: selectedMessageID,
			childSessionInfo:  childInfo,
		}
	}
}

func (m *UI) loadSessionFiles(sessionID string) ([]SessionFile, error) {
	files, err := m.com.App.History.ListBySession(context.Background(), sessionID)
	if err != nil {
		return nil, err
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
	return sessionFiles, nil
}

// handleFileEvent processes file change events and updates the session file
// list with new or updated file information.
func (m *UI) handleFileEvent(file history.File) tea.Cmd {
	if m.session == nil || file.SessionID != m.session.ID {
		return nil
	}

	return func() tea.Msg {
		sessionFiles, err := m.loadSessionFiles(m.session.ID)
		// could not load session files
		if err != nil {
			return util.NewErrorMsg(err)
		}

		return sessionFilesUpdatesMsg{
			sessionFiles: sessionFiles,
		}
	}
}

// filesInfo renders the modified files section for the sidebar, showing files
// with their addition/deletion counts.
func (m *UI) filesInfo(cwd string, width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.Subtle.Render("Modified Files")
	if isSection {
		title = common.Section(t, "Modified Files", width)
	}
	list := t.Subtle.Render("None")
	var filesWithChanges []SessionFile
	for _, f := range m.sessionFiles {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		filesWithChanges = append(filesWithChanges, f)
	}
	if len(filesWithChanges) > 0 {
		list = fileList(t, cwd, filesWithChanges, width, maxItems)
	}

	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// fileList renders a list of files with their diff statistics, truncating to
// maxItems and showing a "...and N more" message if needed.
func fileList(t *styles.Styles, cwd string, filesWithChanges []SessionFile, width, maxItems int) string {
	if maxItems <= 0 {
		return ""
	}
	var renderedFiles []string
	filesShown := 0

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

		// Format file path relative to the detected project root when possible.
		filePath := formatModifiedFilePath(cwd, f.FirstVersion.Path)
		filePath = compactModifiedFilePath(filePath, width-(lipgloss.Width(extraContent)-2))

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

func formatModifiedFilePath(cwd, filePath string) string {
	absPath := filePath
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(cwd, filePath)
	}
	absPath = filepath.Clean(absPath)

	if root, ok := detectModifiedFilesRoot(absPath); ok {
		if rel, ok := relativeDisplayPath(root, absPath); ok {
			return rel
		}
	}
	if rel, ok := relativeDisplayPath(cwd, absPath); ok {
		return rel
	}
	return fsext.PrettyPath(absPath)
}

func detectModifiedFilesRoot(filePath string) (string, bool) {
	startDir := filepath.Dir(filePath)
	for _, marker := range modifiedFilesRootMarkers {
		if found, ok := fsext.LookupClosest(startDir, marker); ok {
			if filepath.Base(found) == marker && marker != ".git" {
				return filepath.Dir(found), true
			}
			if marker == ".git" {
				info, err := os.Stat(found)
				if err == nil && info.IsDir() {
					return filepath.Dir(found), true
				}
			}
		}
	}
	return "", false
}

func relativeDisplayPath(root, filePath string) (string, bool) {
	if root == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, filePath)
	if err != nil {
		return "", false
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func compactModifiedFilePath(path string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(path) <= width {
		return path
	}

	cleanPath := filepath.Clean(path)
	sep := string(filepath.Separator)
	parts := strings.Split(cleanPath, sep)
	if len(parts) >= 2 {
		candidate := filepath.Join("…", parts[len(parts)-2], parts[len(parts)-1])
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	if len(parts) >= 1 {
		candidate := filepath.Join("…", parts[len(parts)-1])
		if lipgloss.Width(candidate) <= width {
			return candidate
		}
	}
	return ansi.Truncate(path, width, "…")
}

func (m *UI) childSessions(parentID string) ([]session.Session, error) {
	msgs, err := m.com.App.Messages.List(context.Background(), parentID)
	if err != nil {
		return nil, err
	}

	children := make([]session.Session, 0)
	for _, msg := range msgs {
		for _, tc := range msg.ToolCalls() {
			if !isChildSessionToolCall(tc.Name) {
				continue
			}
			childID := m.com.App.Sessions.CreateAgentToolSessionID(msg.ID, tc.ID)
			child, err := m.com.App.Sessions.Get(context.Background(), childID)
			if err != nil {
				continue
			}
			if child.ParentSessionID == parentID {
				children = append(children, child)
			}
		}
	}

	return children, nil
}

func (m *UI) sessionRoleLabel(sess *session.Session) string {
	if sess == nil || sess.ParentSessionID == "" {
		return "Main"
	}

	if m.childSessionInfoCache != nil {
		if info, ok := m.childSessionInfoCache[sess.ID]; ok {
			return info.RoleLabel
		}
	}

	return "Subagent"
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

type childSessionInfo struct {
	RoleLabel string
}

func fetchChildSessionMetadata(app *app.App, sessionID string) (childSessionInfo, bool) {
	messageID, toolCallID, ok := app.Sessions.ParseAgentToolSessionID(sessionID)
	if !ok {
		return childSessionInfo{}, false
	}

	msg, err := app.Messages.Get(context.Background(), messageID)
	if err != nil {
		return childSessionInfo{}, false
	}

	for _, tc := range msg.ToolCalls() {
		if tc.ID != toolCallID {
			continue
		}
		switch tc.Name {
		case agent.AgentToolName:
			var params agent.AgentParams
			if err := json.Unmarshal([]byte(tc.Input), &params); err != nil {
				return childSessionInfo{}, false
			}
			return childSessionInfo{
				RoleLabel: titleCase(config.CanonicalSubagentID(params.SubagentType)),
			}, true
		case agenttools.AgenticFetchToolName:
			return childSessionInfo{RoleLabel: "Fetch"}, true
		}
	}

	return childSessionInfo{}, false
}

func isChildSessionToolCall(toolName string) bool {
	switch toolName {
	case agent.AgentToolName, agenttools.AgenticFetchToolName:
		return true
	default:
		return false
	}
}

type openChildSessionMsg struct {
	sessionID string
}

func (m *UI) openSelectedChildSession() tea.Cmd {
	if m.session == nil {
		return nil
	}

	selected := m.chat.SelectedMessageItem()
	if selected == nil {
		return nil
	}

	toolItem, ok := selected.(chat.ToolMessageItem)
	if !ok || !isChildSessionToolCall(toolItem.ToolCall().Name) {
		return nil
	}

	childID := m.com.App.Sessions.CreateAgentToolSessionID(toolItem.MessageID(), toolItem.ToolCall().ID)
	parentSessionID := m.session.ID

	return func() tea.Msg {
		child, err := m.com.App.Sessions.Get(context.Background(), childID)
		if err != nil || child.ParentSessionID != parentSessionID {
			return nil
		}
		return openChildSessionMsg{sessionID: child.ID}
	}
}

func (m *UI) openParentSession() tea.Cmd {
	if m.session == nil || m.session.ParentSessionID == "" {
		return nil
	}

	_, toolCallID, ok := m.com.App.Sessions.ParseAgentToolSessionID(m.session.ID)
	if ok {
		return m.loadSessionWithSelection(m.session.ParentSessionID, toolCallID)
	}

	return m.loadSession(m.session.ParentSessionID)
}

func (m *UI) cycleSiblingChildSession(step int) tea.Cmd {
	if m.session == nil || m.session.ParentSessionID == "" {
		return nil
	}

	parentID := m.session.ParentSessionID
	currentID := m.session.ID

	return func() tea.Msg {
		children, err := m.childSessions(parentID)
		if err != nil {
			return util.ReportError(err)
		}
		if len(children) < 2 {
			return nil
		}

		currentIndex := -1
		for i, child := range children {
			if child.ID == currentID {
				currentIndex = i
				break
			}
		}
		if currentIndex == -1 {
			return nil
		}

		nextIndex := currentIndex + step
		if nextIndex < 0 || nextIndex >= len(children) {
			return nil
		}

		return openChildSessionMsg{sessionID: children[nextIndex].ID}
	}
}

// startLSPs starts LSP servers for the given file paths.
func (m *UI) startLSPs(paths []string) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		for _, path := range paths {
			m.com.App.LSPManager.Start(ctx, path)
		}
		return nil
	}
}
