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

			files, truncated, err := globFiles(searchCtx, params.Pattern, searchPath, 100)
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

func globFiles(ctx context.Context, pattern, searchPath string, limit int) ([]string, bool, error) {
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
