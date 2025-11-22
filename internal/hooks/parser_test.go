package hooks

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseShellEnv(t *testing.T) {
	t.Run("parses basic fields", func(t *testing.T) {
		env := []string{
			"PATH=/usr/bin",
			"CRUSH_CONTINUE=false",
			"CRUSH_PERMISSION=approve",
			"CRUSH_MESSAGE=test message",
			"HOME=/home/user",
		}

		result := parseShellEnv(env)

		assert.False(t, result.Continue)
		assert.Equal(t, "approve", result.Permission)
		assert.Equal(t, "test message", result.Message)
	})

	t.Run("parses modified prompt", func(t *testing.T) {
		env := []string{
			"CRUSH_MODIFIED_PROMPT=new prompt text",
		}

		result := parseShellEnv(env)

		require.NotNil(t, result.ModifiedPrompt)
		assert.Equal(t, "new prompt text", *result.ModifiedPrompt)
	})

	t.Run("parses context content", func(t *testing.T) {
		env := []string{
			"CRUSH_CONTEXT_CONTENT=some context",
		}

		result := parseShellEnv(env)

		assert.Equal(t, "some context", result.ContextContent)
	})

	t.Run("parses base64 context content", func(t *testing.T) {
		text := "multiline\ncontext\nhere"
		encoded := base64.StdEncoding.EncodeToString([]byte(text))

		env := []string{
			"CRUSH_CONTEXT_CONTENT=" + encoded,
		}

		result := parseShellEnv(env)

		assert.Equal(t, text, result.ContextContent)
	})

	t.Run("parses context files", func(t *testing.T) {
		env := []string{
			"CRUSH_CONTEXT_FILES=file1.md:file2.txt:file3.go",
		}

		result := parseShellEnv(env)

		assert.Equal(t, []string{"file1.md", "file2.txt", "file3.go"}, result.ContextFiles)
	})

	t.Run("defaults to continue=true", func(t *testing.T) {
		env := []string{}

		result := parseShellEnv(env)

		assert.True(t, result.Continue)
	})

	t.Run("ignores non-CRUSH env vars", func(t *testing.T) {
		env := []string{
			"PATH=/usr/bin",
			"HOME=/home/user",
			"CRUSH_MESSAGE=test",
		}

		result := parseShellEnv(env)

		assert.Equal(t, "test", result.Message)
	})

	t.Run("falls back to raw value for invalid base64", func(t *testing.T) {
		// Invalid base64 string should be used as-is.
		env := []string{
			"CRUSH_CONTEXT_CONTENT=this is not base64!@#$",
		}

		result := parseShellEnv(env)

		assert.Equal(t, "this is not base64!@#$", result.ContextContent)
	})

	t.Run("parses modified input", func(t *testing.T) {
		env := []string{
			"CRUSH_MODIFIED_INPUT=command=ls -la:working_dir=/tmp",
		}

		result := parseShellEnv(env)

		require.NotNil(t, result.ModifiedInput)
		assert.Equal(t, "ls -la", result.ModifiedInput["command"])
		assert.Equal(t, "/tmp", result.ModifiedInput["working_dir"])
	})

	t.Run("parses modified output", func(t *testing.T) {
		env := []string{
			"CRUSH_MODIFIED_OUTPUT=status=redacted:data=[REDACTED]",
		}

		result := parseShellEnv(env)

		require.NotNil(t, result.ModifiedOutput)
		assert.Equal(t, "redacted", result.ModifiedOutput["status"])
		assert.Equal(t, "[REDACTED]", result.ModifiedOutput["data"])
	})

	t.Run("parses modified input with JSON types", func(t *testing.T) {
		env := []string{
			`CRUSH_MODIFIED_INPUT=offset=100:limit=50:run_in_background=true:ignore=["*.log","*.tmp"]`,
		}

		result := parseShellEnv(env)

		require.NotNil(t, result.ModifiedInput)
		assert.Equal(t, float64(100), result.ModifiedInput["offset"]) // JSON numbers are float64
		assert.Equal(t, float64(50), result.ModifiedInput["limit"])
		assert.Equal(t, true, result.ModifiedInput["run_in_background"])
		assert.Equal(t, []any{"*.log", "*.tmp"}, result.ModifiedInput["ignore"])
	})

	t.Run("parses modified input with strings containing colons", func(t *testing.T) {
		// Colons in file paths should work if the value doesn't contain '='
		env := []string{
			`CRUSH_MODIFIED_INPUT=path=/usr/local/bin:name=test`,
		}

		result := parseShellEnv(env)

		require.NotNil(t, result.ModifiedInput)
		// First pair: path=/usr/local/bin
		// Second pair: name=test
		// Note: This splits on first '=' in each pair
		assert.Equal(t, "/usr/local/bin", result.ModifiedInput["path"])
		assert.Equal(t, "test", result.ModifiedInput["name"])
	})
}

func TestParseJSONResult(t *testing.T) {
	t.Run("parses basic fields", func(t *testing.T) {
		json := []byte(`{
			"continue": false,
			"permission": "deny",
			"message": "blocked"
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		assert.False(t, result.Continue)
		assert.Equal(t, "deny", result.Permission)
		assert.Equal(t, "blocked", result.Message)
	})

	t.Run("parses modified_input", func(t *testing.T) {
		json := []byte(`{
			"modified_input": {
				"command": "ls -la",
				"working_dir": "/tmp"
			}
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"command":     "ls -la",
			"working_dir": "/tmp",
		}, result.ModifiedInput)
	})

	t.Run("parses modified_output", func(t *testing.T) {
		json := []byte(`{
			"modified_output": {
				"content": "filtered output"
			}
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		assert.Equal(t, map[string]any{
			"content": "filtered output",
		}, result.ModifiedOutput)
	})

	t.Run("parses context_files array", func(t *testing.T) {
		json := []byte(`{
			"context_files": ["file1.md", "file2.txt"]
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		assert.Equal(t, []string{"file1.md", "file2.txt"}, result.ContextFiles)
	})

	t.Run("returns error on invalid JSON", func(t *testing.T) {
		json := []byte(`{invalid}`)

		_, err := parseJSONResult(json)

		assert.Error(t, err)
	})

	t.Run("defaults to continue=true", func(t *testing.T) {
		json := []byte(`{"message": "test"}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		assert.True(t, result.Continue)
	})

	t.Run("handles wrong type for modified_input", func(t *testing.T) {
		// modified_input should be a map, but here it's a string.
		json := []byte(`{
			"modified_input": "not a map"
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		// Should be nil/empty since type assertion failed.
		assert.Nil(t, result.ModifiedInput)
	})

	t.Run("handles wrong type for modified_output", func(t *testing.T) {
		// modified_output should be a map, but here it's an array.
		json := []byte(`{
			"modified_output": ["not", "a", "map"]
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		assert.Nil(t, result.ModifiedOutput)
	})

	t.Run("handles non-string elements in context_files", func(t *testing.T) {
		// context_files should be array of strings, but has numbers.
		json := []byte(`{
			"context_files": ["file1.md", 123, "file2.md", null]
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		// Should only include valid strings.
		assert.Equal(t, []string{"file1.md", "file2.md"}, result.ContextFiles)
	})

	t.Run("handles wrong type for context_files", func(t *testing.T) {
		// context_files should be an array, but here it's a string.
		json := []byte(`{
			"context_files": "not an array"
		}`)

		result, err := parseJSONResult(json)

		require.NoError(t, err)
		// Should be empty since type assertion failed.
		assert.Empty(t, result.ContextFiles)
	})
}

func TestMergeJSONResult(t *testing.T) {
	t.Run("merges continue flag", func(t *testing.T) {
		base := &HookResult{Continue: true}
		json := &HookResult{Continue: false}

		mergeJSONResult(base, json)

		assert.False(t, base.Continue)
	})

	t.Run("merges permission", func(t *testing.T) {
		base := &HookResult{}
		json := &HookResult{Permission: "approve"}

		mergeJSONResult(base, json)

		assert.Equal(t, "approve", base.Permission)
	})

	t.Run("appends messages", func(t *testing.T) {
		base := &HookResult{Message: "first"}
		json := &HookResult{Message: "second"}

		mergeJSONResult(base, json)

		assert.Equal(t, "first; second", base.Message)
	})

	t.Run("merges modified_input maps", func(t *testing.T) {
		base := &HookResult{
			ModifiedInput: map[string]any{
				"field1": "value1",
			},
		}
		json := &HookResult{
			ModifiedInput: map[string]any{
				"field2": "value2",
			},
		}

		mergeJSONResult(base, json)

		assert.Equal(t, map[string]any{
			"field1": "value1",
			"field2": "value2",
		}, base.ModifiedInput)
	})

	t.Run("overwrites conflicting modified_input fields", func(t *testing.T) {
		base := &HookResult{
			ModifiedInput: map[string]any{
				"field": "old",
			},
		}
		json := &HookResult{
			ModifiedInput: map[string]any{
				"field": "new",
			},
		}

		mergeJSONResult(base, json)

		assert.Equal(t, "new", base.ModifiedInput["field"])
	})

	t.Run("appends context content", func(t *testing.T) {
		base := &HookResult{ContextContent: "first"}
		json := &HookResult{ContextContent: "second"}

		mergeJSONResult(base, json)

		assert.Equal(t, "first\n\nsecond", base.ContextContent)
	})

	t.Run("appends context files", func(t *testing.T) {
		base := &HookResult{ContextFiles: []string{"file1.md"}}
		json := &HookResult{ContextFiles: []string{"file2.md", "file3.md"}}

		mergeJSONResult(base, json)

		assert.Equal(t, []string{"file1.md", "file2.md", "file3.md"}, base.ContextFiles)
	})

	t.Run("initializes ModifiedInput when nil", func(t *testing.T) {
		// Base has nil ModifiedInput.
		base := &HookResult{}
		json := &HookResult{
			ModifiedInput: map[string]any{
				"field": "value",
			},
		}

		mergeJSONResult(base, json)

		require.NotNil(t, base.ModifiedInput)
		assert.Equal(t, "value", base.ModifiedInput["field"])
	})

	t.Run("initializes ModifiedOutput when nil", func(t *testing.T) {
		// Base has nil ModifiedOutput.
		base := &HookResult{}
		json := &HookResult{
			ModifiedOutput: map[string]any{
				"filtered": true,
			},
		}

		mergeJSONResult(base, json)

		require.NotNil(t, base.ModifiedOutput)
		assert.Equal(t, true, base.ModifiedOutput["filtered"])
	})

	t.Run("sets context content when base is empty", func(t *testing.T) {
		base := &HookResult{}
		json := &HookResult{ContextContent: "new content"}

		mergeJSONResult(base, json)

		assert.Equal(t, "new content", base.ContextContent)
	})

	t.Run("sets message when base is empty", func(t *testing.T) {
		base := &HookResult{}
		json := &HookResult{Message: "new message"}

		mergeJSONResult(base, json)

		assert.Equal(t, "new message", base.Message)
	})
}
