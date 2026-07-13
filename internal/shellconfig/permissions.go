package shellconfig

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/crush/internal/shell"
)

// handlePermissions implements the `permissions` builtin.
//
// Usage: permissions --allow TOOL [--allow TOOL ...]
func handlePermissions(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := shell.ConfigBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	f := newFragmentBuilder()

	i := 1
	for i < len(args) {
		switch args[i] {
		case "--allow":
			v, err := flagStr(args, &i, "allow")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.m["permissions"] = mergeAllowedTools(f.m, v)
		default:
			return usage(stderr, fmt.Sprintf("permissions: unknown flag %s", args[i]))
		}
	}

	return f.append(b)
}

// mergeAllowedTools returns a permissions object with the tool appended to
// the allowed_tools array.
func mergeAllowedTools(existing any, tool string) map[string]any {
	m, ok := existing.(map[string]any)
	if !ok {
		m = make(map[string]any)
	}
	arr, _ := m["allowed_tools"].([]any)
	m["allowed_tools"] = append(arr, tool)
	return m
}
