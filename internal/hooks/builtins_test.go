package hooks

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/interp"
)

func TestBuiltinsIntegration(t *testing.T) {
	t.Parallel()

	jsonInput := `{
		"prompt": "test prompt",
		"tool_input": {
			"command": "ls -la",
			"offset": 100
		},
		"custom_field": "custom_value"
	}`

	script := `
PROMPT=$(crush_get_prompt)
COMMAND=$(crush_get_tool_input "command")
OFFSET=$(crush_get_tool_input "offset")
CUSTOM=$(crush_get_input "custom_field")

echo "prompt=$PROMPT"
echo "command=$COMMAND"
echo "offset=$OFFSET"
echo "custom=$CUSTOM"

crush_log "Processing complete"
`

	hookShell := shell.NewShell(&shell.Options{
		WorkingDir:   t.TempDir(),
		ExecHandlers: []func(interp.ExecHandlerFunc) interp.ExecHandlerFunc{RegisterBuiltins},
	})

	// Need to set _CRUSH_STDIN before running the script
	stdin := strings.NewReader(jsonInput)
	setupScript := `
_CRUSH_STDIN=$(cat)
export _CRUSH_STDIN
` + script

	stdout, _, err := hookShell.ExecWithStdin(context.Background(), setupScript, stdin)

	require.NoError(t, err)
	require.Contains(t, stdout, "prompt=test prompt")
	require.Contains(t, stdout, "command=ls -la")
	require.Contains(t, stdout, "offset=100")
	require.Contains(t, stdout, "custom=custom_value")
}

func TestBuiltinErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		script  string
		stdin   string
		wantErr bool
	}{
		{
			name:    "invalid json",
			script:  `crush_get_input "field"`,
			stdin:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "wrong number of args",
			script:  `crush_get_input`,
			stdin:   `{"field":"value"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			hookShell := shell.NewShell(&shell.Options{
				WorkingDir:   t.TempDir(),
				ExecHandlers: []func(interp.ExecHandlerFunc) interp.ExecHandlerFunc{RegisterBuiltins},
			})

			setupScript := `
_CRUSH_STDIN=$(cat)
export _CRUSH_STDIN
` + tt.script

			stdin := strings.NewReader(tt.stdin)
			_, _, err := hookShell.ExecWithStdin(context.Background(), setupScript, stdin)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBuiltinWithMissingFields(t *testing.T) {
	t.Parallel()

	jsonInput := `{"prompt": "test"}`

	script := `
MISSING=$(crush_get_input "missing_field")
TOOL_MISSING=$(crush_get_tool_input "missing_param")

if [ -z "$MISSING" ]; then
  echo "missing is empty"
fi

if [ -z "$TOOL_MISSING" ]; then
  echo "tool_missing is empty"
fi
`

	hookShell := shell.NewShell(&shell.Options{
		WorkingDir:   t.TempDir(),
		ExecHandlers: []func(interp.ExecHandlerFunc) interp.ExecHandlerFunc{RegisterBuiltins},
	})

	stdin := strings.NewReader(jsonInput)
	setupScript := `
_CRUSH_STDIN=$(cat)
export _CRUSH_STDIN
` + script

	stdout, _, err := hookShell.ExecWithStdin(context.Background(), setupScript, stdin)

	require.NoError(t, err)
	require.Contains(t, stdout, "missing is empty")
	require.Contains(t, stdout, "tool_missing is empty")
}

func TestBuiltinWithComplexTypes(t *testing.T) {
	t.Parallel()

	jsonInput := `{
		"array_field": [1, 2, 3],
		"object_field": {"key": "value"},
		"bool_field": true,
		"null_field": null
	}`

	script := `
ARRAY=$(crush_get_input "array_field")
OBJECT=$(crush_get_input "object_field")
BOOL=$(crush_get_input "bool_field")
NULL=$(crush_get_input "null_field")

echo "array=$ARRAY"
echo "object=$OBJECT"
echo "bool=$BOOL"
echo "null=$NULL"
`

	hookShell := shell.NewShell(&shell.Options{
		WorkingDir:   t.TempDir(),
		ExecHandlers: []func(interp.ExecHandlerFunc) interp.ExecHandlerFunc{RegisterBuiltins},
	})

	stdin := strings.NewReader(jsonInput)
	setupScript := `
_CRUSH_STDIN=$(cat)
export _CRUSH_STDIN
` + script

	stdout, _, err := hookShell.ExecWithStdin(context.Background(), setupScript, stdin)

	require.NoError(t, err)
	require.Contains(t, stdout, "array=[1,2,3]")
	require.Contains(t, stdout, `object={"key":"value"}`)
	require.Contains(t, stdout, "bool=true")
	require.Contains(t, stdout, "null=")
}
