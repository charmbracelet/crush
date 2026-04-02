package plugin

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/shell"
)

type localToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  any            `json:"parameters"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Execute     string         `json:"execute"`
}

func DiscoverLocalTools(workingDir string) (map[string]ToolDefinition, error) {
	result := make(map[string]ToolDefinition)
	dirs := localToolDirs(workingDir)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			tool, err := loadLocalToolDefinition(path, workingDir)
			if err != nil {
				slog.Warn("Failed to load local tool definition", "path", path, "error", err)
				continue
			}
			result[tool.Name] = tool
		}
	}
	return result, nil
}

func localToolDirs(workingDir string) []string {
	globalDir := filepath.Join(filepath.Dir(config.GlobalConfig()), "tools")
	projectDir := filepath.Join(workingDir, ".crush", "tools")
	return []string{globalDir, projectDir}
}

func loadLocalToolDefinition(path string, workingDir string) (ToolDefinition, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ToolDefinition{}, err
	}

	var parsed localToolDefinition
	if err := json.Unmarshal(content, &parsed); err != nil {
		return ToolDefinition{}, err
	}
	if parsed.Name == "" {
		return ToolDefinition{}, fmt.Errorf("missing name")
	}
	if parsed.Description == "" {
		return ToolDefinition{}, fmt.Errorf("missing description")
	}
	if parsed.Execute == "" {
		return ToolDefinition{}, fmt.Errorf("missing execute")
	}

	parameters := map[string]any{}
	if parsedMap, ok := parsed.Parameters.(map[string]any); ok {
		parameters = parsedMap
	}

	return ToolDefinition{
		Name:        parsed.Name,
		Description: parsed.Description,
		Parameters:  parameters,
		Metadata:    parsed.Metadata,
		Execute:     localToolExecutor(parsed.Execute, workingDir),
	}, nil
}

func localToolExecutor(command string, defaultWorkingDir string) func(ctx context.Context, args map[string]any, execCtx ToolContext) (string, error) {
	return func(ctx context.Context, args map[string]any, execCtx ToolContext) (string, error) {
		workingDir := cmp.Or(execCtx.Directory, defaultWorkingDir)
		env := append(os.Environ(), argsToEnv(args)...)
		sh := shell.NewShell(&shell.Options{
			WorkingDir: workingDir,
			Env:        env,
		})
		stdout, stderr, err := sh.Exec(ctx, command)
		output := strings.TrimSpace(strings.TrimSpace(stdout) + "\n" + strings.TrimSpace(stderr))
		if err != nil {
			if output != "" {
				return output, err
			}
			return "", err
		}
		return strings.TrimSpace(output), nil
	}
}

func argsToEnv(args map[string]any) []string {
	if len(args) == 0 {
		return nil
	}

	env := make([]string, 0, len(args))
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, key := range keys {
		normalized := normalizeEnvKey(key)
		env = append(env, fmt.Sprintf("CRUSH_TOOL_ARG_%s=%v", normalized, args[key]))
	}
	return env
}

func normalizeEnvKey(key string) string {
	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	return strings.ToUpper(replacer.Replace(key))
}
