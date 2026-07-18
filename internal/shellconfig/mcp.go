package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// handleMCP implements the `mcp` builtin.
//
// Usage:
//
//	mcp add <name> --type stdio|sse|http [--command CMD] [--args ARG ...]
//	    [--env KEY VALUE ...] [--url URL] [--header KEY VALUE ...]
//	    [--timeout N] [--disabled true|false]
//	    [--disabled-tools TOOL ...] [--enabled-tools TOOL ...]
//	mcp remove <name>   (alias: rm)
//
// "add" defines or updates an MCP server; repeated calls with the same <name>
// update the same entry. "remove" deletes it.
func handleMCP(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: mcp add <name> --type stdio|sse|http [flags] | mcp remove <name>")
	}

	switch args[1] {
	case "add":
		return mcpAdd(b, args, stderr)
	case "remove", "rm":
		return mcpRemove(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("mcp: unknown subcommand %q (expected add or remove)", args[1]))
	}
}

func mcpAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: mcp add <name> --type stdio|sse|http [--command CMD] [--args ARG ...] [--env KEY VALUE ...] [--url URL] [--header KEY VALUE ...] [--timeout N] [--disabled true|false] [--disabled-tools TOOL ...] [--enabled-tools TOOL ...]")
	}
	name := args[2]
	slog.Info("MCP server defined in shell config", "name", name)
	m := childMap(b.section("mcp"), name)

	// Default type is stdio.
	if _, ok := m["type"]; !ok {
		m["type"] = "stdio"
	}

	i := 3
	for i < len(args) {
		switch args[i] {
		case "--type":
			v, err := flagStr(args, &i, "type")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["type"] = v
		case "--command":
			v, err := flagStr(args, &i, "command")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["command"] = v
		case "--args":
			v, err := flagStr(args, &i, "args")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["args"] = appendArr(m, "args", v)
		case "--env":
			k, v, err := flagKeyValue(args, &i, "env")
			if err != nil {
				return usage(stderr, err.Error())
			}
			childMap(m, "env")[k] = v
		case "--url":
			v, err := flagStr(args, &i, "url")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["url"] = v
		case "--header":
			k, v, err := flagKeyValue(args, &i, "header")
			if err != nil {
				return usage(stderr, err.Error())
			}
			childMap(m, "headers")[k] = v
		case "--timeout":
			v, err := flagInt(args, &i, "timeout")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["timeout"] = v
		case "--disabled":
			v, err := flagBool(args, &i, "disabled")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["disabled"] = v
		case "--disabled-tools":
			v, err := flagStr(args, &i, "disabled-tools")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["disabled_tools"] = appendArr(m, "disabled_tools", v)
		case "--enabled-tools":
			v, err := flagStr(args, &i, "enabled-tools")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["enabled_tools"] = appendArr(m, "enabled_tools", v)
		default:
			return usage(stderr, fmt.Sprintf("mcp add: unknown flag %s", args[i]))
		}
	}

	slog.Debug("MCP recorded", "name", name)
	return nil
}

func mcpRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: mcp remove <name>")
	}
	name := args[2]
	delete(b.section("mcp"), name)
	slog.Info("MCP server removed in shell config", "name", name)
	return nil
}
