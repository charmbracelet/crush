package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/resolver"
)

type PromptID string

const (
	PromptCoder      PromptID = "coder"
	PromptTitle      PromptID = "title"
	PromptTask       PromptID = "task"
	PromptSummarizer PromptID = "summarizer"
	PromptDefault    PromptID = "default"
)

func getContextFromPaths(workingDir string, contextPaths []string) string {
	return processContextPaths(workingDir, contextPaths)
}

// expandPath expands ~ and environment variables in file paths
func expandPath(path string) string {
	// Handle tilde expansion
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
	} else if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = homeDir
		}
	}

	// Handle environment variable expansion using the same pattern as config
	if strings.HasPrefix(path, "$") {
		resolver := resolver.New()
		if expanded, err := resolver.ResolveValue(path); err == nil {
			path = expanded
		}
	}

	return path
}

func processContextPaths(workDir string, paths []string) string {
	var (
		wg       sync.WaitGroup
		resultCh = make(chan string)
	)

	// Track processed files to avoid duplicates
	processedFiles := make(map[string]bool)
	var processedMutex sync.Mutex

	for _, path := range paths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			// Expand ~ and environment variables before processing
			p = expandPath(p)

			// Use absolute path if provided, otherwise join with workDir
			fullPath := p
			if !filepath.IsAbs(p) {
				fullPath = filepath.Join(workDir, p)
			}

			// Check if the path is a directory using os.Stat
			info, err := os.Stat(fullPath)
			if err != nil {
				return // Skip if path doesn't exist or can't be accessed
			}

			if info.IsDir() {
				filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() {
						// Check if we've already processed this file (case-insensitive)
						lowerPath := strings.ToLower(path)

						processedMutex.Lock()
						alreadyProcessed := processedFiles[lowerPath]
						if !alreadyProcessed {
							processedFiles[lowerPath] = true
						}
						processedMutex.Unlock()

						if !alreadyProcessed {
							if result := processFile(path); result != "" {
								resultCh <- result
							}
						}
					}
					return nil
				})
			} else {
				// It's a file, process it directly
				// Check if we've already processed this file (case-insensitive)
				lowerPath := strings.ToLower(fullPath)

				processedMutex.Lock()
				alreadyProcessed := processedFiles[lowerPath]
				if !alreadyProcessed {
					processedFiles[lowerPath] = true
				}
				processedMutex.Unlock()

				if !alreadyProcessed {
					result := processFile(fullPath)
					if result != "" {
						resultCh <- result
					}
				}
			}
		}(path)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	results := make([]string, 0)
	for result := range resultCh {
		results = append(results, result)
	}

	return strings.Join(results, "\n")
}

func processFile(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	return "# From:" + filePath + "\n" + string(content)
}
