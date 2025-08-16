package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/permission"
)

type ContextTool struct {
	permissions permission.Service
	workingDir  string
}

func NewContextTool(permissions permission.Service, workingDir string) BaseTool {
	return &ContextTool{
		permissions: permissions,
		workingDir:  workingDir,
	}
}

func (t *ContextTool) Name() string {
	return "context"
}

func (t *ContextTool) Info() ToolInfo {
	return ToolInfo{
		Name: "context",
		Description: `Load project context from configured context paths on-demand. This tool allows you to access project-specific context files (like CLAUDE.md, .cursorrules, etc.) only when needed, rather than including them in every request.

Use this tool when you need to:
- Understand project-specific conventions or instructions
- Access documentation that might guide your work
- Check for existing project rules or patterns

The tool loads context from the configured context paths in the project configuration.`,
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"paths": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional: Specific context paths to load. If not provided, loads from configured context paths.",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Optional: Filter context content by this query string to find relevant sections.",
				},
			},
		},
	}
}

type ContextInput struct {
	Paths []string `json:"paths,omitempty"`
	Query string   `json:"query,omitempty"`
}

func (t *ContextTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var input ContextInput
	if err := json.Unmarshal([]byte(call.Input), &input); err != nil {
		response := NewTextResponse(fmt.Sprintf("Error parsing input: %v", err))
		response.IsError = true
		return response, nil
	}

	// Get context paths to load
	contextPaths := input.Paths
	if len(contextPaths) == 0 {
		// Use configured context paths if none specified
		cfg := config.Get()
		if cfg.Options != nil && len(cfg.Options.ContextPaths) > 0 {
			contextPaths = cfg.Options.ContextPaths
		} else {
			// Use default context paths
			contextPaths = []string{
				"CLAUDE.md",
				"CLAUDE.local.md", 
				".cursorrules",
				"crush.md",
				"CRUSH.md",
			}
		}
	}

	// Convert relative paths to absolute paths
	absolutePaths := make([]string, 0, len(contextPaths))
	for _, path := range contextPaths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(t.workingDir, path)
		}
		absolutePaths = append(absolutePaths, path)
	}

	// Load context content
	contextContent := processContextPaths(t.workingDir, absolutePaths)
	
	if contextContent == "" {
		return NewTextResponse("No context files found at the specified paths."), nil
	}

	// Apply query filter if provided
	if input.Query != "" {
		lines := strings.Split(contextContent, "\n")
		var filteredLines []string
		var currentFile string
		var inRelevantFile bool
		
		for _, line := range lines {
			if strings.HasPrefix(line, "# From:") {
				currentFile = line
				inRelevantFile = false
				// Check if this file might contain relevant content
				// We include the file header if any part of the file matches
			} else if strings.Contains(strings.ToLower(line), strings.ToLower(input.Query)) {
				if !inRelevantFile && currentFile != "" {
					filteredLines = append(filteredLines, currentFile)
					inRelevantFile = true
				}
				filteredLines = append(filteredLines, line)
			} else if inRelevantFile {
				// Include some context around matches
				filteredLines = append(filteredLines, line)
			}
		}
		
		if len(filteredLines) > 0 {
			contextContent = strings.Join(filteredLines, "\n")
		} else {
			contextContent = fmt.Sprintf("No content found matching query: %s", input.Query)
		}
	}

	response := NewTextResponse(fmt.Sprintf("Project Context:\n\n%s", contextContent))
	
	// Add metadata as JSON string
	response.Metadata = fmt.Sprintf(`{"paths_loaded": %d, "query": "%s", "content_size": %d}`, 
		len(absolutePaths), input.Query, len(contextContent))
	
	return response, nil
}

// processContextPaths processes context files from given paths
// This is a copy of the function from prompt package to avoid import cycles
func processContextPaths(workDir string, paths []string) string {
	var (
		wg       sync.WaitGroup
		resultCh = make(chan string)
	)

	// Track processed files to avoid duplicates
	processedFiles := csync.NewMap[string, bool]()

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

						if alreadyProcessed, _ := processedFiles.Get(lowerPath); !alreadyProcessed {
							processedFiles.Set(lowerPath, true)
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

				if alreadyProcessed, _ := processedFiles.Get(lowerPath); !alreadyProcessed {
					processedFiles.Set(lowerPath, true)
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

	// Handle environment variable expansion
	if strings.HasPrefix(path, "$") {
		// Basic env var expansion - just the simple case
		if expanded := os.Getenv(path[1:]); expanded != "" {
			path = expanded
		}
	}

	return path
}