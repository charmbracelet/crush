package cmd

import (
	"testing"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

func TestConvertPartsSubagentReport(t *testing.T) {
	t.Parallel()

	got := convertParts([]message.ContentPart{
		message.SubagentReport{Name: "code-reviewer-1", Status: "completed", Output: "looks good"},
	})

	require.Equal(t, []sessionShowPart{{
		Type:    "subagent_report",
		Name:    "code-reviewer-1",
		Status:  "completed",
		Content: "looks good",
	}}, got)
}
