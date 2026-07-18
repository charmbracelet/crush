package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// handleHook implements the `hook` builtin.
//
// Usage: hook <event> --command CMD [--matcher REGEX] [--timeout N] [--name NAME]
//
// Multiple hooks for the same event accumulate into an array.
func handleHook(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: hook <event> --command CMD [--matcher REGEX] [--timeout N] [--name NAME]")
	}

	event := args[1]
	slog.Info("Hook defined in shell config", "event", event)
	h := map[string]any{}

	i := 2
	for i < len(args) {
		switch args[i] {
		case "--command":
			v, err := flagStr(args, &i, "command")
			if err != nil {
				return usage(stderr, err.Error())
			}
			h["command"] = v
		case "--matcher":
			v, err := flagStr(args, &i, "matcher")
			if err != nil {
				return usage(stderr, err.Error())
			}
			h["matcher"] = v
		case "--timeout":
			v, err := flagInt(args, &i, "timeout")
			if err != nil {
				return usage(stderr, err.Error())
			}
			h["timeout"] = v
		case "--name":
			v, err := flagStr(args, &i, "name")
			if err != nil {
				return usage(stderr, err.Error())
			}
			h["name"] = v
		default:
			return usage(stderr, fmt.Sprintf("hook: unknown flag %s", args[i]))
		}
	}

	if _, ok := h["command"]; !ok {
		return usage(stderr, "hook: --command is required")
	}

	hooks := b.section("hooks")
	arr, _ := hooks[event].([]any)
	hooks[event] = append(arr, h)

	slog.Debug("Hook recorded", "event", event)
	return nil
}
