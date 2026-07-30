package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func runMemoryTool(t *testing.T, dataDir string, params MemoryWriteParams) fantasy.ToolResponse {
	t.Helper()
	tool := NewMemoryTool(dataDir)
	input, err := json.Marshal(params)
	require.NoError(t, err)
	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  MemoryWriteToolName,
		Input: string(input),
	})
	require.NoError(t, err)
	return resp
}

func TestMemoryWriteSaveAndIndex(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "build-commands",
		Description: "How to run tests",
		Content:     "Use `task test` for unit tests.",
	})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, `Saved memory "build-commands"`)
	require.Contains(t, resp.Content, "next crush launch")

	path := filepath.Join(dataDir, "memory", "build-commands.md")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "---\ndescription: How to run tests\n---\n\nUse `task test` for unit tests.", string(b))

	index, err := os.ReadFile(filepath.Join(dataDir, "memory", "MEMORY.md"))
	require.NoError(t, err)
	require.Equal(t, "# Memory index\n\n- build-commands: How to run tests\n", string(index))
}

func TestMemoryWriteOverwrite(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "pref",
		Description: "old",
		Content:     "old content",
	})
	require.False(t, resp.IsError)

	resp = runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "pref",
		Description: "new",
		Content:     "new content",
	})
	require.False(t, resp.IsError)

	b, err := os.ReadFile(filepath.Join(dataDir, "memory", "pref.md"))
	require.NoError(t, err)
	require.Contains(t, string(b), "description: new")
	require.Contains(t, string(b), "new content")

	index, err := os.ReadFile(filepath.Join(dataDir, "memory", "MEMORY.md"))
	require.NoError(t, err)
	require.Contains(t, string(index), "- pref: new")
	require.NotContains(t, string(index), "old")
}

func TestMemoryWriteDelete(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "temp-fact",
		Description: "temporary",
		Content:     "will be deleted",
	})
	require.False(t, resp.IsError)

	resp = runMemoryTool(t, dataDir, MemoryWriteParams{
		Action: "delete",
		Name:   "temp-fact",
	})
	require.False(t, resp.IsError)
	require.Contains(t, resp.Content, `Deleted memory "temp-fact"`)

	_, err := os.Stat(filepath.Join(dataDir, "memory", "temp-fact.md"))
	require.True(t, os.IsNotExist(err))

	index, err := os.ReadFile(filepath.Join(dataDir, "memory", "MEMORY.md"))
	require.NoError(t, err)
	require.Equal(t, "# Memory index\n\n", string(index))
}

func TestMemoryWriteDeleteMissing(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action: "delete",
		Name:   "does-not-exist",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "no memory named does-not-exist")
}

func TestMemoryWriteSlugValidation(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	cases := []string{
		"../x",
		"a/b",
		"UPPER",
		"",
		"has space",
		"under_score",
		"-leading-hyphen",
		strings.Repeat("a", 65),
	}
	for _, name := range cases {
		resp := runMemoryTool(t, dataDir, MemoryWriteParams{
			Action:      "save",
			Name:        name,
			Description: "desc",
			Content:     "body",
		})
		require.True(t, resp.IsError, "expected rejection for name %q", name)
	}
}

func TestMemoryWriteCaps(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "ok",
		Description: strings.Repeat("d", maxMemoryDescription+1),
		Content:     "body",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "description must be at most")

	resp = runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "ok",
		Description: "desc",
		Content:     strings.Repeat("x", maxMemoryContentBytes+1),
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "content must be at most")

	resp = runMemoryTool(t, dataDir, MemoryWriteParams{
		Action: "save",
		Name:   "ok",
		// empty description
		Content: "body",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "description is required")

	resp = runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "ok",
		Description: "desc",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "content is required")

	resp = runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "ok",
		Description: "line1\nline2",
		Content:     "body",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "description must be a single line")
}

func TestMemoryWriteFileCountCap(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	memoryDir := filepath.Join(dataDir, "memory")
	require.NoError(t, os.MkdirAll(memoryDir, 0o755))

	// Pre-fill to the limit without going through the tool so the test
	// stays fast.
	for i := range maxMemoryFiles {
		name := filepath.Join(memoryDir, "m"+strconv.Itoa(i)+".md")
		require.NoError(t, os.WriteFile(name, []byte("---\ndescription: x\n---\n\nbody"), 0o644))
	}

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "one-more",
		Description: "overflow",
		Content:     "should fail",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "memory limit")

	// Overwriting an existing name must still succeed.
	resp = runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "m0",
		Description: "updated",
		Content:     "ok to overwrite at limit",
	})
	require.False(t, resp.IsError)
}

func TestMemoryWriteIndexSortedAndSlugFallback(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	memoryDir := filepath.Join(dataDir, "memory")
	require.NoError(t, os.MkdirAll(memoryDir, 0o755))

	// Hand-written file without frontmatter → description falls back to slug.
	require.NoError(t, os.WriteFile(filepath.Join(memoryDir, "zebra.md"), []byte("no frontmatter"), 0o644))

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action:      "save",
		Name:        "alpha",
		Description: "first",
		Content:     "alpha body",
	})
	require.False(t, resp.IsError)

	index, err := os.ReadFile(filepath.Join(memoryDir, "MEMORY.md"))
	require.NoError(t, err)
	require.Equal(t, "# Memory index\n\n- alpha: first\n- zebra: zebra\n", string(index))
}

// The tool must never report a save it will not show the model. Filling the
// store at the tool's own advertised per-file limits used to produce a ~27KiB
// MEMORY.md, of which prompt.go injects only the first 16KiB — the tail of the
// alphabet was saved to disk yet permanently invisible.
func TestMemoryWriteIndexStaysWithinPromptBudget(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	memoryDir := filepath.Join(dataDir, "memory")

	saved := []string{}
	rejected := 0
	for i := range maxMemoryFiles {
		suffix := fmt.Sprintf("-%03d", i)
		name := strings.Repeat("a", 64-len(suffix)) + suffix
		resp := runMemoryTool(t, dataDir, MemoryWriteParams{
			Action:      "save",
			Name:        name,
			Description: strings.Repeat("d", maxMemoryDescription),
			Content:     "body",
		})
		if resp.IsError {
			require.Contains(t, resp.Content, "memory index would exceed")
			rejected++
			// A rejected save must leave nothing behind.
			_, err := os.Stat(filepath.Join(memoryDir, name+".md"))
			require.True(t, os.IsNotExist(err), "rejected save left an orphan file for %s", name)
			continue
		}
		saved = append(saved, name)
	}
	index, err := os.ReadFile(filepath.Join(memoryDir, memoryIndexFileName))
	require.NoError(t, err)
	require.LessOrEqual(t, len(index), MaxMemoryIndexBytes,
		"MEMORY.md (%d bytes) exceeds the %d-byte budget prompt.go injects, so %d of %d saves are invisible to the model",
		len(index), MaxMemoryIndexBytes,
		len(saved)-strings.Count(loadIndexAsPromptWould(string(index)), "\n- "), len(saved))
	require.Positive(t, rejected, "expected the budget to be reached within %d files", maxMemoryFiles)

	// Everything the tool said it saved must actually be in the index, and in
	// the part of it the prompt layer keeps.
	visible := loadIndexAsPromptWould(string(index))
	for _, name := range saved {
		require.Contains(t, visible, "- "+name+": ",
			"tool reported saving %s but it is not visible in the prompt index", name)
	}
}

// loadIndexAsPromptWould mirrors the truncation prompt.loadMemoryIndex applies.
func loadIndexAsPromptWould(index string) string {
	if len(index) <= MaxMemoryIndexBytes {
		return index
	}
	return index[:MaxMemoryIndexBytes]
}

func TestMemoryWriteInvalidAction(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()

	resp := runMemoryTool(t, dataDir, MemoryWriteParams{
		Action: "update",
		Name:   "x",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "action must be save or delete")
}

// Two concurrent writers must not lose an entry.
func TestMemoryWriteConcurrent(t *testing.T) {
	dataDir := t.TempDir()
	memoryDir := filepath.Join(dataDir, "memory")

	const writers = 10
	names := make([]string, writers)
	for i := range writers {
		names[i] = fmt.Sprintf("mem-%03d", i)
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, writers)

	// Use a stand-alone runner so we don't call t.Helper / require from
	// goroutines.
	run := func(dataDir string, params MemoryWriteParams) error {
		tool := NewMemoryTool(dataDir)
		input, jErr := json.Marshal(params)
		if jErr != nil {
			return jErr
		}
		resp, tErr := tool.Run(context.Background(), fantasy.ToolCall{
			ID:    "test-call",
			Name:  MemoryWriteToolName,
			Input: string(input),
		})
		if tErr != nil {
			return tErr
		}
		if resp.IsError {
			return fmt.Errorf("tool error: %s", resp.Content)
		}
		return nil
	}

	for _, name := range names {
		name := name
		go func() {
			err := run(dataDir, MemoryWriteParams{
				Action:      "save",
				Name:        name,
				Description: "entry " + name,
				Content:     "body for " + name,
			})
			results <- result{name: name, err: err}
		}()
	}

	for range writers {
		r := <-results
		require.NoError(t, r.err, "save %s", r.name)
	}

	// All 10 files exist on disk.
	for _, name := range names {
		_, err := os.Stat(filepath.Join(memoryDir, name+".md"))
		require.NoError(t, err, "missing file for %s", name)
	}

	// All 10 entries appear in MEMORY.md.
	index, err := os.ReadFile(filepath.Join(memoryDir, memoryIndexFileName))
	require.NoError(t, err)
	for _, name := range names {
		require.Contains(t, string(index), "- "+name+":", "missing %s in index", name)
	}

	// No stray entries in index.
	count := strings.Count(string(index), "\n- ")
	require.Equal(t, writers, count, "index has %d entries, expected %d", count, writers)
}
