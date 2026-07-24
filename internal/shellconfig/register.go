// Package shellconfig implements the Bash-powered config format for Crush.
//
// It provides shell builtins (provider, model, mcp, lsp, permissions, hook,
// option) that populate config by mutating a ConfigBuilder
// stored on the shell context. The builtins are registered at init time via
// shell.RegisterBuiltin and are gated by the ConfigBuilder on the context —
// they are no-ops during normal bash tool execution.
//
// This package sits between shell and config: it imports shell (for
// RegisterBuiltin and Run), and config imports shellconfig to run crushrc
// files.
package shellconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
)

func init() {
	shell.RegisterBuiltin("provider", handleProvider)
	shell.RegisterBuiltin("model", handleModel)
	shell.RegisterBuiltin("mcp", handleMCP)
	shell.RegisterBuiltin("lsp", handleLSP)
	shell.RegisterBuiltin("permissions", handlePermissions)
	shell.RegisterBuiltin("hook", handleHook)
	shell.RegisterBuiltin("option", handleOption)
}

// flagStr returns the value at args[i+1] as a string and advances i by 2.
func flagStr(args []string, i *int, flag string) (string, error) {
	if *i+1 >= len(args) {
		return "", fmt.Errorf("%s: --%s requires a value", args[0], flag)
	}
	v := args[*i+1]
	*i += 2
	return v, nil
}

// flagBool parses the next arg as a boolean for the given flag. Values are
// case-insensitive.
func flagBool(args []string, i *int, flag string) (bool, error) {
	v, err := flagStr(args, i, flag)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(v) {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s: --%s expects true/false, got %q", args[0], flag, v)
	}
}

// flagInt parses the next arg as an int.
func flagInt(args []string, i *int, flag string) (int, error) {
	v, err := flagStr(args, i, flag)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s: --%s expects an integer, got %q", args[0], flag, v)
	}
	return n, nil
}

// flagFloat64 parses the next arg as a float64.
func flagFloat64(args []string, i *int, flag string) (float64, error) {
	v, err := flagStr(args, i, flag)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: --%s expects a number, got %q", args[0], flag, v)
	}
	return f, nil
}

// flagKeyValue returns the key at args[i+1] and value at args[i+2], advancing i by 3.
func flagKeyValue(args []string, i *int, flag string) (string, string, error) {
	if *i+2 >= len(args) {
		return "", "", fmt.Errorf("%s: --%s requires a key and value", args[0], flag)
	}
	k := args[*i+1]
	v := args[*i+2]
	*i += 3
	return k, v, nil
}

// jsonUnmarshal is a wrapper around json.Unmarshal for use by handlers that
// accept JSON string flags (e.g. --init-options).
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func flagJSONObject(args []string, i *int, flag string) (map[string]any, error) {
	value, err := flagStr(args, i, flag)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(value), &object); err != nil || object == nil {
		return nil, fmt.Errorf("%s: --%s expects a JSON object, got %q", args[0], flag, value)
	}
	return object, nil
}

func mergeMap(target map[string]any, source map[string]any) {
	maps.Copy(target, source)
}

// usage prints a usage message to stderr and returns an error.
func usage(stderr io.Writer, msg string) error {
	fmt.Fprintln(stderr, msg)
	return fmt.Errorf("%s", msg)
}

// appendArr appends value to the string slice stored at m[key], creating it if
// needed, and returns the result.
func appendArr(m map[string]any, key, value string) []any {
	arr, _ := m[key].([]any)
	return append(arr, value)
}
