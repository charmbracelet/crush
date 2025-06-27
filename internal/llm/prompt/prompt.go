package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/fur/provider"
)

type PromptID string

const (
	PromptCoder      PromptID = "coder"
	PromptTitle      PromptID = "title"
	PromptTask       PromptID = "task"
	PromptSummarizer PromptID = "summarizer"
	PromptDefault    PromptID = "default"
)

func GetPrompt(promptID PromptID, provider provider.InferenceProvider, contextPaths ...string) string {
	basePrompt := ""
	switch promptID {
	case PromptCoder:
		basePrompt = CoderPrompt(provider)
	case PromptTitle:
		basePrompt = TitlePrompt(provider)
	case PromptTask:
		basePrompt = TaskPrompt(provider)
	case PromptSummarizer:
		basePrompt = SummarizerPrompt(provider)
	default:
		basePrompt = "You are a helpful assistant"
	}
	return basePrompt
}

func getContextFromPaths(contextPaths []string) string {
	return processContextPaths(config.WorkingDirectory(), contextPaths)
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

			if strings.HasSuffix(p, "/") {
				filepath.WalkDir(filepath.Join(workDir, p), func(path string, d os.DirEntry, err error) error {
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
				fullPath := filepath.Join(workDir, p)

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
