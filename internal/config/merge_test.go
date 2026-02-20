package config

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/charmbracelet/crush/internal/csync"
	"github.com/stretchr/testify/require"
)

// TestConfigMerging defines the rules on how configuration merging works.
// Generally, things are either appended to or replaced by the later configuration.
// Whether one or the other happen depends on effects its effects.
func TestConfigMerging(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		c := exerciseMerge(t, Config{}, Config{})
		require.NotNil(t, c)
	})

	t.Run("mcps", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"foo": {
					Command: "foo-mcp",
					Args:    []string{"serve"},
					Type:    MCPSSE,
					Timeout: 10,
				},
				"zaz": {
					Disabled: true,
					Env:      map[string]string{"FOO": "bar"},
					Headers:  map[string]string{"api-key": "exposed"},
					URL:      "nope",
				},
			},
		}, Config{
			MCP: MCPs{
				"foo": {
					Args:    []string{"serve", "--stdio"},
					Type:    MCPStdio,
					Timeout: 7,
				},
				"bar": {
					Command: "bar",
				},
				"zaz": {
					Env:     map[string]string{"FOO": "foo", "BAR": "bar"},
					Headers: map[string]string{"api-key": "$API"},
					URL:     "http://bar",
				},
			},
		})
		require.NotNil(t, c)
		require.Len(t, slices.Collect(maps.Keys(c.MCP)), 3)

		// foo: merged from both configs
		foo := c.MCP["foo"]
		require.Equal(t, "foo-mcp", foo.Command)
		require.Equal(t, []string{"serve", "--stdio"}, foo.Args)
		require.Equal(t, MCPStdio, foo.Type)
		require.Equal(t, 10, foo.Timeout) // max of 10 and 7

		// bar: only in second config
		require.Equal(t, "bar", c.MCP["bar"].Command)

		// zaz: merged, env/headers merged, disabled stays true
		zaz := c.MCP["zaz"]
		require.True(t, zaz.Disabled)
		require.Equal(t, "http://bar", zaz.URL)
		require.Equal(t, "foo", zaz.Env["FOO"]) // overwritten
		require.Equal(t, "bar", zaz.Env["BAR"]) // added
		require.Equal(t, "$API", zaz.Headers["api-key"])
	})

	t.Run("lsps", func(t *testing.T) {
		result := exerciseMerge(t, Config{
			LSP: LSPs{
				"gopls": LSPConfig{
					Env:         map[string]string{"FOO": "bar"},
					RootMarkers: []string{"go.sum"},
					FileTypes:   []string{"go"},
				},
			},
		}, Config{
			LSP: LSPs{
				"gopls": LSPConfig{
					Command:     "gopls",
					InitOptions: map[string]any{"a": 10},
					RootMarkers: []string{"go.sum"},
				},
			},
		}, Config{
			LSP: LSPs{
				"gopls": LSPConfig{
					Args:        []string{"serve", "--stdio"},
					InitOptions: map[string]any{"a": 12, "b": 18},
					RootMarkers: []string{"go.sum", "go.mod"},
					FileTypes:   []string{"go"},
					Disabled:    true,
				},
			},
		},
			Config{
				LSP: LSPs{
					"gopls": LSPConfig{
						Options:     map[string]any{"opt1": "10"},
						RootMarkers: []string{"go.work"},
					},
				},
			},
		)
		require.NotNil(t, result)
		require.Equal(t, LSPConfig{
			Disabled:    true,
			Command:     "gopls",
			Args:        []string{"serve", "--stdio"},
			Env:         map[string]string{"FOO": "bar"},
			FileTypes:   []string{"go"},
			RootMarkers: []string{"go.mod", "go.sum", "go.work"},
			InitOptions: map[string]any{"a": 12.0, "b": 18.0},
			Options:     map[string]any{"opt1": "10"},
		}, result.LSP["gopls"])
	})

	t.Run("tui_options", func(t *testing.T) {
		maxDepth := 5
		maxItems := 100
		newMaxDepth := 10
		newMaxItems := 200

		c := exerciseMerge(t, Config{
			Options: &Options{
				TUI: &TUIOptions{
					CompactMode: false,
					DiffMode:    "unified",
					Completions: Completions{
						MaxDepth: &maxDepth,
						MaxItems: &maxItems,
					},
				},
			},
		}, Config{
			Options: &Options{
				TUI: &TUIOptions{
					CompactMode: true,
					DiffMode:    "split",
					Completions: Completions{
						MaxDepth: &newMaxDepth,
						MaxItems: &newMaxItems,
					},
				},
			},
		})

		require.NotNil(t, c)
		require.True(t, c.Options.TUI.CompactMode)
		require.Equal(t, "split", c.Options.TUI.DiffMode)
		require.Equal(t, newMaxDepth, *c.Options.TUI.Completions.MaxDepth)
	})

	t.Run("options", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				ContextPaths:              []string{"CRUSH.md"},
				Debug:                     false,
				DebugLSP:                  false,
				DisableProviderAutoUpdate: false,
				DisableMetrics:            false,
				DataDirectory:             ".crush",
				DisabledTools:             []string{"bash"},
				Attribution: &Attribution{
					TrailerStyle:  TrailerStyleNone,
					GeneratedWith: false,
				},
				TUI: &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				ContextPaths:              []string{".cursorrules"},
				Debug:                     true,
				DebugLSP:                  true,
				DisableProviderAutoUpdate: true,
				DisableMetrics:            true,
				DataDirectory:             ".custom",
				DisabledTools:             []string{"edit"},
				Attribution: &Attribution{
					TrailerStyle:  TrailerStyleCoAuthoredBy,
					GeneratedWith: true,
				},
				TUI: &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"CRUSH.md", ".cursorrules"}, c.Options.ContextPaths)
		require.True(t, c.Options.Debug)
		require.True(t, c.Options.DebugLSP)
		require.True(t, c.Options.DisableProviderAutoUpdate)
		require.True(t, c.Options.DisableMetrics)
		require.Equal(t, ".custom", c.Options.DataDirectory)
		require.Equal(t, []string{"bash", "edit"}, c.Options.DisabledTools)
		require.Equal(t, TrailerStyleCoAuthoredBy, c.Options.Attribution.TrailerStyle)
		require.True(t, c.Options.Attribution.GeneratedWith)
	})

	t.Run("tools", func(t *testing.T) {
		maxDepth := 5
		maxItems := 100
		newMaxDepth := 10
		newMaxItems := 200

		c := exerciseMerge(t, Config{
			Tools: Tools{
				Ls: ToolLs{
					MaxDepth: &maxDepth,
					MaxItems: &maxItems,
				},
			},
		}, Config{
			Tools: Tools{
				Ls: ToolLs{
					MaxDepth: &newMaxDepth,
					MaxItems: &newMaxItems,
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, newMaxDepth, *c.Tools.Ls.MaxDepth)
	})

	t.Run("models", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Models: map[SelectedModelType]SelectedModel{
				"large": {
					Model:    "gpt-4",
					Provider: "openai",
				},
			},
		}, Config{
			Models: map[SelectedModelType]SelectedModel{
				"large": {
					Model:    "gpt-4o",
					Provider: "openai",
				},
				"small": {
					Model:    "gpt-3.5-turbo",
					Provider: "openai",
				},
			},
		})

		require.NotNil(t, c)
		require.Len(t, c.Models, 2)
		require.Equal(t, "gpt-4o", c.Models["large"].Model)
		require.Equal(t, "gpt-3.5-turbo", c.Models["small"].Model)
	})

	t.Run("schema", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Schema: "https://example.com/schema.json",
		}, Config{
			Schema: "https://example.com/new-schema.json",
		})

		require.NotNil(t, c)
		require.Equal(t, "https://example.com/schema.json", c.Schema)
	})

	t.Run("schema_empty_first", func(t *testing.T) {
		c := exerciseMerge(t, Config{}, Config{
			Schema: "https://example.com/schema.json",
		})

		require.NotNil(t, c)
		require.Equal(t, "https://example.com/schema.json", c.Schema)
	})

	t.Run("permissions", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Permissions: &Permissions{
				AllowedTools: []string{"bash", "view"},
			},
		}, Config{
			Permissions: &Permissions{
				AllowedTools: []string{"edit", "write"},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"bash", "view", "edit", "write"}, c.Permissions.AllowedTools)
	})

	t.Run("mcp_timeout_max", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"test": {
					Timeout: 10,
				},
			},
		}, Config{
			MCP: MCPs{
				"test": {
					Timeout: 5,
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, 10, c.MCP["test"].Timeout)
	})

	t.Run("mcp_disabled_true_if_any", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"test": {
					Disabled: false,
				},
			},
		}, Config{
			MCP: MCPs{
				"test": {
					Disabled: true,
				},
			},
		})

		require.NotNil(t, c)
		require.True(t, c.MCP["test"].Disabled)
	})

	t.Run("lsp_disabled_true_if_any", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			LSP: LSPs{
				"test": {
					Disabled: false,
				},
			},
		}, Config{
			LSP: LSPs{
				"test": {
					Disabled: true,
				},
			},
		})

		require.NotNil(t, c)
		require.True(t, c.LSP["test"].Disabled)
	})

	t.Run("lsp_args_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			LSP: LSPs{
				"test": {
					Args: []string{"old", "args"},
				},
			},
		}, Config{
			LSP: LSPs{
				"test": {
					Args: []string{"new", "args"},
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"new", "args"}, c.LSP["test"].Args)
	})

	t.Run("lsp_filetypes_merged_and_deduplicated", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			LSP: LSPs{
				"test": {
					FileTypes: []string{"go", "mod"},
				},
			},
		}, Config{
			LSP: LSPs{
				"test": {
					FileTypes: []string{"go", "sum"},
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"go", "mod", "sum"}, c.LSP["test"].FileTypes)
	})

	t.Run("lsp_rootmarkers_merged_and_deduplicated", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			LSP: LSPs{
				"test": {
					RootMarkers: []string{"go.mod", "go.sum"},
				},
			},
		}, Config{
			LSP: LSPs{
				"test": {
					RootMarkers: []string{"go.sum", "go.work"},
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"go.mod", "go.sum", "go.work"}, c.LSP["test"].RootMarkers)
	})

	t.Run("options_attribution_nil", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				Attribution: &Attribution{
					TrailerStyle:  TrailerStyleCoAuthoredBy,
					GeneratedWith: true,
				},
				TUI: &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				TUI: &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, TrailerStyleCoAuthoredBy, c.Options.Attribution.TrailerStyle)
		require.True(t, c.Options.Attribution.GeneratedWith)
	})

	t.Run("tui_compact_mode_true_if_any", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				TUI: &TUIOptions{
					CompactMode: false,
				},
			},
		}, Config{
			Options: &Options{
				TUI: &TUIOptions{
					CompactMode: true,
				},
			},
		})

		require.NotNil(t, c)
		require.True(t, c.Options.TUI.CompactMode)
	})

	t.Run("tui_diff_mode_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				TUI: &TUIOptions{
					DiffMode: "unified",
				},
			},
		}, Config{
			Options: &Options{
				TUI: &TUIOptions{
					DiffMode: "split",
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, "split", c.Options.TUI.DiffMode)
	})

	t.Run("options_data_directory_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				DataDirectory: ".crush",
				TUI:           &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				DataDirectory: ".custom",
				TUI:           &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, ".custom", c.Options.DataDirectory)
	})

	t.Run("mcp_args_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"test": {
					Args: []string{"old"},
				},
			},
		}, Config{
			MCP: MCPs{
				"test": {
					Args: []string{"new"},
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"new"}, c.MCP["test"].Args)
	})

	t.Run("mcp_command_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"test": {
					Command: "old-command",
				},
			},
		}, Config{
			MCP: MCPs{
				"test": {
					Command: "new-command",
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, "new-command", c.MCP["test"].Command)
	})

	t.Run("mcp_type_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"test": {
					Type: MCPSSE,
				},
			},
		}, Config{
			MCP: MCPs{
				"test": {
					Type: MCPStdio,
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, MCPStdio, c.MCP["test"].Type)
	})

	t.Run("mcp_url_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"test": {
					URL: "http://old",
				},
			},
		}, Config{
			MCP: MCPs{
				"test": {
					URL: "http://new",
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, "http://new", c.MCP["test"].URL)
	})

	t.Run("lsp_command_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			LSP: LSPs{
				"test": {
					Command: "old-command",
				},
			},
		}, Config{
			LSP: LSPs{
				"test": {
					Command: "new-command",
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, "new-command", c.LSP["test"].Command)
	})

	t.Run("lsp_timeout_max", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			LSP: LSPs{
				"test": {
					Timeout: 60,
				},
			},
		}, Config{
			LSP: LSPs{
				"test": {
					Timeout: 30,
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, 60, c.LSP["test"].Timeout)
	})

	t.Run("mcp_disabled_tools_appended", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			MCP: MCPs{
				"test": {
					DisabledTools: []string{"tool1"},
				},
			},
		}, Config{
			MCP: MCPs{
				"test": {
					DisabledTools: []string{"tool2"},
				},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"tool1", "tool2"}, c.MCP["test"].DisabledTools)
	})

	t.Run("options_skills_paths_appended", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				SkillsPaths: []string{"/path/1"},
				TUI:         &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				SkillsPaths: []string{"/path/2"},
				TUI:         &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, []string{"/path/1", "/path/2"}, c.Options.SkillsPaths)
	})

	t.Run("tui_transparent_replaced", func(t *testing.T) {
		trueVal := true
		falseVal := false
		c := exerciseMerge(t, Config{
			Options: &Options{
				TUI: &TUIOptions{
					Transparent: &falseVal,
				},
			},
		}, Config{
			Options: &Options{
				TUI: &TUIOptions{
					Transparent: &trueVal,
				},
			},
		})

		require.NotNil(t, c)
		require.True(t, *c.Options.TUI.Transparent)
	})

	t.Run("options_initialize_as_replaced", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				InitializeAs: "CRUSH.md",
				TUI:          &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				InitializeAs: "AGENTS.md",
				TUI:          &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.Equal(t, "AGENTS.md", c.Options.InitializeAs)
	})

	t.Run("options_auto_lsp_replaced", func(t *testing.T) {
		trueVal := true
		c := exerciseMerge(t, Config{
			Options: &Options{
				TUI: &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				AutoLSP: &trueVal,
				TUI:     &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.True(t, *c.Options.AutoLSP)
	})

	t.Run("options_disable_auto_summarize_true_if_any", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				DisableAutoSummarize: false,
				TUI:                  &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				DisableAutoSummarize: true,
				TUI:                  &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.True(t, c.Options.DisableAutoSummarize)
	})

	t.Run("options_disable_default_providers_true_if_any", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Options: &Options{
				DisableDefaultProviders: false,
				TUI:                     &TUIOptions{},
			},
		}, Config{
			Options: &Options{
				DisableDefaultProviders: true,
				TUI:                     &TUIOptions{},
			},
		})

		require.NotNil(t, c)
		require.True(t, c.Options.DisableDefaultProviders)
	})

	t.Run("provider_config_merge_preserves_fields", func(t *testing.T) {
		// Tests that merging a later provider config with empty fields
		// does not overwrite earlier non-empty fields.
		c := exerciseMerge(t, Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {
					APIKey:  "key1",
					BaseURL: "https://api.openai.com/v1",
				},
			}),
		}, Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {
					APIKey:  "key2",
					BaseURL: "https://api.openai.com/v2",
				},
			}),
		}, Config{
			// Later config with empty provider - should not clear fields.
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {},
			}),
		})

		require.NotNil(t, c)
		pc, ok := c.Providers.Get("openai")
		require.True(t, ok)
		require.Equal(t, "key2", pc.APIKey)
		require.Equal(t, "https://api.openai.com/v2", pc.BaseURL)
	})

	t.Run("provider_config_disable_true_if_any", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {
					APIKey:  "key1",
					Disable: false,
				},
			}),
		}, Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {
					Disable: true,
				},
			}),
		})

		require.NotNil(t, c)
		pc, ok := c.Providers.Get("openai")
		require.True(t, ok)
		require.True(t, pc.Disable)
		require.Equal(t, "key1", pc.APIKey)
	})

	t.Run("provider_config_extra_headers_merged", func(t *testing.T) {
		c := exerciseMerge(t, Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {
					ExtraHeaders: map[string]string{"X-First": "value1"},
				},
			}),
		}, Config{
			Providers: csync.NewMapFrom(map[string]ProviderConfig{
				"openai": {
					ExtraHeaders: map[string]string{"X-Second": "value2"},
				},
			}),
		})

		require.NotNil(t, c)
		pc, ok := c.Providers.Get("openai")
		require.True(t, ok)
		require.Equal(t, "value1", pc.ExtraHeaders["X-First"])
		require.Equal(t, "value2", pc.ExtraHeaders["X-Second"])
	})
}

func exerciseMerge(tb testing.TB, confs ...Config) *Config {
	tb.Helper()
	data := make([][]byte, 0, len(confs))
	for _, c := range confs {
		bts, err := json.Marshal(c)
		require.NoError(tb, err)
		data = append(data, bts)
	}
	result, err := loadFromBytes(data)
	require.NoError(tb, err)
	return result
}
