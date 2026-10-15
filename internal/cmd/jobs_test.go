package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestPrintJobsTableAndJSON(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	jobs := []shell.JobInfo{
		{ID: "01B", SessionID: "0123456789abcdef", Command: "npm run dev", Description: "dev server", StartedAt: now.Add(-90 * time.Second), Status: shell.JobRunning, Background: true},
		{ID: "01A", Command: "go test ./...", StartedAt: now.Add(-5 * time.Minute), FinishedAt: now.Add(-4 * time.Minute), Status: shell.JobFailed, ExitCode: 1},
	}
	var table bytes.Buffer
	require.NoError(t, printJobs(&table, jobs, false, now))
	lines := strings.Split(strings.TrimSpace(table.String()), "\n")
	require.Len(t, lines, 3)
	require.Contains(t, lines[1], "01B")
	require.Contains(t, lines[1], "RUNNING")
	require.Contains(t, lines[1], "1m30s")
	require.Contains(t, lines[1], "01234567")
	require.Contains(t, lines[1], "bg")
	require.Contains(t, lines[1], "dev server")
	require.Contains(t, lines[2], "FAILED (1)")
	require.Contains(t, lines[2], "1m0s")
	require.Contains(t, lines[2], "go test ./...")

	var js bytes.Buffer
	require.NoError(t, printJobs(&js, jobs, true, now))
	var back []shell.JobInfo
	require.NoError(t, json.Unmarshal(js.Bytes(), &back))
	require.Len(t, back, 2)

	var empty bytes.Buffer
	require.NoError(t, printJobs(&empty, nil, true, now))
	require.Equal(t, "[]\n", empty.String())
}
