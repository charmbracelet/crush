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
	perms := f.rootMap("permissions")

	i := 1
	for i < len(args) {
		switch args[i] {
		case "--allow":
			v, err := flagStr(args, &i, "allow")
			if err != nil {
				return usage(stderr, err.Error())
			}
			perms["allowed_tools"] = appendArr(perms, "allowed_tools", v)
		default:
			return usage(stderr, fmt.Sprintf("permissions: unknown flag %s", args[i]))
		}
	}

	return f.append(b)
}
