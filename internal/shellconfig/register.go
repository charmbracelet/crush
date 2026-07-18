// Package shellconfig implements the Bash-powered config format for Crush.
//
// It provides shell builtins (provider, model, mcp, lsp, permissions, hook,
// options) that populate config via JSON fragments stored on a
// shell.ConfigBuilder. The builtins are registered at init time via
// shell.RegisterBuiltin and are gated by the ConfigBuilder on the context —
// they are no-ops during normal bash tool execution.
//
// This package sits between shell and config: it imports shell (for the
// ConfigBuilder and RegisterBuiltin), and config imports shell (for
// ExpandValue). The shellconfig package is imported by the config loader
// to run crush.sh files.
package shellconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/crush/internal/shell"
)

func init() {
	shell.RegisterBuiltin("provider", handleProvider)
	shell.RegisterBuiltin("provider-model", handleProviderModel)
	shell.RegisterBuiltin("model", handleModel)
	shell.RegisterBuiltin("mcp", handleMCP)
	shell.RegisterBuiltin("lsp", handleLSP)
	shell.RegisterBuiltin("permissions", handlePermissions)
	shell.RegisterBuiltin("hook", handleHook)
	shell.RegisterBuiltin("option", handleOption)
}

// fragmentBuilder helps accumulate key-value pairs into a JSON object
// before appending to the ConfigBuilder.
type fragmentBuilder struct {
	m map[string]any
}

func newFragmentBuilder() *fragmentBuilder {
	return &fragmentBuilder{m: make(map[string]any)}
}

// nestedMap returns the map at f.m[parent][key], creating both levels if
// needed. The returned map can be mutated directly by callers.
func (f *fragmentBuilder) nestedMap(parent, key string) map[string]any {
	p, ok := f.m[parent].(map[string]any)
	if !ok {
		p = make(map[string]any)
		f.m[parent] = p
	}
	inner, ok := p[key].(map[string]any)
	if !ok {
		inner = make(map[string]any)
		p[key] = inner
	}
	return inner
}

// rootMap returns the map at f.m[key], creating it if needed.
func (f *fragmentBuilder) rootMap(key string) map[string]any {
	m, ok := f.m[key].(map[string]any)
	if !ok {
		m = make(map[string]any)
		f.m[key] = m
	}
	return m
}

// append marshals the fragment to JSON and adds it to the builder.
func (f *fragmentBuilder) append(b *shell.ConfigBuilder) error {
	data, err := json.Marshal(f.m)
	if err != nil {
		return err
	}
	b.AppendFragment(data)
	return nil
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

// flagBool parses the next arg as a boolean for the given flag.
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

// jsonUnmarshal is a wrapper around json.Unmarshal for use by handlers
// that accept JSON string flags (e.g. --init-options).
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// usage prints a usage message to stderr and returns exit code 2.
func usage(stderr io.Writer, msg string) error {
	fmt.Fprintln(stderr, msg)
	return fmt.Errorf("%s", msg)
}

// appendArr returns the slice for the given key, appending value. If the key
// doesn't exist yet, creates a new slice.
func appendArr(m map[string]any, key, value string) []any {
	arr, _ := m[key].([]any)
	return append(arr, value)
}

// Compiler note: ctx, stdin, stdout params are used by some handlers and
// unused by others. They exist for signature compatibility with
// shell.BuiltinHandler.
var _ = context.Background
