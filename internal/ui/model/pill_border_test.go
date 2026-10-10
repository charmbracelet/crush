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
// render with its rounded border regardless of panel expansion or which pill
// section is focused.
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
		cron     []scheduler.Task
		expected pillSection
	}{
		{"todos focus but only queue", pillSectionTodos, nil, 2, nil, pillSectionQueue},
		{"queue focus but only todos", pillSectionQueue, []session.Todo{{Content: "a", Status: session.TodoStatusPending}}, 0, nil, pillSectionTodos},
		{"queue focus but only cron", pillSectionQueue, nil, 0, []scheduler.Task{{ID: "t1"}}, pillSectionCron},
		{"todos focus with todos", pillSectionTodos, []session.Todo{{Content: "a", Status: session.TodoStatusPending}}, 2, nil, pillSectionTodos},
		{"queue focus with queue", pillSectionQueue, nil, 2, nil, pillSectionQueue},
		{"cron focus with cron", pillSectionCron, nil, 0, []scheduler.Task{{ID: "t1"}}, pillSectionCron},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := newTestUI()
			u.session = &session.Session{ID: "s1", Todos: tc.todos}
			u.promptQueue = tc.queue
			u.cronTasks = tc.cron
			u.focusedPillSection = tc.stored
			if got := u.effectiveFocusedSection(); got != tc.expected {
				t.Fatalf("effectiveFocusedSection() = %d, want %d", got, tc.expected)
			}
		})
	}
}

// TestSwitchPillSectionCyclesThroughSections verifies ←/→ moves focus
// through every section that has content — todos, queued prompts, and
// scheduled tasks — and wraps around at the ends.
func TestSwitchPillSectionCyclesThroughSections(t *testing.T) {
	u := newTestUI()
	u.session = &session.Session{ID: "s1", Todos: []session.Todo{
		{Content: "a", Status: session.TodoStatusPending},
	}}
	u.promptQueue = 1
	u.cronTasks = []scheduler.Task{{ID: "t1", NextRunAt: time.Now()}}
	u.pillsExpanded = true
	u.focusedPillSection = pillSectionTodos

	u.switchPillSection(1)
	if u.focusedPillSection != pillSectionQueue {
		t.Fatalf("after right from todos, focusedPillSection = %d, want %d", u.focusedPillSection, pillSectionQueue)
	}
	u.switchPillSection(1)
	if u.focusedPillSection != pillSectionCron {
		t.Fatalf("after right from queue, focusedPillSection = %d, want %d", u.focusedPillSection, pillSectionCron)
	}
	u.switchPillSection(1)
	if u.focusedPillSection != pillSectionTodos {
		t.Fatalf("after right from cron, focusedPillSection = %d, want %d (wrap)", u.focusedPillSection, pillSectionTodos)
	}
	u.switchPillSection(-1)
	if u.focusedPillSection != pillSectionCron {
		t.Fatalf("after left from todos, focusedPillSection = %d, want %d (wrap)", u.focusedPillSection, pillSectionCron)
	}
}

// TestExpandedPillsShowFocusedSection verifies the expanded panel lists the
// focused section's content only, so ←/→ switching changes what is shown.
func TestExpandedPillsShowFocusedSection(t *testing.T) {
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

	u.focusedPillSection = pillSectionTodos
	u.renderPills()
	if !strings.Contains(u.pillsView, "write the todo") {
		t.Fatalf("expected expanded pills to contain the todo:\n%s", u.pillsView)
	}

	u.switchPillSection(1) // queue
	u.renderPills()
	if !strings.Contains(u.pillsView, "queued prompt") {
		t.Fatalf("expected expanded pills to contain the queued prompt:\n%s", u.pillsView)
	}
	if strings.Contains(u.pillsView, "write the todo") {
		t.Fatalf("expected the todo list to be hidden while the queue is focused:\n%s", u.pillsView)
	}

	u.switchPillSection(1) // cron
	u.renderPills()
	if !strings.Contains(u.pillsView, "scheduled prompt") {
		t.Fatalf("expected expanded pills to contain the scheduled task:\n%s", u.pillsView)
	}
}

// TestPillsAreaHeightCoversFocusedSection verifies the reserved height
// accounts for the focused section's list, so it is not clipped.
func TestPillsAreaHeightCoversFocusedSection(t *testing.T) {
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
	u.focusedPillSection = pillSectionTodos
	if got, want := u.pillsAreaHeight(), pillHeightWithBorder+2; got != want {
		t.Fatalf("expanded todos pillsAreaHeight() = %d, want %d", got, want)
	}
	u.focusedPillSection = pillSectionQueue
	if got, want := u.pillsAreaHeight(), pillHeightWithBorder+3; got != want {
		t.Fatalf("expanded queue pillsAreaHeight() = %d, want %d", got, want)
	}
	u.focusedPillSection = pillSectionCron
	if got, want := u.pillsAreaHeight(), pillHeightWithBorder+2; got != want {
		t.Fatalf("expanded cron pillsAreaHeight() = %d, want %d", got, want)
	}
}
