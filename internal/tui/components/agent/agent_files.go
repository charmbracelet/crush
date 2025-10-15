package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/env"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/tui/components/core"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
)

type RenderOptions struct {
	MaxWidth    int
	MaxItems    int
	ShowSection bool
	SectionName string
}

// expandPath expands ~ and environment variables in file paths
func expandPath(path string) string {
	path = home.Long(path)
	if strings.HasPrefix(path, "$") {
		resolver := config.NewEnvironmentVariableResolver(env.New())
		if expanded, err := resolver.ResolveValue(path); err == nil {
			path = expanded
		}
	}
	return path
}

// CollectContextFileNames returns all unique file names (not full paths)
// from the given context paths, after expansion and deduplication.
func CollectContextFileNames(workDir string, paths []string) []string {
	var (
		wg       sync.WaitGroup
		resultCh = make(chan string)
	)
	processedFiles := csync.NewMap[string, bool]()
	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			p = expandPath(p)
			fullPath := p
			if !filepath.IsAbs(p) {
				fullPath = filepath.Join(workDir, p)
			}
			info, err := os.Stat(fullPath)
			if err != nil {
				return
			}
			if info.IsDir() {
				filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() {
						lowerPath := strings.ToLower(path)
						if alreadyProcessed, _ := processedFiles.Get(lowerPath); !alreadyProcessed {
							processedFiles.Set(lowerPath, true)
							resultCh <- filepath.Base(path)
						}
					}
					return nil
				})
			} else {
				lowerPath := strings.ToLower(fullPath)
				if alreadyProcessed, _ := processedFiles.Get(lowerPath); !alreadyProcessed {
					processedFiles.Set(lowerPath, true)
					resultCh <- filepath.Base(fullPath)
				}
			}
		}(path)
	}
	go func() {
		wg.Wait()
		close(resultCh)
	}()
	fileNameSet := make(map[string]struct{})
	fileList := []string{}
	for name := range resultCh {
		// Deduplicate filenames (case-insensitive)
		lowerName := strings.ToLower(name)
		if _, exists := fileNameSet[lowerName]; !exists {
			fileNameSet[lowerName] = struct{}{}
			fileList = append(fileList, name)
		}
	}
	return fileList
}

func RenderAgentFilesBlock(opts RenderOptions) string {
	t := styles.CurrentTheme()
	lines := []string{}

	// Section header (same style approach as LSPs)
	if opts.ShowSection {
		sectionName := opts.SectionName
		if sectionName == "" {
			sectionName = "Agent Files"
		}
		lines = append(lines, t.S().Subtle.Render(sectionName), "")
	}

	cfg := config.Get()
	agentFiles := CollectContextFileNames(cfg.WorkingDir(), cfg.Options.ContextPaths)

	if len(agentFiles) == 0 {
		// “None” styled like LSPs
		lines = append(lines, t.S().Base.Foreground(t.Border).Render("None"))
	} else {
		// Stable order like LSP list uses deterministic ordering
		sort.Strings(agentFiles)

		// Determine how many items to show (avoid relying on min helper)
		maxItems := len(agentFiles)
		if opts.MaxItems > 0 && opts.MaxItems < maxItems {
			maxItems = opts.MaxItems
		}

		// Render each row using core.Status with a themed status dot
		for i := 0; i < maxItems; i++ {
			f := agentFiles[i]
			lines = append(lines, core.Status(
				core.StatusOpts{
					Icon:        t.ItemOnlineIcon.String(), 
					Title:       f,                        
				},
				opts.MaxWidth,
			))
		}

		// Truncation indicator consistent with LSPs
		if len(agentFiles) > maxItems && opts.MaxItems > 0 {
			remaining := len(agentFiles) - maxItems
			if remaining == 1 {
				lines = append(lines, t.S().Base.Foreground(t.FgMuted).Render("…"))
			} else {
				lines = append(lines, t.S().Base.Foreground(t.FgSubtle).Render(
					fmt.Sprintf("…and %d more", remaining),
				))
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	if opts.MaxWidth > 0 {
		return lipgloss.NewStyle().Width(opts.MaxWidth).Render(content)
	}
	return content
}
