package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/stretchr/testify/require"
)

func TestGoalAndLoopCommandsVisibleWithSession(t *testing.T) {
	t.Parallel()

	com := &common.Common{Workspace: &rcCommandsWorkspace{}}
	c := &Commands{
		com:         com,
		hasSession:  true,
		list:        list.NewFilterableList(),
		windowWidth: 200,
	}

	ids := make([]string, 0, len(c.defaultCommands()))
	for _, it := range c.defaultCommands() {
		ids = append(ids, it.ID())
	}

	require.Contains(t, ids, "goal")
	require.Contains(t, ids, "loop")
}
