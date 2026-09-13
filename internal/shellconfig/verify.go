package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// handleVerify implements the `verify` builtin.
//
// Usage:
//
//	verify add --command CMD [--name NAME] [--timeout N]
//	verify remove [--name NAME]   (alias: rm)
//
// "add" appends a check command to the verify list; multiple commands
// accumulate. "remove" drops the named check(s), or clears the whole list
// when no --name is given. Only named checks can be removed individually.
//
// Trust boundary: verify commands run at the end-of-turn gate without the
// bash tool's permission prompt and without its banned-command blocking —
// declaring a command here pre-approves it. A command runs once per gate
// turn and once per retry, so it must be idempotent and side-effect-safe.
func handleVerify(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: verify add --command CMD [--name NAME] [--timeout N] | verify remove [--name NAME]")
	}

	switch args[1] {
	case "add":
		return verifyAdd(b, args, stderr)
	case "remove", "rm":
		return verifyRemove(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("verify: unknown subcommand %q (expected add or remove)", args[1]))
	}
}

// verifyAddFlags is the declarative flag surface for `verify add`.
var verifyAddFlags = []flagSpec{
	{name: "--command", jsonKey: "command", kind: flagString, op: opSet},
	{name: "--name", jsonKey: "name", kind: flagString, op: opSet},
	{name: "--timeout", jsonKey: "timeout", kind: flagInt, op: opSet},
}

func verifyAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	h := map[string]any{}
	if err := applyFlags(verifyAddFlags, args, 2, h, "verify add", stderr); err != nil {
		return err
	}
	if cmd, ok := h["command"].(string); !ok || strings.TrimSpace(cmd) == "" {
		return usage(stderr, "verify add: --command is required and must not be empty")
	}

	arr, _ := b.root["verify"].([]any)
	b.root["verify"] = append(arr, h)

	slog.Info("Verify check defined in shell config", "command", h["command"])
	return nil
}

// verifyRemoveFlags is the declarative flag surface for `verify remove`.
var verifyRemoveFlags = []flagSpec{
	{name: "--name", jsonKey: "name", kind: flagString, op: opSet},
}

func verifyRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	flags := map[string]any{}
	if err := applyFlags(verifyRemoveFlags, args, 2, flags, "verify remove", stderr); err != nil {
		return err
	}
	name, _ := flags["name"].(string)

	// No name: clear every check.
	if name == "" {
		delete(b.root, "verify")
		slog.Info("Verify checks cleared in shell config")
		return nil
	}

	arr, _ := b.root["verify"].([]any)
	kept := make([]any, 0, len(arr))
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok && m["name"] == name {
			continue
		}
		kept = append(kept, item)
	}
	if len(kept) == 0 {
		delete(b.root, "verify")
	} else {
		b.root["verify"] = kept
	}

	slog.Info("Verify check removed in shell config", "name", name)
	return nil
}
