package tools

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/fsext"
)

// regexCache provides thread-safe caching of compiled regex patterns
type regexCache struct {
	cache map[string]*regexp.Regexp
	mu    sync.RWMutex
}

// newRegexCache creates a new regex cache
func newRegexCache() *regexCache {
	return &regexCache{
		cache: make(map[string]*regexp.Regexp),
	}
}

// get retrieves a compiled regex from cache or compiles and caches it
func (rc *regexCache) get(pattern string) (*regexp.Regexp, error) {
	// Try to get from cache first (read lock)
	rc.mu.RLock()
	if regex, exists := rc.cache[pattern]; exists {
		rc.mu.RUnlock()
		return regex, nil
	}
	rc.mu.RUnlock()

	// Compile the regex (write lock)
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Double-check in case another goroutine compiled it while we waited
	if regex, exists := rc.cache[pattern]; exists {
		return regex, nil
	}

	// Compile and cache the regex
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	rc.cache[pattern] = regex
	return regex, nil
}

// Global regex cache instances
var (
	searchRegexCache = newRegexCache()
	globRegexCache   = newRegexCache()
	// Pre-compiled regex for glob conversion (used frequently)
	globBraceRegex = regexp.MustCompile(`\{([^}]+)\}`)
)

type GrepParams struct {
	Pattern     string `json:"pattern"`
	Path        string `json:"path"`
	Include     string `json:"include"`
	LiteralText bool   `json:"literal_text"`
}

type grepMatch struct {
	path     string
	modTime  time.Time
	lineNum  int
	lineText string
}

type GrepResponseMetadata struct {
	NumberOfMatches int  `json:"number_of_matches"`
	Truncated       bool `json:"truncated"`
}

type grepTool struct {
	workingDir string
}

const GrepToolName = "grep"

//go:embed grep.md
var grepDescription []byte

func NewGrepTool(workingDir string) BaseTool {
	return &grepTool{
		workingDir: workingDir,
	}
}

func (g *grepTool) Name() string {
	return GrepToolName
}

func (g *grepTool) Info() ToolInfo {
	return ToolInfo{
		Name:        GrepToolName,
		Description: string(grepDescription),
		Parameters: map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regex pattern to search for in file contents",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "The directory to search in. Defaults to the current working directory.",
			},
			"include": map[string]any{
				"type":        "string",
				"description": "File pattern to include in the search (e.g. \"*.js\", \"*.{ts,tsx}\")",
			},
			"literal_text": map[string]any{
				"type":        "boolean",
				"description": "If true, the pattern will be treated as literal text with special regex characters escaped. Default is false.",
			},
		},
		Required: []string{"pattern"},
	}
}

// escapeRegexPattern escapes special regex characters so they're treated as literal characters
func escapeRegexPattern(pattern string) string {
	specialChars := []string{"\\", ".", "+", "*", "?", "(", ")", "[", "]", "{", "}", "^", "$", "|"}
	escaped := pattern

	for _, char := range specialChars {
		escaped = strings.ReplaceAll(escaped, char, "\\"+char)
	}

	return escaped
}

func (g *grepTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params GrepParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.Pattern == "" {
		return NewTextErrorResponse("pattern is required"), nil
	}

	// If literal_text is true, escape the pattern
	searchPattern := params.Pattern
	if params.LiteralText {
		searchPattern = escapeRegexPattern(params.Pattern)
	}

	searchPath := params.Path
	if searchPath == "" {
		searchPath = g.workingDir
	}

	matches, truncated, err := searchFiles(ctx, searchPattern, searchPath, params.Include, 100)
	if err != nil {
		return ToolResponse{}, fmt.Errorf("error searching files: %w", err)
	}

	var output strings.Builder
	if len(matches) == 0 {
		output.WriteString("No files found")
	} else {
		fmt.Fprintf(&output, "Found %d matches\n", len(matches))

		currentFile := ""
		for _, match := range matches {
			if currentFile != match.path {
				if currentFile != "" {
					output.WriteString("\n")
				}
				currentFile = match.path
				fmt.Fprintf(&output, "%s:\n", match.path)
			}
			if match.lineNum > 0 {
				fmt.Fprintf(&output, "  Line %d: %s\n", match.lineNum, match.lineText)
			} else {
				fmt.Fprintf(&output, "  %s\n", match.path)
			}
		}

		if truncated {
			output.WriteString("\n(Results are truncated. Consider using a more specific path or pattern.)")
		}
	}

	return WithResponseMetadata(
		NewTextResponse(output.String()),
		GrepResponseMetadata{
			NumberOfMatches: len(matches),
			Truncated:       truncated,
		},
	), nil
}

func searchFiles(ctx context.Context, pattern, rootPath, include string, limit int) ([]grepMatch, bool, error) {
	matches, err := searchWithRipgrep(ctx, pattern, rootPath, include)
	if err != nil {
		matches, err = searchFilesWithRegex(pattern, rootPath, include)
		if err != nil {
			return nil, false, err
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].modTime.After(matches[j].modTime)
	})

	truncated := len(matches) > limit
	if truncated {
		matches = matches[:limit]
	}

	return matches, truncated, nil
}

func searchWithRipgrep(ctx context.Context, pattern, path, include string) ([]grepMatch, error) {
	cmd := getRgSearchCmd(ctx, pattern, path, include)
	if cmd == nil {
		return nil, fmt.Errorf("ripgrep not found in $PATH")
	}

	// Only add ignore files if they exist
	for _, ignoreFile := range []string{".gitignore", ".crushignore"} {
		ignorePath := filepath.Join(path, ignoreFile)
		if _, err := os.Stat(ignorePath); err == nil {
			cmd.Args = append(cmd.Args, "--ignore-file", ignorePath)
		}
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []grepMatch{}, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	matches := make([]grepMatch, 0, len(lines))

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Parse ripgrep output format: file:line:content
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}

		filePath := parts[0]
		lineNum, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		lineText := parts[2]

		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue // Skip files we can't access
		}

		matches = append(matches, grepMatch{
			path:     filePath,
			modTime:  fileInfo.ModTime(),
			lineNum:  lineNum,
			lineText: lineText,
		})
	}

	return matches, nil
}

func searchFilesWithRegex(pattern, rootPath, include string) ([]grepMatch, error) {
	matches := []grepMatch{}

	// Use cached regex compilation
	regex, err := searchRegexCache.get(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var includePattern *regexp.Regexp
	if include != "" {
		regexPattern := globToRegex(include)
		includePattern, err = globRegexCache.get(regexPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid include pattern: %w", err)
		}
	}

	// Create walker with gitignore and crushignore support
	walker := fsext.NewFastGlobWalker(rootPath)

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			// Check if directory should be skipped
			if walker.ShouldSkip(path) {
				return filepath.SkipDir
			}
			return nil // Continue into directory
		}

		// Use walker's shouldSkip method for files
		if walker.ShouldSkip(path) {
			return nil
		}

		// Skip hidden files (starting with a dot) to match ripgrep's default behavior
		base := filepath.Base(path)
		if base != "." && strings.HasPrefix(base, ".") {
			return nil
		}

		if includePattern != nil && !includePattern.MatchString(path) {
			return nil
		}

		match, lineNum, lineText, err := fileContainsPattern(path, regex)
		if err != nil {
			return nil // Skip files we can't read
		}

		if match {
			matches = append(matches, grepMatch{
				path:     path,
				modTime:  info.ModTime(),
				lineNum:  lineNum,
				lineText: lineText,
			})

			if len(matches) >= 200 {
				return filepath.SkipAll
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return matches, nil
}

func fileContainsPattern(filePath string, pattern *regexp.Regexp) (bool, int, string, error) {
	// Quick binary file detection
	if isBinaryFile(filePath) {
		return false, 0, "", nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return false, 0, "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if pattern.MatchString(line) {
			return true, lineNum, line, nil
		}
	}

	return false, 0, "", scanner.Err()
}

var binaryExts = map[string]struct{}{
	".exe": {}, ".dll": {}, ".so": {}, ".dylib": {},
	".bin": {}, ".obj": {}, ".o": {}, ".a": {},
	".zip": {}, ".tar": {}, ".gz": {}, ".bz2": {},
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {},
	".pdf": {}, ".doc": {}, ".docx": {}, ".xls": {},
	".mp3": {}, ".mp4": {}, ".avi": {}, ".mov": {},
}

// isBinaryFile performs a quick check to determine if a file is binary
func isBinaryFile(filePath string) bool {
	// Check file extension first (fastest)
	ext := strings.ToLower(filepath.Ext(filePath))
	if _, isBinary := binaryExts[ext]; isBinary {
		return true
	}

	// Quick content check for files without clear extensions
	file, err := os.Open(filePath)
	if err != nil {
		return false // If we can't open it, let the caller handle the error
	}
	defer file.Close()

	// Read first 512 bytes to check for null bytes
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false
	}

	// Check for null bytes (common in binary files)
	for i := range n {
		if buffer[i] == 0 {
			return true
		}
	}

	return false
}

func globToRegex(glob string) string {
	regexPattern := strings.ReplaceAll(glob, ".", "\\.")
	regexPattern = strings.ReplaceAll(regexPattern, "*", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "?", ".")

	// Use pre-compiled regex instead of compiling each time
	regexPattern = globBraceRegex.ReplaceAllStringFunc(regexPattern, func(match string) string {
		inner := match[1 : len(match)-1]
		return "(" + strings.ReplaceAll(inner, ",", "|") + ")"
	})

	return regexPattern
}
