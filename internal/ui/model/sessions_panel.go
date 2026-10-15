package model

// Sessions panel: the project's sessions with a live status glyph, shown in
// the chat sidebar and on the landing screen. The list and the per-session
// status probes are synchronous round-trips in client/server mode, so they
// follow the off-thread pattern of workspace_cache.go and mcp.go: events
// request a refresh (queued while one is in flight), fetches are spaced by
// sessionListMinInterval, a TTL backstop catches sessions created elsewhere
// (other processes, scripts), and results land as sessionListMsg.

import (
	"cmp"
	"context"
	"fmt"
	"image/color"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

// sessionsPanelMax is how many sessions the panel numbers (1..9).
const sessionsPanelMax = 9

// childRunsShown is how many sub-agent rows a session shows before "+k more".
const childRunsShown = 3

// childRunMaxUnresolved is how many list landings (failed ones too) a
// sub-agent may stay without a known parent (a failed lookup) before it is
// forgotten; its next message looks it up again. It bounds both the map and
// the lookups.
const childRunMaxUnresolved = 2

// sessionListTTL is the backstop refresh interval for the session list.
// Package var so tests can pin it.
var sessionListTTL = 2 * time.Second

// sessionListMinInterval spaces fetches, so a burst of events (every agent
// step publishes some) costs one fetch instead of one per event.
var sessionListMinInterval = 250 * time.Millisecond

// sessionListMsg delivers an off-thread session list fetch and the live
// status of the sessions the panel shows.
type sessionListMsg struct {
	// gen is the tracker version captured at dispatch. When a mark changed
	// while the fetch was in flight, the probes are older than that mark.
	gen      uint64
	sessions []session.Session
	status   map[string]workspace.SessionStatus
	err      error
}

// sessionsPanelKey is everything the rendered panel depends on besides the
// styles, whose changes clear the memo (refreshStyles).
type sessionsPanelKey struct {
	width, maxItems          int
	isSection                bool
	current                  string
	list, activity, children uint64
}

// requestSessionListRefresh asks for a fresh list. While a fetch is in
// flight it only queues one; applySessionList dispatches it on landing.
func (m *UI) requestSessionListRefresh() tea.Cmd {
	if m.sessionListInFlight {
		m.sessionListQueued = true
		return nil
	}
	return m.dispatchSessionListRefresh()
}

// dispatchSessionListRefresh returns a command that lists the sessions and
// probes the status of the ones the panel shows, off the Update goroutine.
// It returns nil while a fetch is in flight. The closure captures only
// locals (never m) so it is safe off-thread.
func (m *UI) dispatchSessionListRefresh() tea.Cmd {
	if m.sessionListInFlight || m.com == nil || m.com.Workspace == nil {
		return nil
	}
	m.sessionListInFlight = true
	m.sessionListQueued = false
	ws := m.com.Workspace
	gen := m.activity.version
	currentID := m.currentSessionID()
	// Session đang chạy/chờ/có kết quả chưa xem cũng được dò dù nằm ngoài 9
	// dòng đầu. Chụp lúc ra lệnh vì closure chạy ngoài luồng Update.
	pinned := m.activity.Active()
	wait := sessionListMinInterval - time.Since(m.sessionListFetchedAt)
	m.sessionListFetchedAt = time.Now().Add(max(wait, 0))
	return func() tea.Msg {
		time.Sleep(wait) // Returns at once when wait <= 0.
		// A hung server must not keep the fetch in flight forever.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		sessions, err := ws.ListSessions(ctx)
		if err != nil {
			return sessionListMsg{gen: gen, err: err}
		}
		// Sorted here, not on landing, so the probed sessions are the
		// shown ones.
		sortSessionsForPanel(sessions)
		shown := panelSessionsOf(sessions, currentID, pinned...)
		status := make(map[string]workspace.SessionStatus, len(shown))
		for _, s := range shown {
			status[s.ID] = ws.AgentSessionStatus(s.ID)
		}
		return sessionListMsg{gen: gen, sessions: sessions, status: status}
	}
}

// applySessionList stores an off-thread fetch and corrects the activity
// tracker from it. Runs on the Update goroutine.
func (m *UI) applySessionList(msg sessionListMsg) []tea.Cmd {
	var cmds []tea.Cmd
	m.sessionListInFlight = false
	m.sessionListCheckedAt = time.Now()
	if msg.err != nil {
		slog.Debug("Failed to list sessions", "error", msg.err)
		m.pruneChildRuns(nil)
	} else {
		// A mark set while this fetch was in flight is newer than its
		// probes, which would undo it: keep the list, skip the probes and
		// fetch again.
		fresh := msg.gen == m.activity.version
		ids := make(map[string]struct{}, len(msg.sessions))
		for _, s := range msg.sessions {
			ids[s.ID] = struct{}{}
		}
		m.activity.Retain(ids)
		m.sessionList = msg.sessions
		m.sessionListVersion++
		m.pruneStaleDrafts()
		m.pruneChildRuns(ids)
		m.refreshJobsTitles()
		if fresh {
			if cmd := m.probedWaitingInfo(msg.status); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.activity.Reconcile(msg.status)
			// Backstop for a missed run end (dropped event stream): an
			// idle parent has no running sub-agents. A failed probe is
			// not idle.
			for id, st := range msg.status {
				if st.Known && !st.Busy {
					m.dropChildRuns(id)
				}
			}
		} else {
			m.sessionListQueued = true
		}
	}
	if !m.sessionListQueued {
		return cmds
	}
	if cmd := m.dispatchSessionListRefresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// sortSessionsForPanel orders sessions newest-created first. ListSessions
// sorts by last update, which changes every agent step and would renumber
// the rows; creation time does not. The ID breaks ties within a second.
func sortSessionsForPanel(sessions []session.Session) {
	slices.SortStableFunc(sessions, func(a, b session.Session) int {
		return cmp.Or(cmp.Compare(b.CreatedAt, a.CreatedAt), strings.Compare(a.ID, b.ID))
	})
}

// inSessionList reports whether id is a listed (top-level) session.
func (m *UI) inSessionList(id string) bool {
	for _, s := range m.sessionList {
		if s.ID == id {
			return true
		}
	}
	return false
}

// panelSessions returns the sessions the panel shows.
func (m *UI) panelSessions() []session.Session {
	return panelSessionsOf(m.sessionList, m.currentSessionID(), m.activity.Active()...)
}

// jumpSessionN returns n (1..9) when msg is the nth key of the jump binding,
// else 0. The index in the key list is the number, so user overrides keep
// working.
func (m *UI) jumpSessionN(msg tea.KeyPressMsg) int {
	b := m.keyMap.Chat.JumpSession
	if !key.Matches(msg, b) {
		return 0
	}
	if i := slices.Index(b.Keys(), msg.String()); i < sessionsPanelMax {
		return i + 1
	}
	return 0
}

// jumpToSession loads the nth session of the Sessions panel (1-based). It
// does nothing for a number past the list or for the session already shown,
// which also covers the pinned row beyond the ninth: it is always the
// current session.
func (m *UI) jumpToSession(n int) tea.Cmd {
	shown := m.panelSessions()
	if n < 1 || n > len(shown) || shown[n-1].ID == m.currentSessionID() {
		return nil
	}
	return m.loadSession(shown[n-1].ID)
}

// cyclePendingTTL là thời hạn của đích Alt+. / Alt+, đang nạp dở: quá hạn mà
// session vẫn chưa hiện (nạp treo) thì lần bấm kế lại tính từ session đang xem.
const cyclePendingTTL = 5 * time.Second

// sessionCycle nhớ đích của lần Alt+. / Alt+, đang nạp dở. Bấm tiếp trước khi
// lần nạp xong phải đi tiếp từ đích đó, không đi lại từ session còn đang hiện.
type sessionCycle struct {
	pending string
	at      time.Time
}

// base là session để tính bước kế: đích đang nạp dở nếu còn mới, không thì
// session đang hiện.
func (c *sessionCycle) base(current string) string {
	if c.pending != "" && time.Since(c.at) < cyclePendingTTL {
		return c.pending
	}
	return current
}

func (c *sessionCycle) start(id string) {
	c.pending, c.at = id, time.Now()
}

// done quên đích chờ khi chính nó đã hiện hoặc nạp hỏng. Một lần nạp khác
// không đụng tới đích mới hơn đang chờ.
func (c *sessionCycle) done(id string) {
	if c.pending == id {
		*c = sessionCycle{}
	}
}

// cycleSession mở session kế (step=1) hoặc trước (step=-1) theo đúng danh sách
// bảng Sessions đang hiện (9 session mới nhất, session đang xem và session
// đang bận), vòng lại ở hai đầu (Alt+. / Alt+,). Dùng danh sách đã nạp sẵn nên
// không làm IO trong Update. Không còn session nào khác thì báo, không im lặng.
func (m *UI) cycleSession(step int) tea.Cmd {
	id := nextSessionID(m.panelSessions(), m.cycle.base(m.currentSessionID()), step)
	if id == "" {
		return util.ReportInfo("No other session")
	}
	m.cycle.start(id)
	return m.loadSession(id)
}

// nextSessionID trả về ID của session cách currentID một bước step trong
// list, vòng lại ở hai đầu. Session hiện tại chưa nằm trong list (chưa lưu)
// thì bắt đầu từ đầu hoặc cuối list; không có nơi để đi thì trả "".
func nextSessionID(list []session.Session, currentID string, step int) string {
	n := len(list)
	i := slices.IndexFunc(list, func(s session.Session) bool { return s.ID == currentID })
	switch {
	case n == 0, i >= 0 && n == 1:
		return ""
	case i < 0 && step > 0:
		return list[0].ID
	case i < 0:
		return list[n-1].ID
	}
	return list[((i+step)%n+n)%n].ID
}

// panelSessionsOf returns the first sessionsPanelMax sessions, plus the viewed
// one and every session in pinned (the busy or unseen ones) as extra entries
// when they fall outside them. The extras keep the order of list, so a pin
// never renumbers the rows.
func panelSessionsOf(list []session.Session, currentID string, pinned ...string) []session.Session {
	if len(list) <= sessionsPanelMax {
		return list
	}
	// Cap the capacity so the append below copies instead of overwriting
	// list[sessionsPanelMax].
	shown := list[:sessionsPanelMax:sessionsPanelMax]
	for _, s := range list[sessionsPanelMax:] {
		if s.ID == currentID || slices.Contains(pinned, s.ID) {
			shown = append(shown, s)
		}
	}
	return shown
}

// sessionsInfo renders the Sessions section, one "<n> <glyph> <title>" row
// per session. It renders from memoized state only, and the sidebar calls it
// on every Draw, so the output is memoized too.
func (m *UI) sessionsInfo(width, maxItems int, isSection bool) string {
	key := sessionsPanelKey{
		width: width, maxItems: maxItems, isSection: isSection,
		current: m.currentSessionID(), list: m.sessionListVersion, activity: m.activity.version,
		children: m.childRunsVersion,
	}
	if m.sessionsPanelOut == "" || key != m.sessionsPanelKey {
		m.sessionsPanelKey, m.sessionsPanelOut = key, m.renderSessionsInfo(width, maxItems, isSection)
	}
	return m.sessionsPanelOut
}

func (m *UI) renderSessionsInfo(width, maxItems int, isSection bool) string {
	t := m.com.Styles

	title := t.Resource.Heading.Render("Sessions")
	if isSection {
		title = common.Section(t, title, width)
	}

	sessions := m.panelSessions()
	if len(sessions) == 0 || maxItems <= 0 {
		list := t.Resource.AdditionalText.Render("None")
		return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
	}

	visible := sessions
	if len(sessions) > maxItems {
		visible = sessions[:maxItems-1]
	}
	// Row numbers name the jump keys: only rows a key reaches get one, and
	// with none bound (the default) there is no number column.
	jumpKeys := 0
	if b := m.keyMap.Chat.JumpSession; b.Enabled() {
		jumpKeys = min(len(b.Keys()), sessionsPanelMax)
	}
	current := m.currentSessionID()
	rows := make([]string, 0, maxItems)
	for i, s := range visible {
		icon := m.sessionGlyph(s.ID)
		if jumpKeys > 0 {
			num := "·"
			if i < jumpKeys {
				num = strconv.Itoa(i + 1)
			}
			icon = t.Resource.AdditionalText.Render(num) + " " + icon
		}
		var titleColor color.Color
		if s.ID == current {
			titleColor = t.Resource.RowTitleBase.GetForeground()
		}
		word := activityWord(m.activity.Get(s.ID))
		rows = append(rows, common.Status(t, common.StatusOpts{
			Icon:         icon,
			Title:        ansi.Truncate(s.Title, max(0, width-lipgloss.Width(icon)-1-wordWidth(word)), "…"),
			TitleColor:   titleColor,
			ExtraContent: statusTag(t.Resource.AdditionalText.Render, word),
		}, width))
		rows = append(rows, m.childRunRows(s.ID, width)...)
	}
	if len(visible) < len(sessions) {
		rows = append(rows, t.Resource.AdditionalText.Render(fmt.Sprintf("…and %d more", len(sessions)-len(visible))))
	}
	list := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().Width(width).Render(fmt.Sprintf("%s\n\n%s", title, list))
}

// activityWord is the short text shown beside the glyph so a state does not
// rest on color alone. Idle has none, to keep rows clean. The tracker has no
// unknown state: a failed probe leaves the last known one in place.
func activityWord(a sessionActivity) string {
	switch a {
	case activityWorking:
		return "running"
	case activityWaiting:
		return "waiting"
	case activityDoneUnseen:
		return "done"
	case activityFailedUnseen:
		return "failed"
	}
	return ""
}

// wordWidth is the columns a status word takes after the title, with its
// separating space.
func wordWidth(word string) int {
	if word == "" {
		return 0
	}
	return len(word) + 1
}

// statusTag styles a status word, or returns "" for none.
func statusTag(style func(...string) string, word string) string {
	if word == "" {
		return ""
	}
	return style(word)
}

// sessionGlyph renders a session's activity glyph in its theme color.
func (m *UI) sessionGlyph(id string) string {
	t := m.com.Styles
	glyph, c := "○", t.Resource.DefaultTitleFg // fgMoreSubtle
	switch m.activity.Get(id) {
	case activityWorking:
		glyph, c = "●", t.Resource.BusyIcon.GetForeground() // busy
	case activityWaiting:
		glyph, c = "▲", t.Status.WarnIndicator.GetBackground() // warning
	case activityDoneUnseen:
		glyph, c = "✓", t.Tool.JobIconSuccess.GetForeground() // success
	case activityFailedUnseen:
		glyph, c = "✗", t.Tool.JobIconError.GetForeground() // error
	}
	return lipgloss.NewStyle().Foreground(c).Render(glyph)
}

// childRun is a running sub-agent (a child session of the agent tool),
// shown as an indented row under the top-level session it belongs to.
type childRun struct {
	id, parentID, title string
	// age counts the list landings seen without a parent.
	age int
}

// childParentMsg delivers the off-thread resolution of a sub-agent session
// to its top-level session.
type childParentMsg struct {
	id, parentID, title string
}

// noteChildActivity tracks running sub-agents from their message events, so
// it sees the sub-agents of other processes too. The first message of an
// unlisted agent-tool session starts tracking it and resolves its parent
// off-thread, once per run; a final assistant message ends it.
func (m *UI) noteChildActivity(msg message.Message) tea.Cmd {
	id := msg.SessionID
	_, tracked := m.childRuns[id]
	if msg.Role == message.Assistant && msg.IsFinished() && msg.FinishReason() != message.FinishReasonToolUse {
		if tracked {
			m.dropChildRuns(id)
		}
		return nil
	}
	// Parse first: it is O(1), the list scan runs on every streamed token.
	if tracked || !m.isChildSession(id) || m.inSessionList(id) {
		return nil
	}
	if m.childRuns == nil {
		m.childRuns = make(map[string]*childRun)
	}
	// Hidden until childParentMsg sets the parent.
	m.childRuns[id] = &childRun{id: id}
	return m.resolveChildParent(id)
}

// isChildSession reports whether id is a sub-agent (agent tool) session.
// String parsing, no IO.
func (m *UI) isChildSession(id string) bool {
	if m.com == nil || m.com.Workspace == nil {
		return false
	}
	_, _, ok := m.com.Workspace.ParseAgentToolSessionID(id)
	return ok
}

// noteChildRename shows a sub-agent's generated title, which lands as a
// session update seconds after its run started.
func (m *UI) noteChildRename(s session.Session) {
	if c := m.childRuns[s.ID]; c != nil && c.title != s.Title {
		c.title = s.Title
		m.childRunsVersion++
	}
}

// resolveChildParent looks up a sub-agent session's title and top-level
// session off the Update goroutine: the child's own record, then
// RootSessionID from its parent. The closure captures only locals.
func (m *UI) resolveChildParent(id string) tea.Cmd {
	ws := m.com.Workspace
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		child, err := ws.GetSession(ctx, id)
		if err != nil {
			slog.Debug("Failed to get sub-agent session", "session_id", id, "error", err)
			return childParentMsg{id: id}
		}
		// On error this is the direct parent, still the right row when it
		// is top-level, as sub-agents are today.
		root, err := workspace.RootSessionID(ctx, ws, child.ParentSessionID)
		if err != nil {
			slog.Debug("Failed to resolve sub-agent root session", "session_id", id, "error", err)
		}
		return childParentMsg{id: id, parentID: root, title: child.Title}
	}
}

// applyChildParent stores a resolution, unless the run ended meanwhile. A
// title set by a rename that landed first is newer than the lookup's.
func (m *UI) applyChildParent(msg childParentMsg) {
	if c := m.childRuns[msg.id]; c != nil {
		c.parentID, c.title = msg.parentID, cmp.Or(c.title, msg.title)
		m.childRunsVersion++
	}
}

// pruneChildRuns forgets runs that cannot show: those still without a
// parent after childRunMaxUnresolved list landings, and those under a
// session no longer listed. listed is nil when the list failed. Neither
// renders, so no version bump.
func (m *UI) pruneChildRuns(listed map[string]struct{}) {
	for id, c := range m.childRuns {
		if c.parentID == "" {
			c.age++
			if c.age >= childRunMaxUnresolved {
				delete(m.childRuns, id)
			}
			continue
		}
		if _, ok := listed[c.parentID]; listed != nil && !ok {
			delete(m.childRuns, id)
		}
	}
}

// dropChildRuns forgets the sub-agent run of session id and the runs under it.
func (m *UI) dropChildRuns(id string) {
	for cid, c := range m.childRuns {
		if cid == id || c.parentID == id {
			delete(m.childRuns, cid)
			m.childRunsVersion++
		}
	}
}

// childRunRows renders the running sub-agents of a session as indented
// "└ ● <title>" rows, at most childRunsShown plus a count of the rest.
func (m *UI) childRunRows(parentID string, width int) []string {
	var kids []*childRun
	for _, c := range m.childRuns {
		if c.parentID == parentID {
			kids = append(kids, c)
		}
	}
	if len(kids) == 0 {
		return nil
	}
	slices.SortFunc(kids, func(a, b *childRun) int { return strings.Compare(a.id, b.id) })
	t := m.com.Styles
	icon := t.Resource.AdditionalText.Render("  └") + " " + t.Resource.BusyIcon.String()
	word := activityWord(activityWorking)
	var rows []string
	for _, c := range kids[:min(len(kids), childRunsShown)] {
		rows = append(rows, common.Status(t, common.StatusOpts{
			Icon:         icon,
			Title:        ansi.Truncate(c.title, max(0, width-lipgloss.Width(icon)-1-wordWidth(word)), "…"),
			ExtraContent: statusTag(t.Resource.AdditionalText.Render, word),
		}, width))
	}
	if more := len(kids) - childRunsShown; more > 0 {
		rows = append(rows, t.Resource.AdditionalText.Render(fmt.Sprintf("  └ +%d more", more)))
	}
	return rows
}
