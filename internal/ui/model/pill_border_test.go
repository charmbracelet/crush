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
