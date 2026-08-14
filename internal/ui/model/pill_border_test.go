package model

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
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

// TestPillsRowClearQueueHint verifies the footer advertises the queue
// clear only when esc actually clears: a queue must exist and the agent
// must be idle. While the agent is busy, esc cancels the active turn and
// deliberately preserves the queue, so advertising it there would point at
// a binding that does something else.
func TestPillsRowClearQueueHint(t *testing.T) {
	cases := []struct {
		name     string
		queue    int
		busy     bool
		wantHint bool
	}{
		{"idle with queue", 2, false, true},
		{"busy with queue", 2, true, false},
		{"idle without queue", 0, false, false},
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

			hasHint := strings.Contains(u.pillsView, "clear the queue")
			if hasHint != tc.wantHint {
				t.Fatalf("clear hint presence = %v, want %v:\n%s", hasHint, tc.wantHint, u.pillsView)
			}
		})
	}
}
