package skills

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassifySource(t *testing.T) {
	t.Parallel()

	projectDirs := []string{"/work/project/.crush/skills", "/work/project/.agents/skills"}

	tests := []struct {
		name  string
		skill *Skill
		want  Source
	}{
		{
			name:  "builtin skill",
			skill: &Skill{Name: "crush-config", Builtin: true, Path: "crush://skills/crush-config"},
			want:  SourceBuiltin,
		},
		{
			name:  "project skill",
			skill: &Skill{Name: "local-lint", Builtin: false, Path: "/work/project/.crush/skills/local-lint"},
			want:  SourceProject,
		},
		{
			name:  "project skill in agents dir",
			skill: &Skill{Name: "my-agent", Builtin: false, Path: "/work/project/.agents/skills/my-agent"},
			want:  SourceProject,
		},
		{
			name:  "user skill",
			skill: &Skill{Name: "custom-deploy", Builtin: false, Path: "/home/user/.config/crush/skills/custom-deploy"},
			want:  SourceUser,
		},
		{
			name:  "user skill from custom path",
			skill: &Skill{Name: "pdf-tool", Builtin: false, Path: "/opt/skills/pdf-tool"},
			want:  SourceUser,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ClassifySource(tt.skill, projectDirs)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClassifySourceEmptyProjectDirs(t *testing.T) {
	t.Parallel()

	s := &Skill{Name: "my-skill", Builtin: false, Path: "/some/path/my-skill"}
	got := ClassifySource(s, nil)
	require.Equal(t, SourceUser, got)
}

func TestGroupBySource(t *testing.T) {
	t.Parallel()

	projectDirs := []string{"/work/project/.crush/skills"}

	all := []*Skill{
		{Name: "commit", Builtin: true, Path: "crush://skills/commit"},
		{Name: "review-pr", Builtin: true, Path: "crush://skills/review-pr"},
		{Name: "local-lint", Builtin: false, Path: "/work/project/.crush/skills/local-lint"},
		{Name: "deploy", Builtin: false, Path: "/home/user/.config/crush/skills/deploy"},
		{Name: "analyze", Builtin: false, Path: "/home/user/.config/crush/skills/analyze"},
	}

	groups := GroupBySource(all, projectDirs)

	require.Len(t, groups, 3)

	// Builtin group first.
	require.Equal(t, SourceBuiltin, groups[0].Source)
	require.Len(t, groups[0].Skills, 2)
	require.Equal(t, "commit", groups[0].Skills[0].Name)
	require.Equal(t, "review-pr", groups[0].Skills[1].Name)

	// Project group second.
	require.Equal(t, SourceProject, groups[1].Source)
	require.Len(t, groups[1].Skills, 1)
	require.Equal(t, "local-lint", groups[1].Skills[0].Name)

	// User group last.
	require.Equal(t, SourceUser, groups[2].Source)
	require.Len(t, groups[2].Skills, 2)
	require.Equal(t, "analyze", groups[2].Skills[0].Name)
	require.Equal(t, "deploy", groups[2].Skills[1].Name)
}

func TestGroupBySourceEmptyGroups(t *testing.T) {
	t.Parallel()

	// Only builtin skills — Project and User groups should be omitted.
	all := []*Skill{
		{Name: "crush-config", Builtin: true, Path: "crush://skills/crush-config"},
	}

	groups := GroupBySource(all, nil)
	require.Len(t, groups, 1)
	require.Equal(t, SourceBuiltin, groups[0].Source)
}

func TestGroupBySourceEmpty(t *testing.T) {
	t.Parallel()

	groups := GroupBySource(nil, nil)
	require.Empty(t, groups)
}

func TestGroupBySourceSortOrder(t *testing.T) {
	t.Parallel()

	// Skills are given out of order; groups should sort them by name.
	all := []*Skill{
		{Name: "zebra", Builtin: true, Path: "crush://skills/zebra"},
		{Name: "alpha", Builtin: true, Path: "crush://skills/alpha"},
		{Name: "middle", Builtin: true, Path: "crush://skills/middle"},
	}

	groups := GroupBySource(all, nil)
	require.Len(t, groups, 1)
	require.Equal(t, "alpha", groups[0].Skills[0].Name)
	require.Equal(t, "middle", groups[0].Skills[1].Name)
	require.Equal(t, "zebra", groups[0].Skills[2].Name)
}
