package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
)

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no frontmatter",
			input:    "just some content",
			expected: "just some content",
		},
		{
			name:     "with frontmatter",
			input:    "---\ndescription: Review code\n---\nReview the following code",
			expected: "Review the following code",
		},
		{
			name:     "with empty frontmatter",
			input:    "---\n---\nContent after empty frontmatter",
			expected: "Content after empty frontmatter",
		},
		{
			name:     "unclosed frontmatter",
			input:    "---\ndescription: broken\nno closing delimiter",
			expected: "---\ndescription: broken\nno closing delimiter",
		},
		{
			name:     "frontmatter with multiple fields",
			input:    "---\ndescription: My command\nallowed-tools: bash, grep\n---\nDo the thing with $ARGUMENTS",
			expected: "Do the thing with $ARGUMENTS",
		},
		{
			name:     "content starting with triple dash but not frontmatter",
			input:    "--- not frontmatter",
			expected: "--- not frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripFrontmatter(tt.input)
			if got != tt.expected {
				t.Errorf("stripFrontmatter() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestLoadCommand_ClaudeFormat(t *testing.T) {
	dir := t.TempDir()
	content := "---\ndescription: Review code changes\n---\nReview the code in $FILE and suggest improvements"
	if err := os.WriteFile(filepath.Join(dir, "review.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	source := commandSource{
		path:        dir,
		prefix:      claudeCommandPrefix,
		frontmatter: true,
	}

	cmd, err := loadCommand(filepath.Join(dir, "review.md"), source)
	if err != nil {
		t.Fatal(err)
	}

	if cmd.ID != "claude:review" {
		t.Errorf("ID = %q, want %q", cmd.ID, "claude:review")
	}
	if cmd.Content != "Review the code in $FILE and suggest improvements" {
		t.Errorf("Content = %q, want frontmatter stripped", cmd.Content)
	}
	if len(cmd.Arguments) != 1 || cmd.Arguments[0].ID != "FILE" {
		t.Errorf("Arguments = %v, want single FILE arg", cmd.Arguments)
	}
}

func TestLoadCommand_CrushFormat(t *testing.T) {
	dir := t.TempDir()
	content := "Review the code in $FILE and suggest improvements"
	if err := os.WriteFile(filepath.Join(dir, "review.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	source := commandSource{
		path:   dir,
		prefix: userCommandPrefix,
	}

	cmd, err := loadCommand(filepath.Join(dir, "review.md"), source)
	if err != nil {
		t.Fatal(err)
	}

	if cmd.ID != "user:review" {
		t.Errorf("ID = %q, want %q", cmd.ID, "user:review")
	}
	if cmd.Content != content {
		t.Errorf("Content = %q, want original content preserved", cmd.Content)
	}
}

func TestLoadFromSource_ClaudeDir(t *testing.T) {
	dir := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude", "commands")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	files := map[string]string{
		"review.md":     "---\ndescription: Review\n---\nReview $ARGUMENTS",
		"deploy.md":     "Deploy to $ENVIRONMENT",
		"git/commit.md": "---\ndescription: Commit helper\n---\nCommit with message: $MESSAGE",
	}
	for name, content := range files {
		path := filepath.Join(claudeDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	source := commandSource{
		path:        claudeDir,
		prefix:      claudeCommandPrefix,
		frontmatter: true,
	}

	cmds, err := loadFromSource(source)
	if err != nil {
		t.Fatal(err)
	}

	if len(cmds) != 3 {
		t.Fatalf("got %d commands, want 3", len(cmds))
	}

	byID := make(map[string]CustomCommand)
	for _, cmd := range cmds {
		byID[cmd.ID] = cmd
	}

	if cmd, ok := byID["claude:review"]; !ok {
		t.Error("missing claude:review command")
	} else if cmd.Content != "Review $ARGUMENTS" {
		t.Errorf("claude:review content = %q, want frontmatter stripped", cmd.Content)
	}

	if cmd, ok := byID["claude:deploy"]; !ok {
		t.Error("missing claude:deploy command")
	} else if cmd.Content != "Deploy to $ENVIRONMENT" {
		t.Errorf("claude:deploy content = %q, no frontmatter to strip", cmd.Content)
	}

	if _, ok := byID["claude:git:commit"]; !ok {
		t.Error("missing claude:git:commit command (nested directory)")
	}
}

func TestLoadFromSource_MissingDir(t *testing.T) {
	source := commandSource{
		path:   filepath.Join(t.TempDir(), "nonexistent"),
		prefix: claudeCommandPrefix,
	}

	cmds, err := loadFromSource(source)
	if err != nil {
		t.Fatalf("unexpected error for missing dir: %v", err)
	}
	if len(cmds) != 0 {
		t.Errorf("got %d commands for missing dir, want 0", len(cmds))
	}
}

func TestBuildCommandSources_IncludesClaudeDirs(t *testing.T) {
	cfg := &config.Config{
		Options: &config.Options{
			DataDirectory: filepath.Join(t.TempDir(), ".crush"),
		},
	}

	sources := buildCommandSources(cfg)

	var hasClaude bool
	for _, s := range sources {
		if s.prefix == claudeCommandPrefix {
			hasClaude = true
			if !s.frontmatter {
				t.Error("Claude source missing frontmatter flag")
			}
			if s.createDir {
				t.Error("Claude source should not create directories")
			}
		}
	}
	if !hasClaude {
		t.Error("no Claude command sources found")
	}
}
