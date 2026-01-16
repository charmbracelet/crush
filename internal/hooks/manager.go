package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/csync"
)

type manager struct {
	workingDir string
	dataDir    string
	config     *Config
	executor   *Executor
	hooks      *csync.Map[HookType, []string]
}

// NewManager creates a new hook manager.
func NewManager(workingDir, dataDir string, cfg *Config) *manager {
	if cfg == nil {
		cfg = &Config{
			TimeoutSeconds: 30,
			Directories:    []string{filepath.Join(dataDir, "hooks")},
		}
	}

	defaultHooksDir := filepath.Join(dataDir, "hooks")

	// Ensure default directory if not specified.
	if len(cfg.Directories) == 0 {
		cfg.Directories = []string{defaultHooksDir}
	} else {
		// Always include default hooks directory even when user overrides config.
		if !slices.Contains(cfg.Directories, defaultHooksDir) {
			cfg.Directories = append([]string{defaultHooksDir}, cfg.Directories...)
		}
	}

	return &manager{
		workingDir: workingDir,
		dataDir:    dataDir,
		config:     cfg,
		executor:   NewExecutor(workingDir),
		hooks:      csync.NewMap[HookType, []string](),
	}
}

// isExecutable checks if a file is executable.
// On Unix: checks execute permission bits for .sh files.
// On Windows: only recognizes .sh extension (as we use POSIX shell emulator).
func isExecutable(info os.FileInfo) bool {
	name := strings.ToLower(info.Name())
	if !strings.HasSuffix(name, ".sh") {
		return false
	}

	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}

// executeHooks is the internal method that executes hooks for a given type.
func (m *manager) executeHooks(ctx context.Context, hookType HookType, hookContext HookContext) (HookResult, error) {
	if m.config.Disabled {
		return HookResult{Continue: true}, nil
	}

	hookContext.HookType = hookType
	hookContext.Environment = m.config.Environment

	hooks := m.discoverHooks(hookType)
	if len(hooks) == 0 {
		return HookResult{Continue: true}, nil
	}

	slog.Debug("Executing hooks", "type", hookType, "count", len(hooks))

	accumulated := HookResult{Continue: true}
	for _, hookPath := range hooks {
		if m.isDisabled(hookPath) {
			slog.Debug("Skipping disabled hook", "path", hookPath)
			continue
		}

		hookCtx, cancel := context.WithTimeout(ctx, time.Duration(m.config.TimeoutSeconds)*time.Second)

		result, err := m.executor.Execute(hookCtx, hookPath, hookContext)
		cancel()

		if err != nil {
			slog.Error("Hook execution failed", "path", hookPath, "error", err)
			if hookType == HookPreToolUse {
				accumulated.Continue = false
				accumulated.Permission = "deny"
				accumulated.Message = fmt.Sprintf("Hook failed: %v", err)
				return accumulated, nil
			}
			continue
		}

		if result.Message != "" {
			slog.Info("Hook message", "path", hookPath, "message", result.Message)
		}

		m.mergeResults(&accumulated, result)

		if !result.Continue {
			slog.Info("Hook stopped execution", "path", hookPath)
			break
		}
	}

	return accumulated, nil
}

// discoverHooks finds all executable hooks for the given type.
func (m *manager) discoverHooks(hookType HookType) []string {
	if cached, ok := m.hooks.Get(hookType); ok {
		return cached
	}

	var hooks []string

	for _, dir := range m.config.Directories {
		if _, err := os.Stat(dir); err == nil {
			entries, err := os.ReadDir(dir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}

					hookPath := filepath.Join(dir, entry.Name())

					info, err := entry.Info()
					if err != nil {
						continue
					}

					if !isExecutable(info) {
						continue
					}

					hooks = append(hooks, hookPath)
					slog.Debug("Discovered catch-all hook", "path", hookPath, "type", hookType)
				}
			}
		}

		hookDir := filepath.Join(dir, string(hookType))
		if _, err := os.Stat(hookDir); os.IsNotExist(err) {
			continue
		}

		entries, err := os.ReadDir(hookDir)
		if err != nil {
			slog.Error("Failed to read hooks directory", "dir", hookDir, "error", err)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			hookPath := filepath.Join(hookDir, entry.Name())

			info, err := entry.Info()
			if err != nil {
				continue
			}

			if !isExecutable(info) {
				slog.Debug("Skipping non-executable hook", "path", hookPath)
				continue
			}

			hooks = append(hooks, hookPath)
		}
	}

	if inlineHooks, ok := m.config.Inline[string(hookType)]; ok {
		for _, inline := range inlineHooks {
			hookPath, err := m.writeInlineHook(hookType, inline)
			if err != nil {
				slog.Error("Failed to write inline hook", "name", inline.Name, "error", err)
				continue
			}
			hooks = append(hooks, hookPath)
		}
	}

	sort.Strings(hooks)
	m.hooks.Set(hookType, hooks)
	return hooks
}

// writeInlineHook writes an inline hook script to a temp file.
func (m *manager) writeInlineHook(hookType HookType, inline InlineHook) (string, error) {
	tempDir := filepath.Join(m.dataDir, "hooks", ".inline", string(hookType))
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", err
	}

	hookPath := filepath.Join(tempDir, inline.Name)
	if err := os.WriteFile(hookPath, []byte(inline.Script), 0o755); err != nil {
		return "", err
	}

	return hookPath, nil
}

// isDisabled checks if a hook is in the disabled list.
func (m *manager) isDisabled(hookPath string) bool {
	for _, dir := range m.config.Directories {
		if rel, err := filepath.Rel(dir, hookPath); err == nil {
			// Normalize to forward slashes for cross-platform comparison
			rel = filepath.ToSlash(rel)
			if slices.Contains(m.config.DisableHooks, rel) {
				return true
			}
		}
	}
	return false
}

// mergeResults merges a new result into the accumulated result.
func (m *manager) mergeResults(accumulated *HookResult, new *HookResult) {
	accumulated.Continue = accumulated.Continue && new.Continue

	if new.Permission != "" {
		if new.Permission == "deny" {
			accumulated.Permission = "deny"
		} else if new.Permission == "approve" && accumulated.Permission == "" {
			accumulated.Permission = "approve"
		}
	}

	if new.ModifiedPrompt != nil {
		accumulated.ModifiedPrompt = new.ModifiedPrompt
	}

	if len(new.ModifiedInput) > 0 {
		if accumulated.ModifiedInput == nil {
			accumulated.ModifiedInput = make(map[string]any)
		}
		maps.Copy(accumulated.ModifiedInput, new.ModifiedInput)
	}

	if len(new.ModifiedOutput) > 0 {
		if accumulated.ModifiedOutput == nil {
			accumulated.ModifiedOutput = make(map[string]any)
		}
		maps.Copy(accumulated.ModifiedOutput, new.ModifiedOutput)
	}

	if new.ContextContent != "" {
		if accumulated.ContextContent == "" {
			accumulated.ContextContent = new.ContextContent
		} else {
			accumulated.ContextContent += "\n\n" + new.ContextContent
		}
	}

	accumulated.ContextFiles = append(accumulated.ContextFiles, new.ContextFiles...)

	if new.Message != "" {
		if accumulated.Message == "" {
			accumulated.Message = new.Message
		} else {
			accumulated.Message += "; " + new.Message
		}
	}
}

// ListHooks implements Manager.
func (m *manager) ListHooks(hookType HookType) []string {
	return m.discoverHooks(hookType)
}

// ExecuteUserPromptSubmit executes user-prompt-submit hooks.
func (m *manager) ExecuteUserPromptSubmit(ctx context.Context, sessionID, workingDir string, data UserPromptSubmitData) (HookResult, error) {
	hookCtx := HookContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Data:       data,
	}

	return m.executeHooks(ctx, HookUserPromptSubmit, hookCtx)
}

// ExecutePreToolUse executes pre-tool-use hooks.
func (m *manager) ExecutePreToolUse(ctx context.Context, sessionID, workingDir string, data PreToolUseData) (HookResult, error) {
	hookCtx := HookContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		ToolName:   data.ToolName,
		ToolCallID: data.ToolCallID,
		Data:       data,
	}

	return m.executeHooks(ctx, HookPreToolUse, hookCtx)
}

// ExecutePostToolUse executes post-tool-use hooks.
func (m *manager) ExecutePostToolUse(ctx context.Context, sessionID, workingDir string, data PostToolUseData) (HookResult, error) {
	hookCtx := HookContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		ToolName:   data.ToolName,
		ToolCallID: data.ToolCallID,
		Data:       data,
	}

	return m.executeHooks(ctx, HookPostToolUse, hookCtx)
}

// ExecuteStop executes stop hooks.
func (m *manager) ExecuteStop(ctx context.Context, sessionID, workingDir string, data StopData) (HookResult, error) {
	hookCtx := HookContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Data:       data,
	}

	return m.executeHooks(ctx, HookStop, hookCtx)
}

// ExecuteSessionStart executes session-start hooks.
func (m *manager) ExecuteSessionStart(ctx context.Context, sessionID, workingDir string, data SessionStartData) (HookResult, error) {
	hookCtx := HookContext{
		SessionID:  sessionID,
		WorkingDir: workingDir,
		Data:       data,
	}

	return m.executeHooks(ctx, HookSessionStart, hookCtx)
}
