package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/charmbracelet/crush/internal/ui/common"
	uistyles "github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
)

func TestSubagentsInfo_Empty(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	m := &UI{
		com:              &common.Common{Styles: &st},
		runningSubagents: nil,
	}

	got := m.subagentsInfo(40, 10, false)

	require.NotEmpty(t, got)
	require.Contains(t, stripANSI(got), "Subagents")
	require.Contains(t, stripANSI(got), "None")
}

func TestSubagentsInfo_SingleEntry(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	m := &UI{
		com: &common.Common{Styles: &st},
		runningSubagents: []workspace.RunningSubagentInfo{
			{Name: "test-agent", Color: "blue"},
		},
	}

	got := m.subagentsInfo(40, 10, false)

	require.Contains(t, stripANSI(got), "test-agent")
	dot := uistyles.SubagentDot("blue")
	require.Contains(t, got, dot)
}

func TestSubagentsInfo_MultipleEntries(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	m := &UI{
		com: &common.Common{Styles: &st},
		runningSubagents: []workspace.RunningSubagentInfo{
			{Name: "alpha-agent", Color: "red"},
			{Name: "beta-agent", Color: "green"},
			{Name: "gamma-agent", Color: "purple"},
		},
	}

	got := m.subagentsInfo(40, 10, false)
	plain := stripANSI(got)

	for _, name := range []string{"alpha-agent", "beta-agent", "gamma-agent"} {
		require.Contains(t, plain, name, "expected %q in output", name)
	}
}

func TestSubagentsInfo_TruncatesAtMaxItems(t *testing.T) {
	t.Parallel()

	st := uistyles.CharmtonePantera()
	m := &UI{
		com: &common.Common{Styles: &st},
		runningSubagents: []workspace.RunningSubagentInfo{
			{Name: "agent-one", Color: "red"},
			{Name: "agent-two", Color: "green"},
			{Name: "agent-three", Color: "blue"},
		},
	}

	// maxItems=2 with 3 entries: the list helper reserves one slot for the
	// trailer, so visibleItems = items[:1] and remaining = 3-(2-1) = 2,
	// producing "…and 2 more" — mirrors the skillsList truncation pattern.
	got := m.subagentsInfo(40, 2, false)
	plain := stripANSI(got)

	require.Contains(t, plain, "…and 2 more")
}
