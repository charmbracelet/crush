package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// handlePermissions implements the `permissions` builtin.
//
// Usage:
//
//	permissions allow <tool> [<tool> ...]
//	permissions deny <tool> [<tool> ...]
//	permissions safe <command line> [<command line> ...]
//	permissions block <command> [<command> ...]
//	permissions unblock <command> [<command> ...]
//
// The first two act on tools. "allow" adds tools to the allow-list (tools
// that skip permission prompts). "deny" hides tools from the agent entirely
// (options.disabled_tools) — the inverse of allow. Adding the same tool twice
// is a no-op.
//
// Precedence: deny wins. If a tool appears in both allow and deny, it is
// still removed from the agent's effective tool set via disabled_tools.
//
// The last three act on shell commands rather than tools, which is a
// different question: not "may the agent run commands at all" but "which
// commands are quiet, and which are worth a warning".
//
//	safe     runs without a prompt, matched as the exact command line given
//	block    treated as dangerous, so it is warned about and prompted for
//	unblock  drops a command from the dangerous list, built-in ones included
func handlePermissions(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: permissions allow|deny|safe|block|unblock <value> [<value> ...]")
	}

	switch args[1] {
	case "allow":
		return permissionsAllow(b, args, stderr)
	case "deny":
		return permissionsDeny(b, args, stderr)
	case "safe":
		return permissionsCommandList(b, args, stderr, "safe_commands")
	case "block":
		return permissionsCommandList(b, args, stderr, "blocked_commands")
	case "unblock":
		return permissionsCommandList(b, args, stderr, "allowed_commands")
	default:
		return usage(stderr, fmt.Sprintf("permissions: unknown subcommand %q (expected allow, deny, safe, block or unblock)", args[1]))
	}
}

// permissionsCommandList appends command entries to one of the command lists
// under permissions. Each argument is one entry, so a command line with
// arguments is given as a single quoted word:
//
//	permissions safe "go build" "cargo check"
func permissionsCommandList(b *ConfigBuilder, args []string, stderr io.Writer, field string) error {
	if len(args) < 3 {
		return usage(stderr, fmt.Sprintf("usage: permissions %s <command> [<command> ...]", args[1]))
	}
	perms := b.section("permissions")
	list, _ := perms[field].([]any)

	for _, cmd := range args[2:] {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		if !containsAny(list, cmd) {
			list = append(list, cmd)
		}
	}
	perms[field] = list

	slog.Info("Permission commands set in shell config", "field", field, "commands", args[2:])
	return nil
}

func permissionsAllow(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: permissions allow <tool> [<tool> ...]")
	}
	perms := b.section("permissions")
	allowed, _ := perms["allowed_tools"].([]any)

	for _, tool := range args[2:] {
		if !containsAny(allowed, tool) {
			allowed = append(allowed, tool)
		}
	}
	perms["allowed_tools"] = allowed

	slog.Info("Permissions allowed in shell config", "tools", args[2:])
	return nil
}

// permissionsDeny hides tools from the agent by adding them to
// options.disabled_tools. It is the inverse of allow.
func permissionsDeny(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: permissions deny <tool> [<tool> ...]")
	}
	opts := b.section("options")
	disabled, _ := opts["disabled_tools"].([]any)

	for _, tool := range args[2:] {
		if !containsAny(disabled, tool) {
			disabled = append(disabled, tool)
		}
	}
	opts["disabled_tools"] = disabled

	slog.Info("Permissions denied in shell config", "tools", args[2:])
	return nil
}

// containsAny reports whether the slice already holds the given string value.
func containsAny(s []any, v string) bool {
	for _, item := range s {
		if str, ok := item.(string); ok && str == v {
			return true
		}
	}
	return false
}
