package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/lock"
)

//go:embed memory.md
var memoryDescription string

const MemoryWriteToolName = "memory_write"

// memoryNameRe validates memory slugs: lowercase kebab-case, 1-64 chars.
var memoryNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

const (
	maxMemoryContentBytes = 4096
	maxMemoryDescription  = 200
	maxMemoryFiles        = 100
	memoryIndexFileName   = "MEMORY.md"
)

// MaxMemoryIndexBytes bounds MEMORY.md. The prompt layer injects the index
// verbatim into the system prompt and truncates anything past this budget,
// so the writer has to enforce the same budget the reader does. Otherwise a
// save reports success and lands on disk while its index entry falls off the
// end of the prompt, leaving the memory permanently invisible to the model.
const MaxMemoryIndexBytes = 16 * 1024

type MemoryWriteParams struct {
	Action      string `json:"action" description:"save (create or overwrite) or delete"`
	Name        string `json:"name" description:"Memory slug: lowercase letters, digits, hyphens (e.g. build-commands)"`
	Description string `json:"description,omitempty" description:"One-line summary shown in the memory index (required for save)"`
	Content     string `json:"content,omitempty" description:"The memory body in markdown (required for save, max 4096 bytes)"`
}

// NewMemoryTool returns the memory_write tool. dataDir is the absolute
// project data directory (Options.DataDirectory).
func NewMemoryTool(dataDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		MemoryWriteToolName,
		memoryDescription,
		func(ctx context.Context, params MemoryWriteParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			switch params.Action {
			case "save", "delete":
			default:
				return fantasy.NewTextErrorResponse("action must be save or delete"), nil
			}

			if !memoryNameRe.MatchString(params.Name) {
				return fantasy.NewTextErrorResponse("name must be lowercase letters, digits, and hyphens only (1-64 chars, e.g. build-commands)"), nil
			}

			memoryDir := filepath.Join(dataDir, "memory")
			path := filepath.Join(memoryDir, params.Name+".md")

			switch params.Action {
			case "save":
				if params.Description == "" {
					return fantasy.NewTextErrorResponse("description is required for save"), nil
				}
				if strings.ContainsAny(params.Description, "\n\r") {
					return fantasy.NewTextErrorResponse("description must be a single line"), nil
				}
				if len(params.Description) > maxMemoryDescription {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("description must be at most %d characters", maxMemoryDescription)), nil
				}
				if params.Content == "" {
					return fantasy.NewTextErrorResponse("content is required for save"), nil
				}
				if len(params.Content) > maxMemoryContentBytes {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("content must be at most %d bytes", maxMemoryContentBytes)), nil
				}

				if err := os.MkdirAll(memoryDir, 0o755); err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("create memory directory: %w", err)
				}

				// Lock around the entire read-check-write-index cycle so
				// concurrent writers do not observe or produce a stale index.
				release, err := lockMemoryDir(ctx, dataDir)
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("acquire memory lock: %w", err)
				}
				defer release()

				if _, err := os.Stat(path); os.IsNotExist(err) {
					count, err := countMemoryFiles(memoryDir)
					if err != nil {
						return fantasy.ToolResponse{}, fmt.Errorf("count memory files: %w", err)
					}
					if count >= maxMemoryFiles {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("memory limit of %d files reached; delete or consolidate existing memories first", maxMemoryFiles)), nil
					}
				} else if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("stat memory file: %w", err)
				}

				// Remember the prior state so an over-budget index can be
				// rolled back below.
				prev, hadPrev := []byte(nil), false
				if b, err := os.ReadFile(path); err == nil {
					prev, hadPrev = b, true
				}

				body := formatMemoryFile(params.Description, params.Content)
				if err := atomicWriteFile(path, []byte(body), 0o644); err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("write memory file: %w", err)
				}

				index, err := renderMemoryIndex(memoryDir)
				if err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("render memory index: %w", err)
				}
				if len(index) > MaxMemoryIndexBytes {
					// The prompt layer would truncate this entry away, so the
					// save would silently do nothing. Undo it and say so.
					if hadPrev {
						err = atomicWriteFile(path, prev, 0o644)
					} else {
						err = os.Remove(path)
					}
					if err != nil {
						return fantasy.ToolResponse{}, fmt.Errorf("roll back memory file: %w", err)
					}
					if err := regenerateMemoryIndex(memoryDir); err != nil {
						return fantasy.ToolResponse{}, fmt.Errorf("regenerate memory index: %w", err)
					}
					return fantasy.NewTextErrorResponse(fmt.Sprintf(
						"memory index would exceed %d bytes and this memory would not be visible to future sessions; not saved. Shorten the description, or delete or consolidate existing memories first",
						MaxMemoryIndexBytes,
					)), nil
				}

				if err := writeMemoryIndex(memoryDir, index); err != nil {
					return fantasy.ToolResponse{}, fmt.Errorf("regenerate memory index: %w", err)
				}

				return fantasy.NewTextResponse(fmt.Sprintf(
					`Saved memory "%s". The memory index in the system prompt refreshes on the next crush launch; rely on this conversation for the current session.`,
					params.Name,
				)), nil

			case "delete":
				if err := DeleteMemory(ctx, dataDir, params.Name); err != nil {
					if errors.Is(err, ErrMemoryNotFound) {
						return fantasy.NewTextErrorResponse(fmt.Sprintf("no memory named %s", params.Name)), nil
					}
					return fantasy.ToolResponse{}, err
				}

				return fantasy.NewTextResponse(fmt.Sprintf(`Deleted memory "%s".`, params.Name)), nil
			}

			return fantasy.NewTextErrorResponse("action must be save or delete"), nil
		},
	)
}

// ErrMemoryNotFound is returned by DeleteMemory when no such memory exists.
var ErrMemoryNotFound = errors.New("memory not found")

// DeleteMemory removes a memory and regenerates MEMORY.md under the memory
// directory lock. It is the ONLY supported way to delete a memory: the UI
// dialog and the memory_write tool both go through it so that a UI-side
// delete cannot race a tool-side save and lose an index update. dataDir is
// the project data directory (Options.DataDirectory).
func DeleteMemory(ctx context.Context, dataDir, slug string) error {
	if !memoryNameRe.MatchString(slug) {
		return fmt.Errorf("invalid memory name %q", slug)
	}
	memoryDir := filepath.Join(dataDir, "memory")

	release, err := lockMemoryDir(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("acquire memory lock: %w", err)
	}
	defer release()

	if err := os.Remove(filepath.Join(memoryDir, slug+".md")); err != nil {
		if os.IsNotExist(err) {
			return ErrMemoryNotFound
		}
		return fmt.Errorf("delete memory file: %w", err)
	}
	if err := regenerateMemoryIndex(memoryDir); err != nil {
		return fmt.Errorf("regenerate memory index: %w", err)
	}
	return nil
}

// RegenerateMemoryIndex rebuilds MEMORY.md from a directory scan under the
// memory directory lock. Used after a memory file is edited out of band (for
// example in the user's $EDITOR from the memory dialog), where the
// description in the frontmatter may have changed but the index still shows
// the old one.
func RegenerateMemoryIndex(ctx context.Context, dataDir string) error {
	release, err := lockMemoryDir(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("acquire memory lock: %w", err)
	}
	defer release()
	return regenerateMemoryIndex(filepath.Join(dataDir, "memory"))
}

// ExtractMemoryDescription returns the description: frontmatter value of a
// memory file body, or slug when there is no parseable frontmatter. Exported
// so the memory dialog renders descriptions with the same parser that builds
// the index, rather than a second copy that can drift from it.
func ExtractMemoryDescription(content, slug string) string {
	return extractMemoryDescription(content, slug)
}

func formatMemoryFile(description, content string) string {
	return fmt.Sprintf("---\ndescription: %s\n---\n\n%s", description, content)
}

func countMemoryFiles(memoryDir string) (int, error) {
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == memoryIndexFileName || !strings.HasSuffix(name, ".md") {
			continue
		}
		count++
	}
	return count, nil
}

// regenerateMemoryIndex rebuilds MEMORY.md from a directory scan of
// individual memory files. MEMORY.md is never the source of truth.
func regenerateMemoryIndex(memoryDir string) error {
	index, err := renderMemoryIndex(memoryDir)
	if err != nil {
		return err
	}
	return writeMemoryIndex(memoryDir, index)
}

func writeMemoryIndex(memoryDir, index string) error {
	if index == "" {
		return nil
	}
	index = clampMemoryIndex(index)
	return atomicWriteFile(filepath.Join(memoryDir, memoryIndexFileName), []byte(index), 0o644)
}

// clampMemoryIndex trims index to whole lines within MaxMemoryIndexBytes.
// The save path rejects an over-budget index outright, but delete and
// rollback regenerate the index unconditionally, and a memory directory
// populated out of band (by the write tool, by hand, or by an older build)
// can hold far more than maxMemoryFiles entries. Without this, deleting one
// memory from a 100k-entry directory writes a multi-megabyte MEMORY.md that
// the prompt layer can only read the first 16 KiB of anyway.
func clampMemoryIndex(index string) string {
	if len(index) <= MaxMemoryIndexBytes {
		return index
	}
	cut := index[:MaxMemoryIndexBytes]
	if i := strings.LastIndexByte(cut, '\n'); i >= 0 {
		cut = cut[:i+1]
	}
	return cut
}

// renderMemoryIndex builds the MEMORY.md body from a directory scan without
// writing it, so callers can check it against MaxMemoryIndexBytes first. It
// returns "" when the memory directory does not exist.
func renderMemoryIndex(memoryDir string) (string, error) {
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	type memoryEntry struct {
		name        string
		description string
	}
	var memories []memoryEntry

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == memoryIndexFileName || !strings.HasSuffix(name, ".md") {
			continue
		}
		slug := strings.TrimSuffix(name, ".md")
		b, err := os.ReadFile(filepath.Join(memoryDir, name))
		if err != nil {
			return "", err
		}
		desc := extractMemoryDescription(string(b), slug)
		memories = append(memories, memoryEntry{name: slug, description: desc})
	}

	slices.SortFunc(memories, func(a, b memoryEntry) int {
		return strings.Compare(a.name, b.name)
	})

	var sb strings.Builder
	sb.WriteString("# Memory index\n\n")
	for _, m := range memories {
		fmt.Fprintf(&sb, "- %s: %s\n", m.name, m.description)
	}

	return sb.String(), nil
}

// extractMemoryDescription returns the description: frontmatter value,
// or the slug if the file has no parseable frontmatter.
func extractMemoryDescription(content, slug string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return slug
	}
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "---" {
			break
		}
		if after, ok := strings.CutPrefix(line, "description:"); ok {
			desc := strings.TrimSpace(after)
			if desc != "" {
				return desc
			}
			break
		}
	}
	return slug
}

// lockMemoryDir acquires an exclusive file lock for the memory directory.
// The lock file lives in dataDir so it does not depend on the memory
// subdirectory itself existing. Use a short timeout so a stuck writer
// does not hang the tool forever.
func lockMemoryDir(ctx context.Context, dataDir string) (func(), error) {
	lockPath := filepath.Join(dataDir, "memory-write.lock")
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return lock.File(ctx, lockPath)
}

// atomicWriteFile writes data to path atomically by writing to a temporary
// file in the same directory and renaming it into place.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := renameWithRetry(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// renameRetries bounds the rename retry loop. Kept small and finite: a
// contended rename must not turn into an unbounded wait.
const (
	renameRetries = 20
	renameBackoff = 25 * time.Millisecond
)

// renameWithRetry renames tmp over path, retrying briefly on failure.
//
// On POSIX the rename succeeds first time and this is a plain os.Rename. On
// Windows it can fail with ERROR_ACCESS_DENIED while another handle has the
// destination open, which is exactly the case atomicWriteFile exists to
// serve: the prompt layer reads MEMORY.md at launch while the agent may be
// rewriting it. Without a retry that surfaces to the user as a failed
// memory_write ("write memory file: ... Access is denied") even though
// nothing is wrong. Bounded at renameRetries*renameBackoff, after which the
// original error is returned.
func renameWithRetry(tmp, path string) error {
	var err error
	for i := range renameRetries {
		if err = os.Rename(tmp, path); err == nil {
			return nil
		}
		if i < renameRetries-1 {
			time.Sleep(renameBackoff)
		}
	}
	return err
}
