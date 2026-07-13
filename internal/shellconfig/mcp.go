package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/charmbracelet/crush/internal/shell"
)

// handleMCP implements the `mcp` builtin.
//
// Usage: mcp <name> --type stdio|sse|http [--command CMD] [--args ARG ...]
//
//	[--env KEY VALUE ...] [--url URL] [--header KEY VALUE ...]
//	[--timeout N] [--disabled true|false]
//	[--disabled-tools TOOL ...] [--enabled-tools TOOL ...]
func handleMCP(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := shell.ConfigBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: mcp <name> --type stdio|sse|http [--command CMD] [--args ARG ...] [--env KEY VALUE ...] [--url URL] [--header KEY VALUE ...] [--timeout N] [--disabled true|false] [--disabled-tools TOOL ...] [--enabled-tools TOOL ...]")
	}

	name := args[1]
	slog.Info("MCP server defined in shell config", "name", name)
	f := newFragmentBuilder()
	if f.m["mcp"] == nil {
		f.m["mcp"] = make(map[string]any)
	}
	mcps := f.m["mcp"].(map[string]any)
	m := make(map[string]any)
	mcps[name] = m

	// Default type is stdio.
	m["type"] = "stdio"

	i := 2
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
			envMap, ok := m["env"].(map[string]any)
			if !ok {
				envMap = make(map[string]any)
				m["env"] = envMap
			}
			envMap[k] = v
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
			hMap, ok := m["headers"].(map[string]any)
			if !ok {
				hMap = make(map[string]any)
				m["headers"] = hMap
			}
			hMap[k] = v
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
			return usage(stderr, fmt.Sprintf("mcp: unknown flag %s", args[i]))
		}
	}

	if err := f.append(b); err != nil {
		slog.Error("Failed to append MCP fragment", "name", name, "error", err)
		return err
	}
	slog.Debug("MCP fragment appended", "name", name)
	return nil
}
