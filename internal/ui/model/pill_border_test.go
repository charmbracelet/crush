package model

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/scheduler"
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
// render with its rounded border regardless of panel expansion.
func TestQueuePillAlwaysHasBorder(t *testing.T) {
	incompleteTodos := []session.Todo{{Content: "a", Status: session.TodoStatusPending}}

	cases := []struct {
		name     string
		expanded bool
		todos    []session.Todo
		queue    int
	}{
		{"collapsed only queue", false, nil, 2},
		{"collapsed queue+todos", false, incompleteTodos, 2},
		{"expanded only queue", true, nil, 2},
		{"expanded queue+todos", true, incompleteTodos, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUI()
			u.session = &session.Session{ID: "s1", Todos: tc.todos}
			u.promptQueue = tc.queue
			u.pillsExpanded = tc.expanded
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

// TestExpandedPillsShowEverySection verifies that expanding the panel lists
// every section that has content, not just one of them: with todos, queued
// prompts and scheduled tasks all present, ctrl+t must reveal all three lists.
func TestExpandedPillsShowEverySection(t *testing.T) {
	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Content: "write the todo", Status: session.TodoStatusPending},
	}}
	u.promptQueue = 1
	u.promptQueueItems = []string{"queued prompt"}
	u.cronTasks = []scheduler.Task{{
		ID:        "abc12345",
		Prompt:    "scheduled prompt",
		NextRunAt: time.Now().Add(time.Hour),
	}}
	u.pillsExpanded = true
	u.updateLayoutAndSize()
	u.renderPills()

	for _, want := range []string{"write the todo", "queued prompt", "scheduled prompt"} {
		if !strings.Contains(u.pillsView, want) {
			t.Fatalf("expected expanded pills to contain %q:\n%s", want, u.pillsView)
		}
	}
}

// TestPillsAreaHeightSumsExpandedSections verifies the reserved height accounts
// for every expanded list, so the stacked sections are not clipped.
func TestPillsAreaHeightSumsExpandedSections(t *testing.T) {
	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Content: "a", Status: session.TodoStatusPending},
		{Content: "b", Status: session.TodoStatusPending},
	}}
	u.promptQueue = 3
	u.cronTasks = []scheduler.Task{
		{ID: "t1", NextRunAt: time.Now()},
		{ID: "t2", NextRunAt: time.Now()},
	}

	if got, want := u.pillsAreaHeight(), pillHeightWithBorder; got != want {
		t.Fatalf("collapsed pillsAreaHeight() = %d, want %d", got, want)
	}

	u.pillsExpanded = true
	if got, want := u.pillsAreaHeight(), pillHeightWithBorder+2+3+2; got != want {
		t.Fatalf("expanded pillsAreaHeight() = %d, want %d", got, want)
	}
}
