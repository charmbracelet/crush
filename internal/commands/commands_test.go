package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/skills"
	"github.com/stretchr/testify/require"
)

func TestLoadFromSource_NonExistentDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "does-not-exist")

	cmds, err := loadFromSource(commandSource{path: dir, prefix: userCommandPrefix})
	require.NoError(t, err)
	require.Empty(t, cmds)

	// directory must NOT have been created
	_, statErr := os.Stat(dir)
	require.True(t, os.IsNotExist(statErr))
}

func TestLoadFromSource_ExistingDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.md"), []byte("say hello"), 0o644))

	cmds, err := loadFromSource(commandSource{path: dir, prefix: userCommandPrefix})
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Equal(t, "user:hello", cmds[0].ID)
	require.Equal(t, "say hello", cmds[0].Content)
}

func TestLoadAll_MixedSources(t *testing.T) {
	t.Parallel()

	existing := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(existing, "cmd.md"), []byte("content"), 0o644))

	missing := filepath.Join(t.TempDir(), "nope")

	cmds, err := loadAll([]commandSource{
		{path: existing, prefix: userCommandPrefix},
		{path: missing, prefix: projectCommandPrefix},
	})
	require.NoError(t, err)
	require.Len(t, cmds, 1)
	require.Equal(t, "user:cmd", cmds[0].ID)
}

func TestFromSkillCatalog_UserInvocableOnly(t *testing.T) {
	t.Parallel()

	cmds := FromSkillCatalog([]skills.CatalogEntry{
		{
			ID:            "/skills/on/SKILL.md",
			Name:          "on",
			Description:   "Enabled.",
			Label:         "user:on",
			UserInvocable: true,
		},
		{
			ID:            "/skills/off/SKILL.md",
			Name:          "off",
			Description:   "Not invocable.",
			Label:         "user:off",
			UserInvocable: false,
		},
	})

	require.Len(t, cmds, 1)
	require.Equal(t, "user:on", cmds[0].ID)
	require.Equal(t, "user:on", cmds[0].Name)
	require.Equal(t, "on", cmds[0].Skill.Name)
	require.Equal(t, "Enabled.", cmds[0].Skill.Description)
	require.Equal(t, "/skills/on/SKILL.md", cmds[0].Skill.SkillFilePath)
}

func TestFromSkillCatalog_UsesDiscoveredSymlinkedSkills(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires special privileges on Windows")
	}
	t.Parallel()

	root := t.TempDir()
	targetParent := t.TempDir()
	targetSkillDir := filepath.Join(targetParent, "linked-skill")
	require.NoError(t, os.MkdirAll(targetSkillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(targetSkillDir, skills.SkillFileName),
		[]byte("---\nname: linked-skill\ndescription: Symlinked.\nuser-invocable: true\n---\nUse me.\n"),
		0o644,
	))

	link := filepath.Join(root, "linked-skill")
	require.NoError(t, os.Symlink(targetSkillDir, link))

	_, activeSkills, _ := skills.DiscoverFromConfig(skills.DiscoveryConfig{
		SkillsPaths: []string{root},
	})
	entries := skills.Catalog(activeSkills, []string{root}, "")
	cmds := FromSkillCatalog(entries)

	require.Len(t, cmds, 1)
	require.Equal(t, "user:linked-skill", cmds[0].ID)
	require.Equal(t, "linked-skill", cmds[0].Skill.Name)
	require.Equal(t, filepath.Join(link, skills.SkillFileName), cmds[0].Skill.SkillFilePath)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestLoadsClaudeCommandsProjectAndUser(t *testing.T) {
	t.Parallel()

	fakeHome, project := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(fakeHome, ".claude", "commands", "review.md"), "Review code")
	writeFile(t, filepath.Join(fakeHome, ".claude", "commands", "git", "sync.md"), "Sync repo")
	writeFile(t, filepath.Join(project, ".claude", "commands", "deploy.md"), "Deploy it")

	cmds, err := loadAll(claudeSources(fakeHome, project))
	require.NoError(t, err)

	byID := map[string]string{}
	for _, c := range cmds {
		byID[c.ID] = c.Content
	}
	require.Equal(t, map[string]string{
		"user:review":    "Review code",
		"user:git:sync":  "Sync repo",
		"project:deploy": "Deploy it",
	}, byID)
}

func TestClaudeArgumentsPlaceholder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "fix.md"), "---\ndescription: Fix an issue\nargument-hint: [issue]\nallowed-tools: Bash(git add:*), Bash(git status:*)\nmodel: sonnet\n---\n\nFix issue $ARGUMENTS now, keep $1 literal.\n")
	writeFile(t, filepath.Join(dir, "broken.md"), "---\ndescription: [unclosed\n---\nBody $ARGUMENTS\n")
	writeFile(t, filepath.Join(dir, "plain.md"), "No frontmatter\n---\nstill body\n")

	cmds, err := loadFromSource(commandSource{path: dir, prefix: userCommandPrefix, claude: true})
	require.NoError(t, err)
	byID := map[string]CustomCommand{}
	for _, c := range cmds {
		byID[c.ID] = c
	}
	require.Len(t, byID, 3)

	fix := byID["user:fix"]
	require.Equal(t, "Fix issue $ARGUMENTS now, keep $1 literal.\n", fix.Content)
	// $ARGUMENTS is optional in Claude Code; positional $1 is not parsed.
	require.Equal(t, []Argument{{ID: "ARGUMENTS", Title: "ARGUMENTS"}}, fix.Arguments)

	// Frontmatter that fails to parse is still stripped.
	require.Equal(t, "Body $ARGUMENTS\n", byID["user:broken"].Content)
	// A "---" that is not at the top is body, not frontmatter.
	require.Equal(t, "No frontmatter\n---\nstill body\n", byID["user:plain"].Content)
}

func TestClaudeCommandsDoNotShadow(t *testing.T) {
	t.Parallel()

	crushDir, claudeDir := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(crushDir, "same.md"), "crush wins")
	writeFile(t, filepath.Join(claudeDir, "same.md"), "claude loses")
	writeFile(t, filepath.Join(claudeDir, "other.md"), "claude only")

	cmds, err := loadAll([]commandSource{
		{path: crushDir, prefix: userCommandPrefix},
		{path: claudeDir, prefix: userCommandPrefix, claude: true},
	})
	require.NoError(t, err)

	byID := map[string]string{}
	for _, c := range cmds {
		byID[c.ID] = c.Content
	}
	require.Len(t, cmds, 2)
	require.Equal(t, "crush wins", byID["user:same"])
	require.Equal(t, "claude only", byID["user:other"])
}

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ in, want string }{
		"empty block":           {"---\n---\nbody\n", "body\n"},
		"no frontmatter":        {"no close\n", "no close\n"},
		"unclosed":              {"---\nunclosed\n", "---\nunclosed\n"},
		"crlf":                  {"---\r\ndescription: x\r\n---\r\nbody\r\n", "body\n"},
		"closing at EOF":        {"---\ndescription: x\n---", ""},
		"closing at EOF nl":     {"---\ndescription: x\n---\n", ""},
		"trailing spaces":       {"---\ndescription: x\n--- \t\nbody\n", "body\n"},
		"bom":                   {"\uFEFF---\na: b\n---\nbody\n", "body\n"},
		"blank line after":      {"---\na: b\n---\n\nbody\n", "body\n"},
		"body starting w/ rule": {"---\nfoo\n---\nbar\n", "bar\n"},
		"rule without closer":   {"---\nfoo bar\n", "---\nfoo bar\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, stripFrontmatter(tc.in))
		})
	}
}

func TestBuildCommandSources(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	dataA := filepath.Join(t.TempDir(), "data")
	dataB := filepath.Join(t.TempDir(), "elsewhere", "data")

	var first []commandSource
	for _, data := range []string{dataA, dataB} {
		cfg := &config.Config{Options: &config.Options{DataDirectory: data}}
		sources := buildCommandSources(cfg, workingDir)
		require.Len(t, sources, 5)

		// Crush sources come first, in their original order.
		require.Equal(t, filepath.Join(home.Config(), "crush", "commands"), sources[0].path)
		require.Equal(t, filepath.Join(home.Dir(), ".crush", "commands"), sources[1].path)
		require.Equal(t, filepath.Join(data, "commands"), sources[2].path)
		for _, s := range sources[:3] {
			require.False(t, s.claude)
		}

		require.Equal(t, commandSource{
			path: filepath.Join(home.Dir(), ".claude", "commands"), prefix: userCommandPrefix, claude: true,
		}, sources[3])
		require.Equal(t, commandSource{
			path: filepath.Join(workingDir, ".claude", "commands"), prefix: projectCommandPrefix, claude: true,
		}, sources[4])

		if first == nil {
			first = sources[3:]
		}
		// DataDirectory has no influence on the Claude paths.
		require.Equal(t, first, sources[3:])
	}
}
