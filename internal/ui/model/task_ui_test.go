package model

import (
	"image"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func TestRenderPillsShowsOnlyQueue(t *testing.T) {
	t.Parallel()

	theme := styles.DefaultStyles()
	ui := &UI{
		session: &session.Session{
			ID: "s1",
			Todos: []session.Todo{
				{ID: "todo-1", Content: "do thing", Status: session.TodoStatusPending},
				{ID: "todo-2", Content: "do next", Status: session.TodoStatusPending},
			},
		},
		promptQueue: 2,
		layout:      uiLayout{pills: image.Rect(0, 0, 80, 4)},
		com:         &common.Common{Styles: &theme},
	}

	ui.renderPills()
	require.Contains(t, ui.pillsView, "Queued")
	require.NotContains(t, ui.pillsView, "To-Do")
}
