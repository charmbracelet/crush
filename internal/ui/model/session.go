package model

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sync/errgroup"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/agent"
	"github.com/charmbracelet/crush/internal/diff"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/pubsub"
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
	// seq is the load this message answers. A message whose load has been
	// superseded by a newer one is dropped; see [sessionLoad].
	seq       uint64
	session   *session.Session
	files     []SessionFile
	readFiles []string
	messages  []message.Message
	// dispatchBusy reports, keyed by the subagent session ID, whether the
	// subagent of each open latest dispatch is still working. A dispatch
	// missing here is treated as finished.
	dispatchBusy map[string]bool
}

// nestedTranscriptsMsg carries the transcripts of the agent sessions started
// by the tool calls of a loaded session. They are read after the session
// itself was applied, so they never hold back the top-level transcript.
type nestedTranscriptsMsg struct {
	// seq is the session load these transcripts belong to.
	seq uint64
	// transcripts holds the transcript of each nested agent session, keyed
	// by its session ID. A session whose read failed or was empty is absent.
	transcripts map[string][]message.Message
}

// sessionLoad tracks the latest session load the UI asked for. Only the
// latest load is applied, so a slow load of a session the user has since
// left can never replace the one they chose.
type sessionLoad struct {
	// seq identifies the latest load. It only grows.
	seq uint64
	// ctx is cancelled once the load is superseded or done, so its reads
	// stop early.
	ctx    context.Context
	cancel context.CancelFunc
	// nestedPending is set while the nested agent transcripts of the
	// applied load are being read.
	nestedPending bool
	// childEvents buffers the child-session message events that arrive
	// while nestedPending is set. The nested transcripts are a snapshot,
	// so these are replayed on top of them once they land; otherwise an
	// event between the read and the apply would be lost.
	childEvents []pubsub.Event[message.Message]
}

// nestedLoadConcurrency bounds the nested transcript reads of a session
// load that run at once. In-process they serialize on the agent's settle
// lock anyway; in client/server mode each one is a request to the server.
const nestedLoadConcurrency = 4

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

// loadSession loads the session along with its associated files and computes
// the diff statistics (additions and deletions) for each file in the session.
// It returns a tea.Cmd that, when executed, fetches the session data and
// returns a sessionFilesLoadedMsg containing the processed session files.
//
// The returned batch also reports the new current-session selection to
// the workspace so the server can update its per-client presence map.
// That report is fire-and-forget: errors are logged at debug and the
// UI never blocks on the call.
func (m *UI) loadSession(sessionID string) tea.Cmd {
	ctx, seq := m.beginSessionLoad()
	return m.sessionLoadCmd(ctx, seq, sessionID)
}

// beginSessionLoad starts a new session load, superseding and cancelling any
// load still in flight, and returns its context and sequence number. It must
// run on the event loop.
func (m *UI) beginSessionLoad() (context.Context, uint64) {
	m.abandonSessionLoad()
	ctx, cancel := context.WithCancel(context.Background())
	m.sessionLoad.ctx = ctx
	m.sessionLoad.cancel = cancel
	return ctx, m.sessionLoad.seq
}

// abandonSessionLoad supersedes and cancels any session load in flight, so
// none of its messages are applied.
func (m *UI) abandonSessionLoad() {
	if m.sessionLoad.cancel != nil {
		m.sessionLoad.cancel()
	}
	m.sessionLoad = sessionLoad{seq: m.sessionLoad.seq + 1}
}

// finishSessionLoad releases the context of the latest load once nothing
// more is read for it. Its sequence number stays current.
func (m *UI) finishSessionLoad() {
	if m.sessionLoad.cancel != nil {
		m.sessionLoad.cancel()
	}
	m.sessionLoad = sessionLoad{seq: m.sessionLoad.seq}
}

// sessionLoadCmd returns the commands of the session load seq, which read
// the session off the event loop. Unlike loadSession it touches no UI
// state, so it may be built inside another command.
func (m *UI) sessionLoadCmd(ctx context.Context, seq uint64, sessionID string) tea.Cmd {
	load := func() tea.Msg {
		// Off the event loop and in parallel: a long transcript takes hundreds of ms to read.
		var (
			sess         session.Session
			sessionFiles []SessionFile
			readFiles    []string
			messages     []message.Message
		)
		g, gctx := errgroup.WithContext(ctx)
		g.Go(func() (err error) {
			sess, err = m.com.Workspace.GetSession(gctx, sessionID)
			return err
		})
		g.Go(func() (err error) {
			sessionFiles, err = m.loadSessionFiles(sessionID)
			return err
		})
		g.Go(func() error {
			var err error
			if readFiles, err = m.com.Workspace.FileTrackerListReadFiles(gctx, sessionID); err != nil {
				slog.Error("Failed to load read files for session", "error", err)
			}
			return nil
		})
		g.Go(func() (err error) {
			messages, err = m.com.Workspace.ListMessages(gctx, sessionID)
			return err
		})
		if err := g.Wait(); err != nil {
			if ctx.Err() != nil {
				// A newer load superseded this one.
				return nil
			}
			return util.ReportError(err)
		}

		return loadSessionMsg{
			seq:          seq,
			session:      &sess,
			files:        sessionFiles,
			readFiles:    readFiles,
			messages:     messages,
			dispatchBusy: m.probeOpenDispatches(messages),
		}
	}
	return tea.Batch(load, m.refreshSkillOverrides(), m.reportCurrentSession(sessionID))
}

// loadNestedTranscriptsCmd returns the command that reads the nested agent
// transcripts of the session load just applied from msgs, or nil when no
// tool call in msgs shows nested tools. Until the transcripts land,
// child-session events are buffered so they can be replayed on top of
// them; see applyNestedTranscripts.
func (m *UI) loadNestedTranscriptsCmd(msgs []message.Message) tea.Cmd {
	ctx, seq := m.sessionLoad.ctx, m.sessionLoad.seq
	if ctx == nil || !hasNestedToolCalls(msgs) {
		m.finishSessionLoad()
		return nil
	}
	m.sessionLoad.nestedPending = true
	m.sessionLoad.childEvents = nil
	return func() tea.Msg {
		transcripts := m.loadNestedTranscripts(ctx, msgs)
		if ctx.Err() != nil {
			return nil
		}
		return nestedTranscriptsMsg{seq: seq, transcripts: transcripts}
	}
}

// bufferChildSessionEvent records a child-session message event while the
// nested transcripts of the applied load are being read, so it can be
// replayed once they land.
func (m *UI) bufferChildSessionEvent(event pubsub.Event[message.Message]) {
	if !m.sessionLoad.nestedPending {
		return
	}
	// Only events with tool calls or results change nested tools; see
	// handleChildSessionMessage.
	if len(event.Payload.ToolCalls()) == 0 && len(event.Payload.ToolResults()) == 0 {
		return
	}
	m.sessionLoad.childEvents = append(m.sessionLoad.childEvents, event)
}

// applyNestedTranscripts attaches the nested agent transcripts of the latest
// session load to the agent tool calls in the chat, then replays the
// child-session events that arrived while they were read. The transcripts
// are a snapshot taken at some point in that time; each event carries the
// whole message, so replaying them in order brings every nested tool up to
// date, including the calls and results the snapshot missed.
func (m *UI) applyNestedTranscripts(msg nestedTranscriptsMsg) tea.Cmd {
	if !m.sessionLoad.nestedPending || msg.seq != m.sessionLoad.seq {
		return nil
	}
	events := m.sessionLoad.childEvents
	m.finishSessionLoad()

	items := make([]chat.MessageItem, 0, m.chat.Len())
	for i := range m.chat.Len() {
		if item := m.chat.MessageItemAt(i); item != nil {
			items = append(items, item)
		}
	}
	m.loadNestedToolCalls(items, msg.transcripts)
	for _, item := range items {
		if _, ok := item.(chat.NestedToolContainer); ok {
			m.chat.UpdateNestedToolIDs(item.ID())
		}
	}

	var cmds []tea.Cmd
	for _, event := range events {
		if cmd := m.handleChildSessionMessage(event); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if m.chat.Follow() {
		m.chat.ScrollToBottom()
	}
	return tea.Batch(cmds...)
}

// hasNestedToolCalls reports whether any tool call in msgs shows the tool
// calls of an agent session it started.
func hasNestedToolCalls(msgs []message.Message) bool {
	for i := range msgs {
		if msgs[i].Role != message.Assistant {
			continue
		}
		for _, tc := range msgs[i].ToolCalls() {
			if chat.HasNestedTools(tc.Name) {
				return true
			}
		}
	}
	return false
}

// nestedSessionIDs returns the agent session IDs started by the tool calls
// in msgs that show nested tools, in transcript order.
func (m *UI) nestedSessionIDs(msgs []message.Message) []string {
	var ids []string
	for i := range msgs {
		if msgs[i].Role != message.Assistant {
			continue
		}
		for _, tc := range msgs[i].ToolCalls() {
			if chat.HasNestedTools(tc.Name) {
				ids = append(ids, m.com.Workspace.CreateAgentToolSessionID(msgs[i].ID, tc.ID))
			}
		}
	}
	return ids
}

// loadNestedTranscripts reads the transcripts of the agent sessions started
// by the tool calls in msgs, then those started inside them, level by level.
// It runs inside a command, off the event loop. A read that fails or comes
// back empty is skipped, so its tool shows no nested tools. Once ctx is
// cancelled no further level is read.
func (m *UI) loadNestedTranscripts(ctx context.Context, msgs []message.Message) map[string][]message.Message {
	ws := m.com.Workspace
	seen := make(map[string]struct{})
	var nested map[string][]message.Message
	level := m.nestedSessionIDs(msgs)
	for len(level) > 0 && ctx.Err() == nil {
		ids := make([]string, 0, len(level))
		for _, id := range level {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
		fetched := make([][]message.Message, len(ids))
		var g errgroup.Group
		g.SetLimit(nestedLoadConcurrency)
		for i, id := range ids {
			g.Go(func() error {
				transcript, err := ws.ListMessages(ctx, id)
				if err != nil {
					if ctx.Err() != nil {
						return nil
					}
					slog.Debug("Failed to load nested agent messages", "session", id, "error", err)
					return nil
				}
				fetched[i] = transcript
				return nil
			})
		}
		_ = g.Wait()

		level = nil
		for i, id := range ids {
			if len(fetched[i]) == 0 {
				continue
			}
			if nested == nil {
				nested = make(map[string][]message.Message)
			}
			nested[id] = fetched[i]
			level = append(level, m.nestedSessionIDs(fetched[i])...)
		}
	}
	return nested
}

// probeOpenDispatches asks the backend, for the latest open dispatch of each
// subagent label in msgs, whether its subagent is still working. It runs
// inside a command, off the event loop; see replayDispatchReports.
func (m *UI) probeOpenDispatches(msgs []message.Message) map[string]bool {
	ptrs := make([]*message.Message, len(msgs))
	for i := range msgs {
		ptrs[i] = &msgs[i]
	}
	open := make(map[string]bool)
	for _, state := range agent.LatestDispatches(ptrs) {
		if state.Open {
			open[state.CallID] = true
		}
	}
	if len(open) == 0 {
		return nil
	}
	ws := m.com.Workspace
	busy := make(map[string]bool, len(open))
	for i := range msgs {
		if msgs[i].Role != message.Assistant {
			continue
		}
		for _, tc := range msgs[i].ToolCalls() {
			if !open[tc.ID] {
				continue
			}
			id := ws.CreateAgentToolSessionID(msgs[i].ID, tc.ID)
			busy[id] = ws.AgentIsSessionBusy(id)
		}
	}
	return busy
}

// reportCurrentSession returns a fire-and-forget tea.Cmd that
// informs the workspace which session this client is currently
// viewing. In client/server mode the report scopes the event stream:
// permission and question prompts are only delivered to clients
// viewing the session that raised them. Errors are logged at debug
// only; the call is best-effort, not correctness-critical state.
func (m *UI) reportCurrentSession(sessionID string) tea.Cmd {
	return func() tea.Msg {
		if err := m.com.Workspace.SetCurrentSession(context.Background(), sessionID); err != nil {
			slog.Debug("Failed to report current session", "session_id", sessionID, "error", err)
		}
		return nil
	}
}

func (m *UI) loadSessionFiles(sessionID string) ([]SessionFile, error) {
	files, err := m.com.Workspace.ListSessionHistory(context.Background(), sessionID)
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

	title := t.Files.SectionTitle.Render("Modified Files")
	if isSection {
		title = common.Section(t, "Modified Files", width)
	}
	list := t.Files.EmptyMessage.Render("None")
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
	var renderedFiles []string

	for _, f := range filesWithChanges {
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
		suffix := ""
		if extraContent != "" {
			suffix = " " + extraContent
		}
		maxPathWidth := max(width-lipgloss.Width(suffix), 0)
		filePath = ansi.Truncate(filePath, maxPathWidth, "…")

		line := t.Files.Path.Render(filePath)
		if extraContent != "" {
			line = fmt.Sprintf("%s %s", line, extraContent)
		}

		renderedFiles = append(renderedFiles, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, truncateRows(renderedFiles, maxItems, t.Files.TruncationHint)...)
}

// newSessionFilePaths returns the paths in updated that are not in previous.
// LSPs were already started for the previous files, so only the new ones
// need a start.
func newSessionFilePaths(previous, updated []SessionFile) []string {
	known := make(map[string]struct{}, len(previous))
	for _, f := range previous {
		known[f.LatestVersion.Path] = struct{}{}
	}
	var paths []string
	for _, f := range updated {
		if _, ok := known[f.LatestVersion.Path]; ok {
			continue
		}
		paths = append(paths, f.LatestVersion.Path)
	}
	return paths
}

// startLSPs starts LSP servers for the given file paths.
func (m *UI) startLSPs(paths []string) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}

	return func() tea.Msg {
		ctx := context.Background()
		for _, path := range paths {
			m.com.Workspace.LSPStart(ctx, path)
		}
		return nil
	}
}
