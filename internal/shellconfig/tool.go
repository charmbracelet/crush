package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// handleTool implements the `tool` builtin.
//
// Usage:
//
//	tool add <name> --description TEXT --command CMD [--param NAME DESC ...]
//	    [--schema JSON] [--required NAME ...] [--timeout N]
//	    [--env KEY VALUE ...] [--disabled true|false]
//	    [--skip-permission true|false] [--title TEMPLATE] [--summary TEMPLATE]
//	tool action <tool> <action> --description TEXT [--command CMD]
//	    [--param NAME DESC ...] [--required NAME ...]
//	    [--skip-permission true|false] [--title TEMPLATE] [--summary TEMPLATE]
//	tool remove <name>   (alias: rm)
//
// "add" defines or updates a custom agent tool; repeated calls with the same
// <name> update the same entry. When the agent calls the tool, CMD runs in the
// embedded shell with each declared parameter as a shell variable and the
// call's JSON input on stdin. "action" adds a named operation to a tool, so
// one tool can cover a whole CLI with its own command and permission per
// operation. --title and --summary are text/template strings, filled from a
// call's parameters, for how the call shows in the chat. "remove" deletes a
// tool.
func handleTool(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: tool add <name> --description TEXT --command CMD [flags] | tool action <tool> <action> --description TEXT [flags] | tool remove <name>")
	}

	switch args[1] {
	case "add":
		return toolAdd(b, args, stderr)
	case "action":
		return toolAction(b, args, stderr)
	case "remove", "rm":
		return toolRemove(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("tool: unknown subcommand %q (expected add, action, or remove)", args[1]))
	}
}

// toolAddFlags is the declarative flag surface for `tool add`.
var toolAddFlags = []flagSpec{
	{name: "--description", jsonKey: "description", kind: flagString, op: opSet},
	{name: "--command", jsonKey: "command", kind: flagString, op: opSet},
	{name: "--schema", jsonKey: "schema", kind: flagJSONObject, op: opSet},
	{name: "--param", child: "params", kind: flagKeyValue, op: opSetChild},
	{name: "--required", jsonKey: "required", kind: flagString, op: opAppend},
	{name: "--timeout", jsonKey: "timeout", kind: flagInt, op: opSet},
	{name: "--env", child: "env", kind: flagKeyValue, op: opSetChild},
	{name: "--disabled", jsonKey: "disabled", kind: flagBool, op: opSet},
	{name: "--skip-permission", jsonKey: "skip_permission", kind: flagBool, op: opSet},
	{name: "--title", jsonKey: "title", kind: flagString, op: opSet},
	{name: "--summary", jsonKey: "summary", kind: flagString, op: opSet},
}

// toolActionFlags is the declarative flag surface for `tool action`.
var toolActionFlags = []flagSpec{
	{name: "--description", jsonKey: "description", kind: flagString, op: opSet},
	{name: "--command", jsonKey: "command", kind: flagString, op: opSet},
	{name: "--param", child: "params", kind: flagKeyValue, op: opSetChild},
	{name: "--required", jsonKey: "required", kind: flagString, op: opAppend},
	{name: "--skip-permission", jsonKey: "skip_permission", kind: flagBool, op: opSet},
	{name: "--title", jsonKey: "title", kind: flagString, op: opSet},
	{name: "--summary", jsonKey: "summary", kind: flagString, op: opSet},
}

func toolAction(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 4 {
		return usage(stderr, "usage: tool action <tool> <action> --description TEXT [--command CMD] [--param NAME DESC ...] [--required NAME ...] [--skip-permission true|false]")
	}
	name, action := args[2], args[3]
	t := childMap(b.section("custom_tools"), name)
	a := childMap(childMap(t, "actions"), action)
	before, _ := a["command"].(string)

	if err := applyFlags(toolActionFlags, args, 4, a, "tool action", stderr); err != nil {
		return err
	}
	if after, _ := a["command"].(string); after != before {
		t["source"] = b.source
	}

	slog.Debug("Tool action recorded", "tool", name, "action", action)
	return nil
}

func toolAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: tool add <name> --description TEXT --command CMD [--param NAME DESC ...] [--schema JSON] [--required NAME ...] [--timeout N] [--env KEY VALUE ...] [--disabled true|false]")
	}
	name := args[2]
	slog.Info("Tool defined in shell config", "tool", name)
	t := childMap(b.section("custom_tools"), name)
	before, _ := t["command"].(string)

	if err := applyFlags(toolAddFlags, args, 3, t, "tool add", stderr); err != nil {
		return err
	}
	// Required fields are checked after every config is merged, not here,
	// so a project crushrc can tweak a plugin's tool (say, --timeout)
	// without repeating its command. The source follows the command: it is
	// the file a command may source, so only the script that set it counts.
	if after, _ := t["command"].(string); after != before {
		t["source"] = b.source
	}

	slog.Debug("Tool recorded", "tool", name)
	return nil
}

func toolRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: tool remove <name>")
	}
	name := args[2]
	delete(b.section("custom_tools"), name)
	slog.Info("Tool removed in shell config", "tool", name)
	return nil
}
