package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/skills"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	return cmd
}

func cmdOutput(cmd *cobra.Command) string {
	return cmd.OutOrStdout().(*bytes.Buffer).String()
}

func TestFilterSkills(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{Name: "pdf-processing", Description: "Extracts text from PDFs"},
		{Name: "commit", Description: "Create git commits"},
		{Name: "review-pr", Description: "Review pull requests"},
	}

	tests := []struct {
		name    string
		term    string
		wantLen int
		wantHas string
	}{
		{"match by name", "pdf", 1, "pdf-processing"},
		{"match by description", "git", 1, "commit"},
		{"no match", "xyz", 0, ""},
		{"case insensitive", "PDF", 1, "pdf-processing"},
		{"multiple matches", "pr", 2, "review-pr"}, // review-pr and pdf-processing
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := filterSkills(all, tt.term)
			require.Len(t, result, tt.wantLen)
			if tt.wantHas != "" {
				var found bool
				for _, s := range result {
					if s.Name == tt.wantHas {
						found = true
					}
				}
				require.True(t, found, "expected to find %q in results", tt.wantHas)
			}
		})
	}
}

func TestOutputSkillsJSON(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{Name: "crush-config", Description: "Configure crush", Builtin: true, Path: "crush://skills/crush-config", SkillFilePath: "crush://skills/crush-config/SKILL.md"},
		{Name: "local-lint", Description: "Project linter", Builtin: false, Path: "/work/.crush/skills/local-lint", SkillFilePath: "/work/.crush/skills/local-lint/SKILL.md"},
		{Name: "deploy", Description: "Deploy tool", Builtin: false, Path: "/home/user/.config/crush/skills/deploy", SkillFilePath: "/home/user/.config/crush/skills/deploy/SKILL.md"},
	}
	disabledSet := map[string]bool{"deploy": true}
	projectDirs := []string{"/work/.crush/skills"}

	cmd := newTestCmd()
	err := outputSkillsJSON(cmd, all, disabledSet, projectDirs)
	require.NoError(t, err)

	var result []skillJSON
	require.NoError(t, json.Unmarshal([]byte(cmdOutput(cmd)), &result))

	require.Len(t, result, 3)

	// Verify source classification.
	byName := make(map[string]skillJSON)
	for _, s := range result {
		byName[s.Name] = s
	}

	require.Equal(t, "builtin", byName["crush-config"].Source)
	require.Equal(t, "enabled", byName["crush-config"].Status)

	require.Equal(t, "project", byName["local-lint"].Source)
	require.Equal(t, "enabled", byName["local-lint"].Status)

	require.Equal(t, "user", byName["deploy"].Source)
	require.Equal(t, "disabled", byName["deploy"].Status)
}

func TestOutputSkillsFlat(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{Name: "crush-config", Builtin: true, Path: "crush://skills/crush-config"},
		{Name: "deploy", Builtin: false, Path: "/home/user/.config/crush/skills/deploy"},
	}
	disabledSet := map[string]bool{"deploy": true}

	cmd := newTestCmd()
	err := outputSkillsFlat(cmd, all, disabledSet, nil)
	require.NoError(t, err)

	output := cmdOutput(cmd)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 2)

	require.Contains(t, lines[0], "crush-config")
	require.Contains(t, lines[0], "builtin")
	require.Contains(t, lines[0], "enabled")

	require.Contains(t, lines[1], "deploy")
	require.Contains(t, lines[1], "user")
	require.Contains(t, lines[1], "disabled")
}

func TestOutputSkillsFlatAlignment(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{Name: "crush-config", Builtin: true, Path: "crush://skills/crush-config"},
		{Name: "slack-gif-creator", Builtin: false, Path: "/home/user/.config/crush/skills/slack-gif-creator"},
		{Name: "pdf", Builtin: false, Path: "/work/.crush/skills/pdf"},
	}
	disabledSet := map[string]bool{}
	projectDirs := []string{"/work/.crush/skills"}

	cmd := newTestCmd()
	err := outputSkillsFlat(cmd, all, disabledSet, projectDirs)
	require.NoError(t, err)

	output := cmdOutput(cmd)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	require.Len(t, lines, 3)

	// All source columns should start at the same position (aligned by tabwriter).
	sourceCol := strings.Index(lines[0], "builtin")
	require.Greater(t, sourceCol, 0, "source column should be present")
	require.Equal(t, sourceCol, strings.Index(lines[1], "user"), "source columns should be aligned")
	require.Equal(t, sourceCol, strings.Index(lines[2], "project"), "source columns should be aligned")

	// All status columns should also be aligned.
	statusCol0 := strings.Index(lines[0], "enabled")
	statusCol1 := strings.Index(lines[1], "enabled")
	statusCol2 := strings.Index(lines[2], "enabled")
	require.Equal(t, statusCol0, statusCol1, "status columns should be aligned")
	require.Equal(t, statusCol0, statusCol2, "status columns should be aligned")
}

func TestOutputSkillsTree(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{Name: "crush-config", Description: "Configure crush", Builtin: true, Path: "crush://skills/crush-config"},
		{Name: "local-lint", Description: "Project linter", Builtin: false, Path: "/work/.crush/skills/local-lint"},
		{Name: "deploy", Description: "Deploy tool", Builtin: false, Path: "/home/user/.config/crush/skills/deploy"},
	}
	disabledSet := map[string]bool{"deploy": true}
	projectDirs := []string{"/work/.crush/skills"}

	cmd := newTestCmd()
	err := outputSkillsTree(cmd, all, disabledSet, projectDirs)
	require.NoError(t, err)

	output := cmdOutput(cmd)

	// Verify group headers are present.
	require.Contains(t, output, "Builtin")
	require.Contains(t, output, "Project")
	require.Contains(t, output, "User")

	// Verify skill entries.
	require.Contains(t, output, "crush-config")
	require.Contains(t, output, "local-lint")
	require.Contains(t, output, "deploy")

	// Verify disabled annotation.
	require.Contains(t, output, "(disabled)")
}

func TestOutputSkillsTreeSingleGroup(t *testing.T) {
	t.Parallel()

	all := []*skills.Skill{
		{Name: "crush-config", Description: "Configure crush", Builtin: true, Path: "crush://skills/crush-config"},
	}

	cmd := newTestCmd()
	err := outputSkillsTree(cmd, all, nil, nil)
	require.NoError(t, err)

	output := cmdOutput(cmd)
	require.Contains(t, output, "Builtin")
	require.Contains(t, output, "crush-config")
	require.NotContains(t, output, "Project")
	require.NotContains(t, output, "User")
}
