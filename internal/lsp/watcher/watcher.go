package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/logging"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/lsp/protocol"
	"github.com/fsnotify/fsnotify"
)

// WorkspaceWatcher manages LSP file watching
type WorkspaceWatcher struct {
	client        *lsp.Client
	workspacePath string

	debounceTime time.Duration
	debounceMap  map[string]*time.Timer
	debounceMu   sync.Mutex

	// File watchers registered by the server
	registrations  []protocol.FileSystemWatcher
	registrationMu sync.RWMutex
}

// NewWorkspaceWatcher creates a new workspace watcher
func NewWorkspaceWatcher(client *lsp.Client) *WorkspaceWatcher {
	return &WorkspaceWatcher{
		client:        client,
		debounceTime:  300 * time.Millisecond,
		debounceMap:   make(map[string]*time.Timer),
		registrations: []protocol.FileSystemWatcher{},
	}
}

// AddRegistrations adds file watchers to track
func (w *WorkspaceWatcher) AddRegistrations(ctx context.Context, id string, watchers []protocol.FileSystemWatcher) {
	cfg := config.Get()

	logging.Debug("Adding file watcher registrations")
	w.registrationMu.Lock()
	defer w.registrationMu.Unlock()

	// Add new watchers
	w.registrations = append(w.registrations, watchers...)

	// Print detailed registration information for debugging
	if cfg.Options.DebugLSP {
		logging.Debug("Adding file watcher registrations",
			"id", id,
			"watchers", len(watchers),
			"total", len(w.registrations),
		)

		for i, watcher := range watchers {
			logging.Debug("Registration", "index", i+1)

			// Log the GlobPattern
			switch v := watcher.GlobPattern.Value.(type) {
			case string:
				logging.Debug("GlobPattern", "pattern", v)
			case protocol.RelativePattern:
				logging.Debug("GlobPattern", "pattern", v.Pattern)

				// Log BaseURI details
				switch u := v.BaseURI.Value.(type) {
				case string:
					logging.Debug("BaseURI", "baseURI", u)
				case protocol.DocumentUri:
					logging.Debug("BaseURI", "baseURI", u)
				default:
					logging.Debug("BaseURI", "baseURI", u)
				}
			default:
				logging.Debug("GlobPattern", "unknown type", fmt.Sprintf("%T", v))
			}

			// Log WatchKind
			watchKind := protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
			if watcher.Kind != nil {
				watchKind = *watcher.Kind
			}

			logging.Debug("WatchKind", "kind", watchKind)
		}
	}

	// Determine server type for specialized handling
	serverName := getServerNameFromContext(ctx)
	logging.Debug("Server type detected", "serverName", serverName)

	// Check if this server has sent file watchers
	hasFileWatchers := len(watchers) > 0

	// For servers that need file preloading, we'll use a smart approach
	if shouldPreloadFiles(serverName) || !hasFileWatchers {
		go func() {
			startTime := time.Now()
			filesOpened := 0

			// Determine max files to open based on server type
			maxFilesToOpen := 50 // Default conservative limit

			switch serverName {
			case "typescript", "typescript-language-server", "tsserver", "vtsls":
				// TypeScript servers benefit from seeing more files
				maxFilesToOpen = 100
			case "java", "jdtls":
				// Java servers need to see many files for project model
				maxFilesToOpen = 200
			}

			// First, open high-priority files
			highPriorityFilesOpened := w.openHighPriorityFiles(ctx, serverName)
			filesOpened += highPriorityFilesOpened

			if cfg.Options.DebugLSP {
				logging.Debug("Opened high-priority files",
					"count", highPriorityFilesOpened,
					"serverName", serverName)
			}

			// If we've already opened enough high-priority files, we might not need more
			if filesOpened >= maxFilesToOpen {
				if cfg.Options.DebugLSP {
					logging.Debug("Reached file limit with high-priority files",
						"filesOpened", filesOpened,
						"maxFiles", maxFilesToOpen)
				}
				return
			}

			// For the remaining slots, walk the directory and open matching files

			err := filepath.WalkDir(w.workspacePath, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}

				// Skip directories that should be excluded
				if d.IsDir() {
					if path != w.workspacePath && shouldExcludeDir(path) {
						if cfg.Options.DebugLSP {
							logging.Debug("Skipping excluded directory", "path", path)
						}
						return filepath.SkipDir
					}
				} else {
					// Process files, but limit the total number
					if filesOpened < maxFilesToOpen {
						// Only process if it's not already open (high-priority files were opened earlier)
						if !w.client.IsFileOpen(path) {
							w.openMatchingFile(ctx, path)
							filesOpened++

							// Add a small delay after every 10 files to prevent overwhelming the server
							if filesOpened%10 == 0 {
								time.Sleep(50 * time.Millisecond)
							}
						}
					} else {
						// We've reached our limit, stop walking
						return filepath.SkipAll
					}
				}

				return nil
			})

			elapsedTime := time.Since(startTime)
			if cfg.Options.DebugLSP {
				logging.Debug("Limited workspace scan complete",
					"filesOpened", filesOpened,
					"maxFiles", maxFilesToOpen,
					"elapsedTime", elapsedTime.Seconds(),
					"workspacePath", w.workspacePath,
				)
			}

			if err != nil && cfg.Options.DebugLSP {
				logging.Debug("Error scanning workspace for files to open", "error", err)
			}
		}()
	} else if cfg.Options.DebugLSP {
		logging.Debug("Using on-demand file loading for server", "server", serverName)
	}
}

// openHighPriorityFiles opens important files for the server type
// Returns the number of files opened
func (w *WorkspaceWatcher) openHighPriorityFiles(ctx context.Context, serverName string) int {
	cfg := config.Get()
	filesOpened := 0

	// Define patterns for high-priority files based on server type
	var patterns []string

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
				logging.Debug("Error finding high-priority files", "pattern", pattern, "error", err)
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
					logging.Debug("Error opening high-priority file", "path", fullPath, "error", err)
				}
			} else {
				filesOpened++
				if cfg.Options.DebugLSP {
					logging.Debug("Opened high-priority file", "path", fullPath)
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

// WatchWorkspace sets up file watching for a workspace
func (w *WorkspaceWatcher) WatchWorkspace(ctx context.Context, workspacePath string) {
	cfg := config.Get()
	w.workspacePath = workspacePath

	// Store the watcher in the context for later use
	ctx = context.WithValue(ctx, "workspaceWatcher", w)

	// If the server name isn't already in the context, try to detect it
	if _, ok := ctx.Value("serverName").(string); !ok {
		serverName := getServerNameFromContext(ctx)
		ctx = context.WithValue(ctx, "serverName", serverName)
	}

	serverName := getServerNameFromContext(ctx)
	logging.Debug("Starting workspace watcher", "workspacePath", workspacePath, "serverName", serverName)

	// Register handler for file watcher registrations from the server
	lsp.RegisterFileWatchHandler(func(id string, watchers []protocol.FileSystemWatcher) {
		w.AddRegistrations(ctx, id, watchers)
	})

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logging.Error("Error creating watcher", "error", err)
	}
	defer watcher.Close()

	// Watch the workspace recursively
	err = filepath.WalkDir(workspacePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories (except workspace root)
		if d.IsDir() && path != workspacePath {
			if shouldExcludeDir(path) {
				if cfg.Options.DebugLSP {
					logging.Debug("Skipping excluded directory", "path", path)
				}
				return filepath.SkipDir
			}
		}

		// Add directories to watcher
		if d.IsDir() {
			err = watcher.Add(path)
			if err != nil {
				logging.Error("Error watching path", "path", path, "error", err)
			}
		}

		return nil
	})
	if err != nil {
		logging.Error("Error walking workspace", "error", err)
	}

	// Event loop
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			uri := string(protocol.URIFromPath(event.Name))

			// Add new directories to the watcher
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil {
					if info.IsDir() {
						// Skip excluded directories
						if !shouldExcludeDir(event.Name) {
							if err := watcher.Add(event.Name); err != nil {
								logging.Error("Error adding directory to watcher", "path", event.Name, "error", err)
							}
						}
					} else {
						// For newly created files
						if !shouldExcludeFile(event.Name) {
							w.openMatchingFile(ctx, event.Name)
						}
					}
				}
			}

			// Debug logging
			if cfg.Options.DebugLSP {
				matched, kind := w.isPathWatched(event.Name)
				logging.Debug("File event",
					"path", event.Name,
					"operation", event.Op.String(),
					"watched", matched,
					"kind", kind,
				)
			}

			// Check if this path should be watched according to server registrations
			if watched, watchKind := w.isPathWatched(event.Name); watched {
				switch {
				case event.Op&fsnotify.Write != 0:
					if watchKind&protocol.WatchChange != 0 {
						w.debounceHandleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Changed))
					}
				case event.Op&fsnotify.Create != 0:
					// Already handled earlier in the event loop
					// Just send the notification if needed
					info, err := os.Stat(event.Name)
					if err != nil {
						logging.Error("Error getting file info", "path", event.Name, "error", err)
						return
					}
					if !info.IsDir() && watchKind&protocol.WatchCreate != 0 {
						w.debounceHandleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Created))
					}
				case event.Op&fsnotify.Remove != 0:
					if watchKind&protocol.WatchDelete != 0 {
						w.handleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Deleted))
					}
				case event.Op&fsnotify.Rename != 0:
					// For renames, first delete
					if watchKind&protocol.WatchDelete != 0 {
						w.handleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Deleted))
					}

					// Then check if the new file exists and create an event
					if info, err := os.Stat(event.Name); err == nil && !info.IsDir() {
						if watchKind&protocol.WatchCreate != 0 {
							w.debounceHandleFileEvent(ctx, uri, protocol.FileChangeType(protocol.Created))
						}
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logging.Error("Error watching file", "error", err)
		}
	}
}

// isPathWatched checks if a path should be watched based on server registrations
func (w *WorkspaceWatcher) isPathWatched(path string) (bool, protocol.WatchKind) {
	w.registrationMu.RLock()
	defer w.registrationMu.RUnlock()

	// If no explicit registrations, watch everything
	if len(w.registrations) == 0 {
		return true, protocol.WatchKind(protocol.WatchChange | protocol.WatchCreate | protocol.WatchDelete)
	}

	// Check each registration
	for _, reg := range w.registrations {
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

// matchesGlob handles advanced glob patterns including ** and alternatives
func matchesGlob(pattern, path string) bool {
	// Handle file extension patterns with braces like *.{go,mod,sum}
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		// Extract extensions from pattern like "*.{go,mod,sum}"
		parts := strings.SplitN(pattern, "{", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			extPart := strings.SplitN(parts[1], "}", 2)
			if len(extPart) == 2 {
				extensions := strings.Split(extPart[0], ",")
				suffix := extPart[1]

				// Check if the path matches any of the extensions
				for _, ext := range extensions {
					extPattern := prefix + ext + suffix
					isMatch := matchesSimpleGlob(extPattern, path)
					if isMatch {
						return true
					}
				}
				return false
			}
		}
	}

	return matchesSimpleGlob(pattern, path)
}

// matchesSimpleGlob handles glob patterns with ** wildcards
func matchesSimpleGlob(pattern, path string) bool {
	// Handle special case for **/*.ext pattern (common in LSP)
	if strings.HasPrefix(pattern, "**/") {
		rest := strings.TrimPrefix(pattern, "**/")

		// If the rest is a simple file extension pattern like *.go
		if strings.HasPrefix(rest, "*.") {
			ext := strings.TrimPrefix(rest, "*")
			isMatch := strings.HasSuffix(path, ext)
			return isMatch
		}

		// Otherwise, try to check if the path ends with the rest part
		isMatch := strings.HasSuffix(path, rest)

		// If it matches directly, great!
		if isMatch {
			return true
		}

		// Otherwise, check if any path component matches
		pathComponents := strings.Split(path, "/")
		for i := range pathComponents {
			subPath := strings.Join(pathComponents[i:], "/")
			if strings.HasSuffix(subPath, rest) {
				return true
			}
		}

		return false
	}

	// Handle other ** wildcard pattern cases
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")

		// Validate the path starts with the first part
		if !strings.HasPrefix(path, parts[0]) && parts[0] != "" {
			return false
		}

		// For patterns like "**/*.go", just check the suffix
		if len(parts) == 2 && parts[0] == "" {
			isMatch := strings.HasSuffix(path, parts[1])
			return isMatch
		}

		// For other patterns, handle middle part
		remaining := strings.TrimPrefix(path, parts[0])
		if len(parts) == 2 {
			isMatch := strings.HasSuffix(remaining, parts[1])
			return isMatch
		}
	}

	// Handle simple * wildcard for file extension patterns (*.go, *.sum, etc)
	if strings.HasPrefix(pattern, "*.") {
		ext := strings.TrimPrefix(pattern, "*")
		isMatch := strings.HasSuffix(path, ext)
		return isMatch
	}

	// Fall back to simple matching for simpler patterns
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		logging.Error("Error matching pattern", "pattern", pattern, "path", path, "error", err)
		return false
	}

	return matched
}

// matchesPattern checks if a path matches the glob pattern
func (w *WorkspaceWatcher) matchesPattern(path string, pattern protocol.GlobPattern) bool {
	patternInfo, err := pattern.AsPattern()
	if err != nil {
		logging.Error("Error parsing pattern", "pattern", pattern, "error", err)
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

	// For relative patterns
	basePath = protocol.DocumentUri(basePath).Path()
	basePath = filepath.ToSlash(basePath)

	// Make path relative to basePath for matching
	relPath, err := filepath.Rel(basePath, path)
	if err != nil {
		logging.Error("Error getting relative path", "path", path, "basePath", basePath, "error", err)
		return false
	}
	relPath = filepath.ToSlash(relPath)

	isMatch := matchesGlob(patternText, relPath)

	return isMatch
}

// debounceHandleFileEvent handles file events with debouncing to reduce notifications
func (w *WorkspaceWatcher) debounceHandleFileEvent(ctx context.Context, uri string, changeType protocol.FileChangeType) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	// Create a unique key based on URI and change type
	key := fmt.Sprintf("%s:%d", uri, changeType)

	// Cancel existing timer if any
	if timer, exists := w.debounceMap[key]; exists {
		timer.Stop()
	}

	// Create new timer
	w.debounceMap[key] = time.AfterFunc(w.debounceTime, func() {
		w.handleFileEvent(ctx, uri, changeType)

		// Cleanup timer after execution
		w.debounceMu.Lock()
		delete(w.debounceMap, key)
		w.debounceMu.Unlock()
	})
}

// handleFileEvent sends file change notifications
func (w *WorkspaceWatcher) handleFileEvent(ctx context.Context, uri string, changeType protocol.FileChangeType) {
	// If the file is open and it's a change event, use didChange notification
	filePath := protocol.DocumentUri(uri).Path()
	if changeType == protocol.FileChangeType(protocol.Deleted) {
		w.client.ClearDiagnosticsForURI(protocol.DocumentUri(uri))
	} else if changeType == protocol.FileChangeType(protocol.Changed) && w.client.IsFileOpen(filePath) {
		err := w.client.NotifyChange(ctx, filePath)
		if err != nil {
			logging.Error("Error notifying change", "error", err)
		}
		return
	}

	// Notify LSP server about the file event using didChangeWatchedFiles
	if err := w.notifyFileEvent(ctx, uri, changeType); err != nil {
		logging.Error("Error notifying LSP server about file event", "error", err)
	}
}

// notifyFileEvent sends a didChangeWatchedFiles notification for a file event
func (w *WorkspaceWatcher) notifyFileEvent(ctx context.Context, uri string, changeType protocol.FileChangeType) error {
	cfg := config.Get()
	if cfg.Options.DebugLSP {
		logging.Debug("Notifying file event",
			"uri", uri,
			"changeType", changeType,
		)
	}

	params := protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{
			{
				URI:  protocol.DocumentUri(uri),
				Type: changeType,
			},
		},
	}

	return w.client.DidChangeWatchedFiles(ctx, params)
}

// getServerNameFromContext extracts the server name from the context
// This is a best-effort function that tries to identify which LSP server we're dealing with
func getServerNameFromContext(ctx context.Context) string {
	// First check if the server name is directly stored in the context
	if serverName, ok := ctx.Value("serverName").(string); ok && serverName != "" {
		return strings.ToLower(serverName)
	}

	// Otherwise, try to extract server name from the client command path
	if w, ok := ctx.Value("workspaceWatcher").(*WorkspaceWatcher); ok && w != nil && w.client != nil && w.client.Cmd != nil {
		path := strings.ToLower(w.client.Cmd.Path)

		// Extract server name from path
		if strings.Contains(path, "typescript") || strings.Contains(path, "tsserver") || strings.Contains(path, "vtsls") {
			return "typescript"
		} else if strings.Contains(path, "gopls") {
			return "gopls"
		} else if strings.Contains(path, "rust-analyzer") {
			return "rust-analyzer"
		} else if strings.Contains(path, "pyright") || strings.Contains(path, "pylsp") || strings.Contains(path, "python") {
			return "python"
		} else if strings.Contains(path, "clangd") {
			return "clangd"
		} else if strings.Contains(path, "jdtls") || strings.Contains(path, "java") {
			return "java"
		}

		// Return the base name as fallback
		return filepath.Base(path)
	}

	return "unknown"
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
	excludedDirNames = map[string]bool{
		".git":         true,
		"node_modules": true,
		"dist":         true,
		"build":        true,
		"out":          true,
		"bin":          true,
		".idea":        true,
		".vscode":      true,
		".cache":       true,
		"coverage":     true,
		"target":       true, // Rust build output
		"vendor":       true, // Go vendor directory
	}

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

// shouldExcludeDir returns true if the directory should be excluded from watching/opening
func shouldExcludeDir(dirPath string) bool {
	dirName := filepath.Base(dirPath)

	// Skip dot directories
	if strings.HasPrefix(dirName, ".") {
		return true
	}

	// Skip common excluded directories
	if excludedDirNames[dirName] {
		return true
	}

	return false
}

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

	// Skip temporary files
	if strings.HasSuffix(filePath, "~") {
		return true
	}

	// Check file size
	info, err := os.Stat(filePath)
	if err != nil {
		// If we can't stat the file, skip it
		return true
	}

	// Skip large files
	if info.Size() > maxFileSize {
		if cfg.Options.DebugLSP {
			logging.Debug("Skipping large file",
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
func (w *WorkspaceWatcher) openMatchingFile(ctx context.Context, path string) {
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
	if watched, _ := w.isPathWatched(path); watched {
		// Get server name for specialized handling
		serverName := getServerNameFromContext(ctx)

		// Check if the file is a high-priority file that should be opened immediately
		// This helps with project initialization for certain language servers
		if isHighPriorityFile(path, serverName) {
			if cfg.Options.DebugLSP {
				logging.Debug("Opening high-priority file", "path", path, "serverName", serverName)
			}
			if err := w.client.OpenFile(ctx, path); err != nil && cfg.Options.DebugLSP {
				logging.Error("Error opening high-priority file", "path", path, "error", err)
			}
			return
		}

		// For non-high-priority files, we'll use different strategies based on server type
		if shouldPreloadFiles(serverName) {
			// For servers that benefit from preloading, open files but with limits

			// Check file size - for preloading we're more conservative
			if info.Size() > (1 * 1024 * 1024) { // 1MB limit for preloaded files
				if cfg.Options.DebugLSP {
					logging.Debug("Skipping large file for preloading", "path", path, "size", info.Size())
				}
				return
			}

			// Check file extension for common source files
			ext := strings.ToLower(filepath.Ext(path))

			// Only preload source files for the specific language
			shouldOpen := false

			switch serverName {
			case "typescript", "typescript-language-server", "tsserver", "vtsls":
				shouldOpen = ext == ".ts" || ext == ".js" || ext == ".tsx" || ext == ".jsx"
			case "gopls":
				shouldOpen = ext == ".go"
			case "rust-analyzer":
				shouldOpen = ext == ".rs"
			case "python", "pyright", "pylsp":
				shouldOpen = ext == ".py"
			case "clangd":
				shouldOpen = ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".hpp"
			case "java", "jdtls":
				shouldOpen = ext == ".java"
			default:
				// For unknown servers, be conservative
				shouldOpen = false
			}

			if shouldOpen {
				// Don't need to check if it's already open - the client.OpenFile handles that
				if err := w.client.OpenFile(ctx, path); err != nil && cfg.Options.DebugLSP {
					logging.Error("Error opening file", "path", path, "error", err)
				}
			}
		}
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
