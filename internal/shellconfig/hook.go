package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// handleHook implements the `hook` builtin.
//
// Usage:
//
//	hook add <event> --command CMD [--name NAME] [--matcher REGEX] [--timeout N]
//	hook remove <event> [--name NAME]   (alias: rm)
//
// "add" appends a hook to the event's list; multiple hooks per event
// accumulate. "remove" drops the named hook(s) from the event, or clears the
// whole event when no --name is given. Only named hooks can be removed
// individually; give a hook a --name if you intend to remove it later.
func handleHook(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: hook add <event> --command CMD [flags] | hook remove <event> [--name NAME]")
	}

	switch args[1] {
	case "add":
		return hookAdd(b, args, stderr)
	case "remove", "rm":
		return hookRemove(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("hook: unknown subcommand %q (expected add or remove)", args[1]))
	}
}

func hookAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: hook add <event> --command CMD [--name NAME] [--matcher REGEX] [--timeout N]")
	}
	event := args[2]
	slog.Info("Hook defined in shell config", "event", event)
	h := map[string]any{}

	i := 3
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
			return usage(stderr, fmt.Sprintf("hook add: unknown flag %s", args[i]))
		}
	}

	if _, ok := h["command"]; !ok {
		return usage(stderr, "hook add: --command is required")
	}

	hooks := b.section("hooks")
	arr, _ := hooks[event].([]any)
	hooks[event] = append(arr, h)

	slog.Debug("Hook recorded", "event", event)
	return nil
}

func hookRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: hook remove <event> [--name NAME]")
	}
	event := args[2]

	var name string
	i := 3
	for i < len(args) {
		switch args[i] {
		case "--name":
			v, err := flagStr(args, &i, "name")
			if err != nil {
				return usage(stderr, err.Error())
			}
			name = v
		default:
			return usage(stderr, fmt.Sprintf("hook remove: unknown flag %s", args[i]))
		}
	}

	hooks := b.section("hooks")

	// No name: clear every hook for the event.
	if name == "" {
		delete(hooks, event)
		slog.Info("Hooks cleared in shell config", "event", event)
		return nil
	}

	// Name given: drop matching hooks, keeping the rest.
	arr, _ := hooks[event].([]any)
	kept := make([]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok && m["name"] == name {
			continue
		}
		kept = append(kept, item)
	}
	hooks[event] = kept

	slog.Info("Hook removed in shell config", "event", event, "name", name)
	return nil
}
