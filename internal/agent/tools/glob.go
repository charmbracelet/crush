package tools

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/charmbracelet/crush/internal/fsext"
)

const GlobToolName = "glob"

//go:embed glob.md.tpl
var globDescriptionTmpl []byte

var globDescriptionTpl = template.Must(
	template.New("globDescription").
		Parse(string(globDescriptionTmpl)),
)

type globDescriptionData struct {
	MaxResults int
}

func globDescription() string {
	return renderTemplate(globDescriptionTpl, globDescriptionData{
		MaxResults: 100,
	})
}

type GlobParams struct {
	Pattern string `json:"pattern" description:"The glob pattern to match files against"`
	Path    string `json:"path,omitempty" description:"The directory to search in. Defaults to the current working directory."`
}

type GlobResponseMetadata struct {
	NumberOfFiles int  `json:"number_of_files"`
	Truncated     bool `json:"truncated"`
}

func NewGlobTool(workingDir string, cfg config.ToolGlob) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		GlobToolName,
		globDescription(),
		func(ctx context.Context, params GlobParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.Pattern == "" {
				return fantasy.NewTextErrorResponse("pattern is required"), nil
			}

			searchPath := cmp.Or(params.Path, workingDir)

			// Bound the search so a huge or symlink-heavy root (e.g. $HOME
			// or a module cache) fails cleanly instead of pinning the CPU
			// and hanging the agent.
			searchCtx, cancel := context.WithTimeout(ctx, cfg.GetTimeout())
			defer cancel()

			files, truncated, err := globFiles(searchCtx, params.Pattern, searchPath, workingDir, cfg, 100)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("error finding files: %v", err)), nil
			}

			var output string
			if len(files) == 0 {
				output = "No files found"
			} else {
				normalizeFilePaths(files)
				output = strings.Join(files, "\n")
				if truncated {
					output += "\n\n(Results are truncated. Consider using a more specific path or pattern.)"
				}
			}

			return fantasy.WithResponseMetadata(
				fantasy.NewTextResponse(output),
				GlobResponseMetadata{
					NumberOfFiles: len(files),
					Truncated:     truncated,
				},
			), nil
		},
	)
}

func globFiles(ctx context.Context, pattern, searchPath, workingDir string, cfg config.ToolGlob, limit int) ([]string, bool, error) {
	// Scope the walk to the pattern's literal directory prefix. A pattern
	// like "internal/agent/*.go" only needs to walk "internal/agent", so we
	// start there instead of enumerating the entire tree and filtering.
	// Patterns that begin with a wildcard (e.g. "**/foo.go") have no prefix
	// and still walk from searchPath.
	prefix, rest := filepathext.SplitGlobPrefix(pattern)
	walkRoot := searchPath
	walkPattern := pattern
	if prefix != "" {
		walkRoot = filepath.Join(searchPath, prefix)
		walkPattern = rest
	}

	// Use locate if enabled and searching outside the working directory
	// (i.e., filesystem-wide search) for better performance.
	if cfg.GetUseLocate() && isOutsideWorkingDir(searchPath, workingDir) {
		matches, err := runLocate(ctx, walkPattern, limit)
		if err == nil {
			return matches, len(matches) >= limit && limit > 0, nil
		}
		slog.Warn("Locate execution failed, falling back to ripgrep", "error", err)
	}

	cmdRg := getRgCmd(ctx, walkPattern)
	if cmdRg != nil {
		cmdRg.Dir = walkRoot
		matches, err := runRipgrep(cmdRg, walkRoot, limit)
		if err == nil {
			return matches, len(matches) >= limit && limit > 0, nil
		}
		slog.Warn("Ripgrep execution failed, falling back to doublestar", "error", err)
	}

	return fsext.GlobGitignoreAwareCtx(ctx, walkPattern, walkRoot, limit)
}

// isOutsideWorkingDir returns true if the search path is outside the working directory.
// This includes absolute paths that are not under the working directory, or root paths.
func isOutsideWorkingDir(searchPath, workingDir string) bool {
	// Normalize paths
	searchPath = filepath.Clean(searchPath)
	workingDir = filepath.Clean(workingDir)

	// If searchPath is root or a top-level system directory, consider it outside
	if searchPath == "/" || searchPath == "\\" {
		return true
	}

	// Check if searchPath is under workingDir
	rel, err := filepath.Rel(workingDir, searchPath)
	if err != nil {
		return true
	}

	// If relative path starts with "..", it's outside the working directory
	return strings.HasPrefix(rel, "..")
}

func runRipgrep(cmd *exec.Cmd, searchRoot string, limit int) ([]string, error) {
	// Stream ripgrep's stdout instead of buffering the whole file list.
	// Over a huge root (e.g. $HOME) the full --files listing can be
	// hundreds of MB; reading it all at once and then sorting allocated
	// gigabytes. We read incrementally and stop once we have a bounded
	// pool of candidates.
	//
	// We collect more than `limit` so the shortest-path preference below
	// still has something to choose from, but the pool is capped so memory
	// stays small (a few thousand paths) no matter how large the tree is.
	candidatePool := max(limit*20, 1000)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ripgrep: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ripgrep: %w", err)
	}

	var matches []string
	reader := bufio.NewReader(stdout)
	for {
		path, err := reader.ReadString(0)
		if len(path) > 0 {
			path = strings.TrimRight(path, "\x00")
			if path != "" {
				absPath := filepathext.SmartJoin(searchRoot, path)
				if !fsext.SkipHidden(absPath) {
					matches = append(matches, absPath)
				}
			}
		}
		if err != nil {
			break // EOF or read error; drain handled by Wait below.
		}
		if len(matches) >= candidatePool {
			// Enough candidates; stop reading and let the process be
			// killed by the command context / Wait. Draining the rest
			// would just buffer paths we are going to discard.
			break
		}
	}

	// Close our end so ripgrep gets SIGPIPE and stops, then reap it.
	_ = stdout.Close()
	waitErr := cmd.Wait()
	if waitErr != nil && len(matches) == 0 {
		if ee, ok := waitErr.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return nil, nil // No matches.
		}
		return nil, fmt.Errorf("ripgrep: %w\n%s", waitErr, stderr.String())
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return len(matches[i]) < len(matches[j])
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func normalizeFilePaths(paths []string) {
	for i, p := range paths {
		paths[i] = filepath.ToSlash(p)
	}
}

// runLocate uses the locate command to find files matching the pattern.
// This is much faster for filesystem-wide searches as it uses a pre-built database.
func runLocate(ctx context.Context, pattern string, limit int) ([]string, error) {
	// Check if locate is available
	path, err := exec.LookPath("locate")
	if err != nil {
		return nil, fmt.Errorf("locate command not found: %w", err)
	}

	// Convert glob pattern to locate pattern
	// locate uses a different pattern syntax - we use -b for basename match
	// and -i for case-insensitive matching
	args := []string{"-b", "-i", "-l", fmt.Sprintf("%d", limit*2), pattern}
	cmd := exec.CommandContext(ctx, path, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("locate: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("locate: %w", err)
	}

	var matches []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		path := scanner.Text()
		if path != "" {
			matches = append(matches, path)
		}
		if len(matches) >= limit*2 {
			break
		}
	}

	_ = stdout.Close()
	waitErr := cmd.Wait()
	if waitErr != nil {
		if ee, ok := waitErr.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return matches, nil // No matches found
		}
		return nil, fmt.Errorf("locate: %w\n%s", waitErr, stderr.String())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("locate: %w", err)
	}

	// Sort by path length (shorter paths first, similar to ripgrep behavior)
	sort.SliceStable(matches, func(i, j int) bool {
		return len(matches[i]) < len(matches[j])
	})

	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}
