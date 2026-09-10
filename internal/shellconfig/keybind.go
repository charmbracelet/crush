package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/charmbracelet/crush/internal/keybinds"
)

// handleKeybind implements the `keybind` builtin.
//
// Usage:
//
//	keybind set <action> <keys...>
//	keybind unset <action>
//	keybind disable <action>
//	keybind reset
//
// "set" replaces the override for the action; later sets win. "unset"
// drops the override so the default returns. "disable" keeps the
// override but empties it so the action never fires. "reset" clears
// every keybind set so far in the script.
func handleKeybind(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: keybind set <action> <keys...> | keybind unset <action> | keybind disable <action> | keybind reset")
	}

	switch args[1] {
	case "set":
		return keybindSet(b, args, stderr)
	case "unset":
		return keybindUnset(b, args, stderr)
	case "disable":
		return keybindDisable(b, args, stderr)
	case "reset":
		return keybindReset(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("keybind: unknown subcommand %q (expected set, unset, disable or reset)", args[1]))
	}
}

func keybindSet(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 4 {
		return usage(stderr, "usage: keybind set <action> <keys...>")
	}
	action, keys := args[2], args[3:]
	if err := keybinds.Validate(action, keys); err != nil {
		return usage(stderr, err.Error())
	}
	normalized := make([]any, len(keys))
	for i, k := range keys {
		normalized[i] = k
	}
	b.section("keybinds")[action] = normalized
	slog.Info("Keybind set in shell config", "action", action, "keys", keys)
	return nil
}

func keybindUnset(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) != 3 {
		return usage(stderr, "usage: keybind unset <action>")
	}
	action := args[2]
	// Shape-check only: unset takes no keys, so Validate would
	// reject it for missing keys rather than a bad shape.
	if !keybinds.ValidShape(action) {
		return usage(stderr, fmt.Sprintf("invalid action %q (expected scope.name)", action))
	}
	delete(b.section("keybinds"), action)
	slog.Info("Keybind unset in shell config", "action", action)
	return nil
}

func keybindDisable(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) != 3 {
		return usage(stderr, "usage: keybind disable <action>")
	}
	action := args[2]
	// Same shape check as unset: disable takes no keys either.
	if !keybinds.ValidShape(action) {
		return usage(stderr, fmt.Sprintf("invalid action %q (expected scope.name)", action))
	}
	b.section("keybinds")[action] = []any{}
	slog.Info("Keybind disabled in shell config", "action", action)
	return nil
}

func keybindReset(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) != 2 {
		return usage(stderr, "usage: keybind reset")
	}
	delete(b.root, "keybinds")
	slog.Info("Keybinds reset in shell config")
	return nil
}
