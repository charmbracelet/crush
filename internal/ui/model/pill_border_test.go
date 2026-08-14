package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/x/ansi"
)

// roundedBorderRunes are chars that only appear when a pill has a visible
// rounded border.
const roundedBorderRunes = "╭╮╰╯"

func hasRoundedBorder(s string) bool {
	return strings.ContainsAny(s, roundedBorderRunes)
}

// queuePillHasBorder reports whether the "N Queued" pill is wrapped in a
// rounded border by checking the line directly above the queue label for a
// top border corner.
func queuePillHasBorder(view string) bool {
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "Queued") {
			continue
		}
		if i == 0 {
			return false
		}
		return strings.ContainsAny(lines[i-1], "╭╮")
	}
	return false
}

// TestQueuePillAlwaysHasBorder guards CHARM-1678: the queued-prompts pill must
// render with its rounded border regardless of panel expansion or which pill
// section is nominally focused.
func TestQueuePillAlwaysHasBorder(t *testing.T) {
	incompleteTodos := []session.Todo{{Content: "a", Status: session.TodoStatusPending}}

	cases := []struct {
		name           string
		expanded       bool
		focusedSection pillSection
		todos          []session.Todo
		queue          int
	}{
		{"collapsed only queue", false, pillSectionTodos, nil, 2},
		{"collapsed queue+todos", false, pillSectionTodos, incompleteTodos, 2},
		{"expanded queue focused", true, pillSectionQueue, nil, 2},
		{"expanded stale todos focus only queue", true, pillSectionTodos, nil, 2},
		{"expanded todos focused queue+todos", true, pillSectionTodos, incompleteTodos, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUI()
			u.session = &session.Session{ID: "s1", Todos: tc.todos}
			u.promptQueue = tc.queue
			u.pillsExpanded = tc.expanded
			u.focusedPillSection = tc.focusedSection
			u.updateLayoutAndSize()
			u.renderPills()

			if !hasRoundedBorder(u.pillsView) {
				t.Fatalf("expected a rounded border somewhere in pills view:\n%s", u.pillsView)
			}
			if !queuePillHasBorder(u.pillsView) {
				t.Fatalf("expected the queue pill to have a border:\n%s", u.pillsView)
			}
		})
	}
}

// TestEffectiveFocusedSectionFallsThrough verifies that a stale focused section
// (pointing at a section with no content) resolves to the section that still
// has content, so the expanded list stays populated.
func TestEffectiveFocusedSectionFallsThrough(t *testing.T) {
	cases := []struct {
		name     string
		stored   pillSection
		todos    []session.Todo
		queue    int
		expected pillSection
	}{
		{"todos focus but only queue", pillSectionTodos, nil, 2, pillSectionQueue},
		{"queue focus but only todos", pillSectionQueue, []session.Todo{{Content: "a", Status: session.TodoStatusPending}}, 0, pillSectionTodos},
		{"todos focus with todos", pillSectionTodos, []session.Todo{{Content: "a", Status: session.TodoStatusPending}}, 2, pillSectionTodos},
		{"queue focus with queue", pillSectionQueue, nil, 2, pillSectionQueue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUI()
			u.session = &session.Session{ID: "s1", Todos: tc.todos}
			u.promptQueue = tc.queue
			u.focusedPillSection = tc.stored
			if got := u.effectiveFocusedSection(); got != tc.expected {
				t.Fatalf("effectiveFocusedSection() = %d, want %d", got, tc.expected)
			}
		})
	}
}

// TestPillsRowPopHint verifies that the pills-row footer advertises the
// queued-message pop binding exactly while a queue exists — in both collapsed
// and expanded states — and omits it when only todos drive the pills row.
func TestPillsRowPopHint(t *testing.T) {
	todos := []session.Todo{{Content: "a", Status: session.TodoStatusPending}}

	cases := []struct {
		name           string
		expanded       bool
		focusedSection pillSection
		todos          []session.Todo
		queue          int
		wantHint       bool
	}{
		{"collapsed queue only", false, pillSectionQueue, nil, 2, true},
		{"expanded queue only", true, pillSectionQueue, nil, 2, true},
		{"collapsed queue+todos", false, pillSectionTodos, todos, 2, true},
		{"expanded todos focused queue+todos", true, pillSectionTodos, todos, 2, true},
		{"no queue collapsed", false, pillSectionTodos, todos, 0, false},
		{"no queue expanded", true, pillSectionTodos, todos, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUI()
			u.session = &session.Session{ID: "s1", Todos: tc.todos}
			u.promptQueue = tc.queue
			u.pillsExpanded = tc.expanded
			u.focusedPillSection = tc.focusedSection
			u.updateLayoutAndSize()
			u.renderPills()

			if !strings.Contains(u.pillsView, "ctrl+t") {
				t.Fatalf("expected the ctrl+t toggle hint in pills view:\n%s", u.pillsView)
			}
			hasHint := strings.Contains(u.pillsView, "shift/alt+up") &&
				strings.Contains(u.pillsView, "pop message")
			if hasHint != tc.wantHint {
				t.Fatalf("pop hint presence = %v, want %v:\n%s", hasHint, tc.wantHint, u.pillsView)
			}
		})
	}
}

// TestPillsRowEscapeQueueHint verifies the footer advertises the Escape
// queue move whenever a queue exists, with wording that matches the gesture:
// a single press while the agent is idle, the confirming press of the
// double-press cancel while it is busy. With no queue there is nothing to
// advertise.
func TestPillsRowEscapeQueueHint(t *testing.T) {
	cases := []struct {
		name     string
		queue    int
		busy     bool
		wantKey  string
		wantDesc string
		// absent pins wording the case must *not* render. The idle wording
		// is a substring of the busy wording ("esc esc" contains "esc",
		// "cancel + pop all messages" contains "pop all messages"), so
		// containment alone is satisfied by the busy rendering and the
		// idle/busy distinction could be deleted undetected.
		absent []string
	}{
		{"idle with queue", 2, false, "esc", "pop all messages", []string{"esc esc", "cancel +"}},
		{"busy with queue", 2, true, "esc esc", "cancel + pop all messages", nil},
		{"idle without queue", 0, false, "", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUI()
			u.session = &session.Session{
				ID:    "s1",
				Todos: []session.Todo{{Content: "a", Status: session.TodoStatusPending}},
			}
			u.promptQueue = tc.queue
			u.agentBusyCache.set(tc.busy)
			u.updateLayoutAndSize()
			u.renderPills()

			if tc.wantDesc == "" {
				if strings.Contains(u.pillsView, "pop all messages") {
					t.Fatalf("expected no escape hint without a queue:\n%s", u.pillsView)
				}
				return
			}
			if !strings.Contains(u.pillsView, tc.wantKey) {
				t.Fatalf("expected %q in pills view:\n%s", tc.wantKey, u.pillsView)
			}
			if !strings.Contains(u.pillsView, tc.wantDesc) {
				t.Fatalf("expected %q in pills view:\n%s", tc.wantDesc, u.pillsView)
			}
			for _, absent := range tc.absent {
				if strings.Contains(u.pillsView, absent) {
					t.Fatalf("expected %q to be absent from pills view:\n%s", absent, u.pillsView)
				}
			}
		})
	}
}

func TestExpandedQueueListEscapesControlsWithoutHidingItems(t *testing.T) {
	u := newTestUI()
	u.session = &session.Session{ID: "s1"}
	u.promptQueueItems = []string{
		"first line\r\nsecond line",
		"tab\tvalue",
		"message after controls",
	}
	u.promptQueue = len(u.promptQueueItems)
	u.pillsExpanded = true
	u.focusedPillSection = pillSectionQueue
	u.updateLayoutAndSize()
	u.renderPills()

	view := ansi.Strip(u.pillsView)
	var queueLines []string
	for line := range strings.SplitSeq(view, "\n") {
		if strings.Contains(line, "•") {
			queueLines = append(queueLines, line)
		}
	}

	if len(queueLines) != len(u.promptQueueItems) {
		t.Fatalf("rendered %d queue rows, want %d:\n%s",
			len(queueLines), len(u.promptQueueItems), view)
	}
	if !strings.Contains(queueLines[0], `first line\nsecond line`) {
		t.Fatalf("expected escaped newline in first queue row: %q", queueLines[0])
	}
	if !strings.Contains(queueLines[1], `tab\tvalue`) {
		t.Fatalf("expected escaped tab in second queue row: %q", queueLines[1])
	}
	if !strings.Contains(queueLines[2], "message after controls") {
		t.Fatalf("expected final queued message to remain visible: %q", queueLines[2])
	}
}

func TestQueueListFitsLiveContentWidth(t *testing.T) {
	styles := newTestUI().com.Styles
	item := "a queue message that must be truncated to fit the viewport"

	for _, width := range []int{0, 1, 8, 40, 80, 100, 140} {
		t.Run(fmt.Sprintf("width %d", width), func(t *testing.T) {
			view := queueList([]string{item}, styles, width)
			if got := ansi.StringWidth(view); got > width && width >= 4 {
				t.Fatalf("queue row width = %d, want <= %d: %q", got, width, ansi.Strip(view))
			}
			if width < 4 && ansi.Strip(view) != "  • " {
				t.Fatalf("narrow queue row = %q, want bullet only", ansi.Strip(view))
			}
			if width == 40 && !strings.HasSuffix(ansi.Strip(view), "…") {
				t.Fatalf("truncated queue row lacks ellipsis: %q", ansi.Strip(view))
			}
		})
	}
}
