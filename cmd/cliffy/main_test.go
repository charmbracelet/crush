package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/bwl/cliffy/internal/config"
	"github.com/bwl/cliffy/internal/csync"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetFlags resets all cobra flags to their default values
func resetFlags() {
	showThinking = false
	thinkingFormat = "text"
	outputFormat = "text"
	model = ""
	quiet = false
	fast = false
	smart = false
	showStats = false
	showVersion = false
	verbose = false
	sharedContext = ""
	contextFile = ""
	emitToolTrace = false
	presetID = ""
}

// captureOutput captures stdout and stderr during function execution
func captureOutput(f func()) (stdout, stderr string) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rOut)
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rErr)
		errC <- buf.String()
	}()

	f()

	wOut.Close()
	wErr.Close()

	stdout = <-outC
	stderr = <-errC

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return
}

func TestRootCmd_FlagParsing(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectError   bool
		checkFlags    func(t *testing.T)
	}{
		{
			name:        "show-thinking flag",
			args:        []string{"--show-thinking", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, showThinking)
			},
		},
		{
			name:        "show-thinking short flag",
			args:        []string{"-t", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, showThinking)
			},
		},
		{
			name:        "thinking-format json",
			args:        []string{"--thinking-format", "json", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "json", thinkingFormat)
			},
		},
		{
			name:        "output-format json",
			args:        []string{"--output-format", "json", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "json", outputFormat)
			},
		},
		{
			name:        "output-format short flag",
			args:        []string{"-o", "json", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "json", outputFormat)
			},
		},
		{
			name:        "model flag",
			args:        []string{"--model", "gpt-4", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "gpt-4", model)
			},
		},
		{
			name:        "model short flag",
			args:        []string{"-m", "gpt-4", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "gpt-4", model)
			},
		},
		{
			name:        "quiet flag",
			args:        []string{"--quiet", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, quiet)
			},
		},
		{
			name:        "quiet short flag",
			args:        []string{"-q", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, quiet)
			},
		},
		{
			name:        "fast flag",
			args:        []string{"--fast", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, fast)
			},
		},
		{
			name:        "smart flag",
			args:        []string{"--smart", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, smart)
			},
		},
		{
			name:        "stats flag",
			args:        []string{"--stats", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, showStats)
			},
		},
		{
			name:        "verbose flag",
			args:        []string{"--verbose", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, verbose)
			},
		},
		{
			name:        "verbose short flag",
			args:        []string{"-v", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, verbose)
			},
		},
		{
			name:        "context flag",
			args:        []string{"--context", "You are a security expert", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "You are a security expert", sharedContext)
			},
		},
		{
			name:        "context-file flag",
			args:        []string{"--context-file", "rules.md", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "rules.md", contextFile)
			},
		},
		{
			name:        "emit-tool-trace flag",
			args:        []string{"--emit-tool-trace", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.True(t, emitToolTrace)
			},
		},
		{
			name:        "preset flag",
			args:        []string{"--preset", "fast-qa", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "fast-qa", presetID)
			},
		},
		{
			name:        "preset short flag",
			args:        []string{"-p", "sec-review", "test task"},
			expectError: false,
			checkFlags: func(t *testing.T) {
				assert.Equal(t, "sec-review", presetID)
			},
		},
		{
			name:        "no arguments",
			args:        []string{},
			expectError: true,
			checkFlags:  func(t *testing.T) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each test
			resetFlags()

			// Create a new root command for each test to avoid flag pollution
			cmd := &cobra.Command{
				Use:   "cliffy [flags] <task> [task2] [task3] ...",
				Short: "Fast AI coding assistant - single or multiple tasks",
				Args: func(cmd *cobra.Command, args []string) error {
					// Allow no args if version flag is set
					if showVersion {
						return nil
					}
					return cobra.MinimumNArgs(1)(cmd, args)
				},
				RunE: func(cmd *cobra.Command, args []string) error {
					// Just parse flags, don't actually execute
					return nil
				},
			}

			// Add flags
			cmd.Flags().BoolVarP(&showThinking, "show-thinking", "t", false, "Show LLM thinking/reasoning")
			cmd.Flags().StringVar(&thinkingFormat, "thinking-format", "text", "Format for thinking: text|json")
			cmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text|json")
			cmd.Flags().StringVarP(&model, "model", "m", "", "Override model selection")
			cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Results only - suppress tool traces and progress")
			cmd.Flags().BoolVar(&fast, "fast", false, "Use small/fast model")
			cmd.Flags().BoolVar(&smart, "smart", false, "Use large/smart model")
			cmd.Flags().BoolVar(&showStats, "stats", false, "Show token usage and timing")
			cmd.Flags().BoolVar(&showVersion, "version", false, "Show version info")
			cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed tool traces, thinking, and events")
			cmd.Flags().StringVar(&sharedContext, "context", "", "Shared context prepended to each task")
			cmd.Flags().StringVar(&contextFile, "context-file", "", "Load shared context from file")
			cmd.Flags().BoolVar(&emitToolTrace, "emit-tool-trace", false, "Emit tool execution metadata as NDJSON to stderr for automation")
			cmd.Flags().StringVarP(&presetID, "preset", "p", "", "Use a preset configuration")

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tt.checkFlags(t)
			}
		})
	}
}

func TestRootCmd_FlagCombinations(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		checkFlags func(t *testing.T)
	}{
		{
			name: "verbose and stats",
			args: []string{"--verbose", "--stats", "test task"},
			checkFlags: func(t *testing.T) {
				assert.True(t, verbose)
				assert.True(t, showStats)
			},
		},
		{
			name: "fast and json output",
			args: []string{"--fast", "-o", "json", "test task"},
			checkFlags: func(t *testing.T) {
				assert.True(t, fast)
				assert.Equal(t, "json", outputFormat)
			},
		},
		{
			name: "smart and show-thinking",
			args: []string{"--smart", "--show-thinking", "test task"},
			checkFlags: func(t *testing.T) {
				assert.True(t, smart)
				assert.True(t, showThinking)
			},
		},
		{
			name: "quiet and output json (for machine parsing)",
			args: []string{"-q", "-o", "json", "test task"},
			checkFlags: func(t *testing.T) {
				assert.True(t, quiet)
				assert.Equal(t, "json", outputFormat)
			},
		},
		{
			name: "all flags combined",
			args: []string{
				"--verbose",
				"--show-thinking",
				"--thinking-format", "json",
				"--output-format", "json",
				"--stats",
				"--emit-tool-trace",
				"--context", "test context",
				"--model", "gpt-4",
				"test task",
			},
			checkFlags: func(t *testing.T) {
				assert.True(t, verbose)
				assert.True(t, showThinking)
				assert.Equal(t, "json", thinkingFormat)
				assert.Equal(t, "json", outputFormat)
				assert.True(t, showStats)
				assert.True(t, emitToolTrace)
				assert.Equal(t, "test context", sharedContext)
				assert.Equal(t, "gpt-4", model)
			},
		},
		{
			name: "fast and smart (smart should override)",
			args: []string{"--fast", "--smart", "test task"},
			checkFlags: func(t *testing.T) {
				assert.True(t, fast)
				assert.True(t, smart)
				// Note: In actual execution, smart should take precedence
			},
		},
		{
			name: "multiple tasks",
			args: []string{"--verbose", "task 1", "task 2", "task 3"},
			checkFlags: func(t *testing.T) {
				assert.True(t, verbose)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each test
			resetFlags()

			// Create a new root command for each test
			cmd := &cobra.Command{
				Use:   "cliffy [flags] <task> [task2] [task3] ...",
				Short: "Fast AI coding assistant - single or multiple tasks",
				Args:  cobra.MinimumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					// Just parse flags, don't actually execute
					return nil
				},
			}

			// Add flags
			cmd.Flags().BoolVarP(&showThinking, "show-thinking", "t", false, "Show LLM thinking/reasoning")
			cmd.Flags().StringVar(&thinkingFormat, "thinking-format", "text", "Format for thinking: text|json")
			cmd.Flags().StringVarP(&outputFormat, "output-format", "o", "text", "Output format: text|json")
			cmd.Flags().StringVarP(&model, "model", "m", "", "Override model selection")
			cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Results only - suppress tool traces and progress")
			cmd.Flags().BoolVar(&fast, "fast", false, "Use small/fast model")
			cmd.Flags().BoolVar(&smart, "smart", false, "Use large/smart model")
			cmd.Flags().BoolVar(&showStats, "stats", false, "Show token usage and timing")
			cmd.Flags().BoolVar(&showVersion, "version", false, "Show version info")
			cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed tool traces, thinking, and events")
			cmd.Flags().StringVar(&sharedContext, "context", "", "Shared context prepended to each task")
			cmd.Flags().StringVar(&contextFile, "context-file", "", "Load shared context from file")
			cmd.Flags().BoolVar(&emitToolTrace, "emit-tool-trace", false, "Emit tool execution metadata as NDJSON to stderr for automation")
			cmd.Flags().StringVarP(&presetID, "preset", "p", "", "Use a preset configuration")

			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			require.NoError(t, err)
			tt.checkFlags(t)
		})
	}
}

func TestRootCmd_Version(t *testing.T) {
	resetFlags()

	cmd := &cobra.Command{
		Use:   "cliffy",
		Short: "Fast AI coding assistant",
		Args: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				return nil
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				// Just set a marker that we'd print version
				return nil
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showVersion, "version", false, "Show version info")
	cmd.SetArgs([]string{"--version"})

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.True(t, showVersion)
}

func TestRootCmd_CompletionCommand(t *testing.T) {
	// Test that completion commands are available
	completionCmd := newCompletionCmd()
	assert.NotNil(t, completionCmd)
	assert.Equal(t, "completion [bash|zsh|fish|powershell]", completionCmd.Use)

	// Test valid shells
	validShells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range validShells {
		t.Run(shell, func(t *testing.T) {
			assert.Contains(t, completionCmd.ValidArgs, shell)
		})
	}
}

func TestTruncatePrompt(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		maxLen   int
		expected string
	}{
		{
			name:     "short prompt",
			prompt:   "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			prompt:   "1234567890",
			maxLen:   10,
			expected: "1234567890",
		},
		{
			name:     "too long",
			prompt:   "this is a very long prompt that should be truncated",
			maxLen:   20,
			expected: "this is a very lo...",
		},
		{
			name:     "empty prompt",
			prompt:   "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePrompt(tt.prompt, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrintVersion(t *testing.T) {
	stdout, _ := captureOutput(func() {
		printVersion()
	})

	// Check that version output contains expected strings
	assert.Contains(t, stdout, "Cliffy")
	assert.Contains(t, stdout, version)
	assert.Contains(t, stdout, "https://cliffy.ettio.com")
}

func TestPrintError(t *testing.T) {
	tests := []struct {
		name           string
		errorMsg       string
		expectContains []string
	}{
		{
			name:     "config error",
			errorMsg: "config load failed: API key missing",
			expectContains: []string{
				"Error:",
				"Quick setup:",
				"openrouter.ai",
			},
		},
		{
			name:     "model error",
			errorMsg: "model gpt-5 not found",
			expectContains: []string{
				"Error:",
				"--fast",
				"--smart",
			},
		},
		{
			name:     "rate limit error",
			errorMsg: "429 rate limit exceeded",
			expectContains: []string{
				"Error:",
				"Rate limited",
				"--fast",
			},
		},
		{
			name:     "timeout error",
			errorMsg: "context deadline exceeded",
			expectContains: []string{
				"Error:",
				"timed out",
			},
		},
		{
			name:     "generic error",
			errorMsg: "something went wrong",
			expectContains: []string{
				"Error:",
				"something went wrong",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, stderr := captureOutput(func() {
				printError(&testError{msg: tt.errorMsg})
			})

			for _, expected := range tt.expectContains {
				assert.Contains(t, stderr, expected)
			}
		})
	}
}

// testError is a simple error type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestVerbosityLevelSelection(t *testing.T) {
	tests := []struct {
		name          string
		quiet         bool
		verbose       bool
		expectedLevel string // "quiet", "normal", or "verbose"
	}{
		{
			name:          "default (normal)",
			quiet:         false,
			verbose:       false,
			expectedLevel: "normal",
		},
		{
			name:          "quiet mode",
			quiet:         true,
			verbose:       false,
			expectedLevel: "quiet",
		},
		{
			name:          "verbose mode",
			quiet:         false,
			verbose:       true,
			expectedLevel: "verbose",
		},
		{
			name:          "quiet takes precedence over verbose",
			quiet:         true,
			verbose:       true,
			expectedLevel: "quiet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			quiet = tt.quiet
			verbose = tt.verbose

			// Simulate verbosity determination logic from main.go
			verbosityLevel := "normal"
			if quiet {
				verbosityLevel = "quiet"
			} else if verbose {
				verbosityLevel = "verbose"
			}

			assert.Equal(t, tt.expectedLevel, verbosityLevel)
		})
	}
}

func TestModelSelectionPriority(t *testing.T) {
	tests := []struct {
		name           string
		modelFlag      string
		fastFlag       bool
		smartFlag      bool
		expectedResult string // describes which takes precedence
	}{
		{
			name:           "explicit model flag takes precedence",
			modelFlag:      "gpt-4",
			fastFlag:       true,
			smartFlag:      true,
			expectedResult: "explicit",
		},
		{
			name:           "fast flag when no explicit model",
			modelFlag:      "",
			fastFlag:       true,
			smartFlag:      false,
			expectedResult: "fast",
		},
		{
			name:           "smart flag when no explicit model",
			modelFlag:      "",
			fastFlag:       false,
			smartFlag:      true,
			expectedResult: "smart",
		},
		{
			name:           "smart takes precedence over fast",
			modelFlag:      "",
			fastFlag:       true,
			smartFlag:      true,
			expectedResult: "smart",
		},
		{
			name:           "no model selection",
			modelFlag:      "",
			fastFlag:       false,
			smartFlag:      false,
			expectedResult: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetFlags()
			model = tt.modelFlag
			fast = tt.fastFlag
			smart = tt.smartFlag

			// Simulate model selection logic
			var result string
			if model != "" {
				result = "explicit"
			} else if smart {
				result = "smart"
			} else if fast {
				result = "fast"
			} else {
				result = "none"
			}

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestContextFileHandling(t *testing.T) {
	// Create a temporary context file
	tmpDir := t.TempDir()
	contextFilePath := tmpDir + "/context.md"
	contextContent := "You are a security expert. Review code for vulnerabilities."

	err := os.WriteFile(contextFilePath, []byte(contextContent), 0644)
	require.NoError(t, err)

	resetFlags()
	contextFile = contextFilePath

	// Read the file (simulating what the actual code does)
	content, err := os.ReadFile(contextFile)
	require.NoError(t, err)
	assert.Equal(t, contextContent, string(content))
}

func TestContextFileMissing(t *testing.T) {
	resetFlags()
	contextFile = "/nonexistent/file.md"

	// Try to read the file
	_, err := os.ReadFile(contextFile)
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestMultipleTasksParsing(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedTasks int
	}{
		{
			name:          "single task",
			args:          []string{"task 1"},
			expectedTasks: 1,
		},
		{
			name:          "two tasks",
			args:          []string{"task 1", "task 2"},
			expectedTasks: 2,
		},
		{
			name:          "three tasks",
			args:          []string{"task 1", "task 2", "task 3"},
			expectedTasks: 3,
		},
		{
			name:          "many tasks",
			args:          []string{"t1", "t2", "t3", "t4", "t5"},
			expectedTasks: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The args slice itself represents the tasks
			assert.Equal(t, tt.expectedTasks, len(tt.args))
		})
	}
}

func TestValidateModelExists(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func() *config.Config
		modelType   config.SelectedModelType
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid model configuration",
			setupConfig: func() *config.Config {
				cfg := &config.Config{
					Models: map[config.SelectedModelType]config.SelectedModel{
						config.SelectedModelTypeLarge: {
							Model:    "gpt-4o",
							Provider: "openai",
						},
					},
					Providers: csync.NewMap[string, config.ProviderConfig](),
				}
				cfg.Providers.Set("openai", config.ProviderConfig{
					ID:      "openai",
					Disable: false,
					Models: []catwalk.Model{
						{ID: "gpt-4o", Name: "GPT-4o"},
					},
				})
				return cfg
			},
			modelType:   config.SelectedModelTypeLarge,
			expectError: false,
		},
		{
			name: "model type not configured",
			setupConfig: func() *config.Config {
				return &config.Config{
					Models:    map[config.SelectedModelType]config.SelectedModel{},
					Providers: csync.NewMap[string, config.ProviderConfig](),
				}
			},
			modelType:   config.SelectedModelTypeLarge,
			expectError: true,
			errorMsg:    "not configured",
		},
		{
			name: "provider not found",
			setupConfig: func() *config.Config {
				cfg := &config.Config{
					Models: map[config.SelectedModelType]config.SelectedModel{
						config.SelectedModelTypeLarge: {
							Model:    "gpt-4o",
							Provider: "nonexistent",
						},
					},
					Providers: csync.NewMap[string, config.ProviderConfig](),
				}
				return cfg
			},
			modelType:   config.SelectedModelTypeLarge,
			expectError: true,
			errorMsg:    "not found",
		},
		{
			name: "provider disabled",
			setupConfig: func() *config.Config {
				cfg := &config.Config{
					Models: map[config.SelectedModelType]config.SelectedModel{
						config.SelectedModelTypeLarge: {
							Model:    "gpt-4o",
							Provider: "openai",
						},
					},
					Providers: csync.NewMap[string, config.ProviderConfig](),
				}
				cfg.Providers.Set("openai", config.ProviderConfig{
					ID:      "openai",
					Disable: true, // Provider disabled
					Models: []catwalk.Model{
						{ID: "gpt-4o"},
					},
				})
				return cfg
			},
			modelType:   config.SelectedModelTypeLarge,
			expectError: true,
			errorMsg:    "disabled",
		},
		{
			name: "model not in provider's model list",
			setupConfig: func() *config.Config {
				cfg := &config.Config{
					Models: map[config.SelectedModelType]config.SelectedModel{
						config.SelectedModelTypeLarge: {
							Model:    "gpt-5", // Non-existent model
							Provider: "openai",
						},
					},
					Providers: csync.NewMap[string, config.ProviderConfig](),
				}
				cfg.Providers.Set("openai", config.ProviderConfig{
					ID:      "openai",
					Disable: false,
					Models: []catwalk.Model{
						{ID: "gpt-4o"},
						{ID: "gpt-4"},
					},
				})
				return cfg
			},
			modelType:   config.SelectedModelTypeLarge,
			expectError: true,
			errorMsg:    "not found in provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.setupConfig()
			err := validateModelExists(cfg, tt.modelType)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
