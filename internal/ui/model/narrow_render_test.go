package model

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestLandingViewFitsMainArea(t *testing.T) {
	t.Parallel()

	for _, sz := range [][2]int{{80, 14}, {40, 14}, {24, 20}, {36, 8}, {60, 6}, {20, 3}} {
		t.Run(fmt.Sprintf("%dx%d", sz[0], sz[1]), func(t *testing.T) {
			t.Parallel()

			com := common.DefaultCommon(nil)
			com.Workspace = &testWorkspace{cfg: &config.Config{Options: &config.Options{}}}
			u := &UI{com: com}
			for i := range 15 {
				u.skillStates = append(u.skillStates, &skills.SkillState{
					Name: fmt.Sprintf("skill-number-%02d", i),
					Path: fmt.Sprintf("/s/%02d/SKILL.md", i),
				})
			}
			u.layout.main = image.Rect(0, 0, sz[0], sz[1])
			out := u.landingView()
			lines := strings.Split(out, "\n")
			require.LessOrEqual(t, len(lines), max(1, sz[1]-1))
			for _, l := range lines {
				require.LessOrEqual(t, ansi.StringWidth(l), sz[0])
			}
		})
	}
}

func TestTodoPillFitsWidth(t *testing.T) {
	t.Parallel()

	sty := common.DefaultCommon(nil).Styles
	todos := []session.Todo{
		{Content: "a", Status: session.TodoStatusCompleted},
		{Content: "Refactor the very long task name that keeps going and going", Status: session.TodoStatusInProgress},
	}
	for _, w := range []int{20, 30, 47, 80} {
		got := todoPill(todos, "*", false, sty, w)
		require.LessOrEqual(t, lipgloss.Width(got), max(w, lipgloss.Width(todoPill(todos, "*", false, sty, 0))), "width %d", w)
	}
	require.Contains(t, ansi.Strip(todoPill(todos, "*", false, sty, 80)), "Refactor")
	require.NotContains(t, ansi.Strip(todoPill(todos, "*", false, sty, 15)), "Refactor")
}
