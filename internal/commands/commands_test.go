package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractArgNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		expected []Argument
	}{
		{
			name:     "no args",
			content:  "say hello",
			expected: nil,
		},
		{
			name:    "ascii args",
			content: "run with $DATA_DIR and $OUTPUT_FILE",
			expected: []Argument{
				{ID: "DATA_DIR", Title: "DATA_DIR", Required: true},
				{ID: "OUTPUT_FILE", Title: "OUTPUT_FILE", Required: true},
			},
		},
		{
			name:    "unicode args",
			content: "python script.py $数据目录 $输出文件",
			expected: []Argument{
				{ID: "数据目录", Title: "数据目录", Required: true},
				{ID: "输出文件", Title: "输出文件", Required: true},
			},
		},
		{
			name:    "mixed args",
			content: "echo $FOO $bar $数据 $123",
			expected: []Argument{
				{ID: "FOO", Title: "FOO", Required: true},
				{ID: "bar", Title: "bar", Required: true},
				{ID: "数据", Title: "数据", Required: true},
			},
		},
		{
			name:    "underscore and numbers",
			content: "echo $MY_VAR_1 $_private $数据2",
			expected: []Argument{
				{ID: "MY_VAR_1", Title: "MY_VAR_1", Required: true},
				{ID: "_private", Title: "_private", Required: true},
				{ID: "数据2", Title: "数据2", Required: true},
			},
		},
		{
			name:    "duplicate args",
			content: "echo $FOO and $FOO again",
			expected: []Argument{
				{ID: "FOO", Title: "FOO", Required: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := extractArgNames(tt.content)
			require.Equal(t, tt.expected, result)
		})
	}
}

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
