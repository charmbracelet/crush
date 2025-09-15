package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"

	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
)

// Client manages LSP file watching for a specific client
// It now delegates actual file watching to the GlobalWatcher
type Client struct {
	client        *lsp.Client
	name          string
	workspacePath string

	// File watchers registered by the server
	registrations *csync.Slice[protocol.FileSystemWatcher]
}

func init() {
	// Ensure the watcher is initialized with a reasonable file limit
	if _, err := Ulimit(); err != nil {
		slog.Error("Error setting file limit", "error", err)
	}
}

// New creates a new workspace watcher for the given client.
func New(name string, client *lsp.Client) *Client {
	return &Client{
		name:          name,
		client:        client,
		registrations: csync.NewSlice[protocol.FileSystemWatcher](),
	}
}

// register adds file watchers to track
func (w *Client) register(ctx context.Context, id string, watchers []protocol.FileSystemWatcher) {
	cfg := config.Get()

	w.registrations.Append(watchers...)

	if cfg.Options.DebugLSP {
		slog.Debug("Adding file watcher registrations",
			"id", id,
			"watchers", len(watchers),
			"total", w.registrations.Len(),
		)

		for i, watcher := range watchers {
			slog.Debug("Registration", "index", i+1)

			// Log the GlobPattern
			switch v := watcher.GlobPattern.Value.(type) {
			case string:
				slog.Debug("GlobPattern", "pattern", v)
			case protocol.RelativePattern:
				slog.Debug("GlobPattern", "pattern", v.Pattern)

				// Log BaseURI details
				switch u := v.BaseURI.Value.(type) {
				case string:
					slog.Debug("BaseURI", "baseURI", u)
				case protocol.DocumentURI:
					slog.Debug("BaseURI", "baseURI", u)
				default:
					slog.Debug("BaseURI", "baseURI", u)
				}
			default:
				slog.Debug("GlobPattern unknown type", "type", fmt.Sprintf("%T", v))
			}

			// Log WatchKind
			watchKind := protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
			if watcher.Kind != nil {
				watchKind = *watcher.Kind
			}

			slog.Debug("WatchKind", "kind", watchKind)
		}
	}

	// For servers that need file preloading, open high-priority files only
	if shouldPreloadFiles(w.name) {
		go func() {
			highPriorityFilesOpened := w.openHighPriorityFiles(ctx, w.name)
			if cfg.Options.DebugLSP {
				slog.Debug("Opened high-priority files",
					"count", highPriorityFilesOpened,
					"serverName", w.name)
			}
		}()
	}
}

// openHighPriorityFiles opens important files for the server type
// Returns the number of files opened
func (w *Client) openHighPriorityFiles(ctx context.Context, serverName string) int {
	cfg := config.Get()
	filesOpened := 0

	// Define patterns for high-priority files based on server type
	var patterns []string

	// TODO: move this to LSP config
	switch serverName {
	case "typescript", "typescript-language-server", "tsserver", "vtsls":
		patterns = []string{
			"**/tsconfig.json",
			"**/package.json",
			"**/jsconfig.json",
			"**/index.ts",
			"**/index.js",
			"**/main.ts",
			"**/main.js",
		}
	case "gopls":
		patterns = []string{
			"**/go.mod",
			"**/go.sum",
			"**/main.go",
		}
	case "rust-analyzer":
		patterns = []string{
			"**/Cargo.toml",
			"**/Cargo.lock",
			"**/src/lib.rs",
			"**/src/main.rs",
		}
	case "python", "pyright", "pylsp":
		patterns = []string{
			"**/pyproject.toml",
			"**/setup.py",
			"**/requirements.txt",
			"**/__init__.py",
			"**/__main__.py",
		}
	case "clangd":
		patterns = []string{
			"**/CMakeLists.txt",
			"**/Makefile",
			"**/compile_commands.json",
		}
	case "java", "jdtls":
		patterns = []string{
			"**/pom.xml",
			"**/build.gradle",
			"**/src/main/java/**/*.java",
		}
	default:
		// For unknown servers, use common configuration files
		patterns = []string{
			"**/package.json",
			"**/Makefile",
			"**/CMakeLists.txt",
			"**/.editorconfig",
		}
	}

	// Collect all files to open first
	var filesToOpen []string

	// For each pattern, find matching files
	for _, pattern := range patterns {
		// Use doublestar.Glob to find files matching the pattern (supports ** patterns)
		matches, err := doublestar.Glob(os.DirFS(w.workspacePath), pattern)
		if err != nil {
			if cfg.Options.DebugLSP {
				slog.Debug("Error finding high-priority files", "pattern", pattern, "error", err)
			}
			continue
		}

		for _, match := range matches {
			// Convert relative path to absolute
			fullPath := filepath.Join(w.workspacePath, match)

			// Skip directories and excluded files
			info, err := os.Stat(fullPath)
			if err != nil || info.IsDir() || shouldExcludeFile(fullPath) {
				continue
			}

			filesToOpen = append(filesToOpen, fullPath)

			// Limit the number of files per pattern
			if len(filesToOpen) >= 5 && (serverName != "java" && serverName != "jdtls") {
				break
			}
		}
	}

	// Open files in batches to reduce overhead
	batchSize := 3
	for i := 0; i < len(filesToOpen); i += batchSize {
		end := min(i+batchSize, len(filesToOpen))

		// Open batch of files
		for j := i; j < end; j++ {
			fullPath := filesToOpen[j]
			if err := w.client.OpenFile(ctx, fullPath); err != nil {
				if cfg.Options.DebugLSP {
					slog.Debug("Error opening high-priority file", "path", fullPath, "error", err)
				}
			} else {
				filesOpened++
				if cfg.Options.DebugLSP {
					slog.Debug("Opened high-priority file", "path", fullPath)
				}
			}
		}

		// Only add delay between batches, not individual files
		if end < len(filesToOpen) {
			time.Sleep(50 * time.Millisecond)
		}
	}

	return filesOpened
}

// Watch sets up file watching for a workspace using the global watcher
func (w *Client) Watch(ctx context.Context, workspacePath string) {
	w.workspacePath = workspacePath

	slog.Debug("Starting workspace watcher", "workspacePath", workspacePath, "serverName", w.name)

	// Register this workspace watcher with the global watcher
	instance().register(w.name, w)
	defer instance().unregister(w.name)

	// Register handler for file watcher registrations from the server
	lsp.RegisterFileWatchHandler(func(id string, watchers []protocol.FileSystemWatcher) {
		w.register(ctx, id, watchers)
	})

	// Wait for context cancellation
	<-ctx.Done()
	slog.Debug("Workspace watcher stopped", "name", w.name)
}

// isPathWatched checks if a path should be watched based on server registrations
// If no explicit registrations, watch everything
func (w *Client) isPathWatched(path string) (bool, protocol.WatchKind) {
	if w.registrations.Len() == 0 {
		return true, protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
	}

	// Check each registration
	for reg := range w.registrations.Seq() {
		isMatch := w.matchesPattern(path, reg.GlobPattern)
		if isMatch {
			kind := protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
			if reg.Kind != nil {
				kind = *reg.Kind
			}
			return true, kind
		}
	}

	return false, 0
}

// matchesGlob handles glob patterns using the doublestar library
func matchesGlob(pattern, path string) bool {
	// Use doublestar for all glob matching - it handles ** and other complex patterns
	matched, err := doublestar.Match(pattern, path)
	if err != nil {
		slog.Error("Error matching pattern", "pattern", pattern, "path", path, "error", err)
		return false
	}
	return matched
}

// matchesPattern checks if a path matches the glob pattern
func (w *Client) matchesPattern(path string, pattern protocol.GlobPattern) bool {
	patternInfo, err := pattern.AsPattern()
	if err != nil {
		slog.Error("Error parsing pattern", "pattern", pattern, "error", err)
		return false
	}

	basePath := patternInfo.GetBasePath()
	patternText := patternInfo.GetPattern()

	path = filepath.ToSlash(path)

	// For simple patterns without base path
	if basePath == "" {
		// Check if the pattern matches the full path or just the file extension
		fullPathMatch := matchesGlob(patternText, path)
		baseNameMatch := matchesGlob(patternText, filepath.Base(path))

		return fullPathMatch || baseNameMatch
	}

	if basePath == "" {
		return false
	}

	// Make path relative to basePath for matching
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		slog.Error("Error getting relative path", "path", path, "basePath", basePath, "error", err, "server", w.name)
		return false
	}
	relPath = filepath.ToSlash(relPath)

	isMatch := matchesGlob(patternText, relPath)

	return isMatch
}

// notifyFileEvent sends a didChangeWatchedFiles notification for a file event
func (w *Client) notifyFileEvent(ctx context.Context, uri string, changeType protocol.FileChangeType) error {
	cfg := config.Get()
	if cfg.Options.DebugLSP {
		slog.Debug("Notifying file event",
			"uri", uri,
			"changeType", changeType,
		)
	}

	params := protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{
			{
				URI:  protocol.DocumentURI(uri),
				Type: changeType,
			},
		},
	}

	return w.client.DidChangeWatchedFiles(ctx, params)
}

// shouldPreloadFiles determines if we should preload files for a specific language server
// Some servers work better with preloaded files, others don't need it
func shouldPreloadFiles(serverName string) bool {
	// TypeScript/JavaScript servers typically need some files preloaded
	// to properly resolve imports and provide intellisense
	switch serverName {
	case "typescript", "typescript-language-server", "tsserver", "vtsls":
		return true
	case "java", "jdtls":
		// Java servers often need to see source files to build the project model
		return true
	default:
		// For most servers, we'll use lazy loading by default
		return false
	}
}

// Common patterns for directories and files to exclude
// TODO: make configurable
var (
	excludedFileExtensions = map[string]bool{
		".swp":   true,
		".swo":   true,
		".tmp":   true,
		".temp":  true,
		".bak":   true,
		".log":   true,
		".o":     true, // Object files
		".so":    true, // Shared libraries
		".dylib": true, // macOS shared libraries
		".dll":   true, // Windows shared libraries
		".a":     true, // Static libraries
		".exe":   true, // Windows executables
		".lock":  true, // Lock files
	}

	// Large binary files that shouldn't be opened
	largeBinaryExtensions = map[string]bool{
		".png":  true,
		".jpg":  true,
		".jpeg": true,
		".gif":  true,
		".bmp":  true,
		".ico":  true,
		".zip":  true,
		".tar":  true,
		".gz":   true,
		".rar":  true,
		".7z":   true,
		".pdf":  true,
		".mp3":  true,
		".mp4":  true,
		".mov":  true,
		".wav":  true,
		".wasm": true,
	}

	// Maximum file size to open (5MB)
	maxFileSize int64 = 5 * 1024 * 1024
)

// shouldExcludeFile returns true if the file should be excluded from opening
func shouldExcludeFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	cfg := config.Get()

	// Skip dot files
	if strings.HasPrefix(fileName, ".") {
		return true
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if excludedFileExtensions[ext] || largeBinaryExtensions[ext] {
		return true
	}

	info, err := os.Stat(filePath)
	if err != nil {
		// If we can't stat the file, skip it
		return true
	}

	// Skip large files
	if info.Size() > maxFileSize {
		if cfg.Options.DebugLSP {
			slog.Debug("Skipping large file",
				"path", filePath,
				"size", info.Size(),
				"maxSize", maxFileSize,
				"debug", cfg.Options.Debug,
				"sizeMB", float64(info.Size())/(1024*1024),
				"maxSizeMB", float64(maxFileSize)/(1024*1024),
			)
		}
		return true
	}

	return false
}

// openMatchingFile opens a file if it matches any of the registered patterns
func (w *Client) openMatchingFile(ctx context.Context, path string) {
	cfg := config.Get()
	// Skip directories
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}

	// Skip excluded files
	if shouldExcludeFile(path) {
		return
	}

	// Check if this path should be watched according to server registrations
	if watched, _ := w.isPathWatched(path); !watched {
		return
	}

	serverName := w.name

	// Get server name for specialized handling
	// Check if the file is a high-priority file that should be opened immediately
	// This helps with project initialization for certain language servers
	if isHighPriorityFile(path, serverName) {
		if cfg.Options.DebugLSP {
			slog.Debug("Opening high-priority file", "path", path, "serverName", serverName)
		}
		if err := w.client.OpenFile(ctx, path); err != nil && cfg.Options.DebugLSP {
			slog.Error("Error opening high-priority file", "path", path, "error", err)
		}
		return
	}

	// For non-high-priority files, we'll use different strategies based on server type
	if !shouldPreloadFiles(serverName) {
		return
	}
	// For servers that benefit from preloading, open files but with limits

	// Check file size - for preloading we're more conservative
	if info.Size() > (1 * 1024 * 1024) { // 1MB limit for preloaded files
		if cfg.Options.DebugLSP {
			slog.Debug("Skipping large file for preloading", "path", path, "size", info.Size())
		}
		return
	}

	// File type is already validated by HandlesFile() and isPathWatched() checks earlier,
	// so we know this client handles this file type. Just open it.
	if err := w.client.OpenFile(ctx, path); err != nil && cfg.Options.DebugLSP {
		slog.Error("Error opening file", "path", path, "error", err)
	}
}

// isHighPriorityFile determines if a file should be opened immediately
// regardless of the preloading strategy
func isHighPriorityFile(path string, serverName string) bool {
	fileName := filepath.Base(path)
	ext := filepath.Ext(path)

	switch serverName {
	case "typescript", "typescript-language-server", "tsserver", "vtsls":
		// For TypeScript, we want to open configuration files immediately
		return fileName == "tsconfig.json" ||
			fileName == "package.json" ||
			fileName == "jsconfig.json" ||
			// Also open main entry points
			fileName == "index.ts" ||
			fileName == "index.js" ||
			fileName == "main.ts" ||
			fileName == "main.js"
	case "gopls":
		// For Go, we want to open go.mod files immediately
		return fileName == "go.mod" ||
			fileName == "go.sum" ||
			// Also open main.go files
			fileName == "main.go"
	case "rust-analyzer":
		// For Rust, we want to open Cargo.toml files immediately
		return fileName == "Cargo.toml" ||
			fileName == "Cargo.lock" ||
			// Also open lib.rs and main.rs
			fileName == "lib.rs" ||
			fileName == "main.rs"
	case "python", "pyright", "pylsp":
		// For Python, open key project files
		return fileName == "pyproject.toml" ||
			fileName == "setup.py" ||
			fileName == "requirements.txt" ||
			fileName == "__init__.py" ||
			fileName == "__main__.py"
	case "clangd":
		// For C/C++, open key project files
		return fileName == "CMakeLists.txt" ||
			fileName == "Makefile" ||
			fileName == "compile_commands.json"
	case "java", "jdtls":
		// For Java, open key project files
		return fileName == "pom.xml" ||
			fileName == "build.gradle" ||
			ext == ".java" // Java servers often need to see source files
	}

	// For unknown servers, prioritize common configuration files
	return fileName == "package.json" ||
		fileName == "Makefile" ||
		fileName == "CMakeLists.txt" ||
		fileName == ".editorconfig"
}
