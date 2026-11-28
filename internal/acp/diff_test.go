package acp

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/stretchr/testify/require"
)

func TestToolDiffContentForEditPermission(t *testing.T) {
	abs := func(p string) string { return filepath.Join("/root", p) }
	contents := toolDiffContent(abs, tools.EditPermissionsParams{
		FilePath:   "main.go",
		OldContent: "a\nb",
		NewContent: "a\nc",
	})
	require.Len(t, contents, 1)
	diff := contents[0].Diff
	require.NotNil(t, diff)
	require.Equal(t, filepath.Join("/root", "main.go"), diff.Path)
	require.Equal(t, "a\nc", diff.NewText)
	require.NotNil(t, diff.OldText)
	require.Equal(t, "a\nb", *diff.OldText)
}

func TestToolDiffContentOmitsOldTextForNewFile(t *testing.T) {
	contents := toolDiffContent(nil, tools.WritePermissionsParams{
		FilePath:   "new.go",
		NewContent: "package main",
	})
	require.Len(t, contents, 1)
	require.Nil(t, contents[0].Diff.OldText)
}

func TestToolDiffContentIgnoresToolsWithoutDiff(t *testing.T) {
	require.Nil(t, toolDiffContent(nil, tools.BashPermissionsParams{Command: "ls"}))
	require.Nil(t, toolDiffContent(nil, nil))
	// An edit that changes nothing has no diff to show.
	require.Nil(t, toolDiffContent(nil, tools.EditPermissionsParams{
		FilePath:   "main.go",
		OldContent: "same",
		NewContent: "same",
	}))
}
