package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// handleLSP implements the `lsp` builtin.
//
// Usage: lsp <name> --command CMD [--args ARG ...] [--env KEY VALUE ...]
//
//	[--filetypes TYPE ...] [--root-markers MARKER ...]
//	[--timeout N] [--disabled true|false]
//	[--init-options JSON] [--options JSON]
func handleLSP(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: lsp <name> --command CMD [--args ARG ...] [--env KEY VALUE ...] [--filetypes TYPE ...] [--root-markers MARKER ...] [--timeout N] [--disabled true|false] [--init-options JSON] [--options JSON]")
	}

	name := args[1]
	slog.Info("LSP server defined in shell config", "name", name)
	l := childMap(b.section("lsp"), name)

	i := 2
	for i < len(args) {
		switch args[i] {
		case "--command":
			v, err := flagStr(args, &i, "command")
			if err != nil {
				return usage(stderr, err.Error())
			}
			l["command"] = v
		case "--args":
			v, err := flagStr(args, &i, "args")
			if err != nil {
				return usage(stderr, err.Error())
			}
			l["args"] = appendArr(l, "args", v)
		case "--env":
			k, v, err := flagKeyValue(args, &i, "env")
			if err != nil {
				return usage(stderr, err.Error())
			}
			childMap(l, "env")[k] = v
		case "--filetypes":
			v, err := flagStr(args, &i, "filetypes")
			if err != nil {
				return usage(stderr, err.Error())
			}
			l["filetypes"] = appendArr(l, "filetypes", v)
		case "--root-markers":
			v, err := flagStr(args, &i, "root-markers")
			if err != nil {
				return usage(stderr, err.Error())
			}
			l["root_markers"] = appendArr(l, "root_markers", v)
		case "--timeout":
			v, err := flagInt(args, &i, "timeout")
			if err != nil {
				return usage(stderr, err.Error())
			}
			l["timeout"] = v
		case "--disabled":
			v, err := flagBool(args, &i, "disabled")
			if err != nil {
				return usage(stderr, err.Error())
			}
			l["disabled"] = v
		case "--init-options":
			v, err := flagStr(args, &i, "init-options")
			if err != nil {
				return usage(stderr, err.Error())
			}
			var parsed any
			if err := jsonUnmarshal([]byte(v), &parsed); err != nil {
				return usage(stderr, fmt.Sprintf("lsp: --init-options expects valid JSON, got %q: %s", v, err))
			}
			l["init_options"] = parsed
		case "--options":
			v, err := flagStr(args, &i, "options")
			if err != nil {
				return usage(stderr, err.Error())
			}
			var parsed any
			if err := jsonUnmarshal([]byte(v), &parsed); err != nil {
				return usage(stderr, fmt.Sprintf("lsp: --options expects valid JSON, got %q: %s", v, err))
			}
			l["options"] = parsed
		default:
			return usage(stderr, fmt.Sprintf("lsp: unknown flag %s", args[i]))
		}
	}

	slog.Debug("LSP recorded", "name", name)
	return nil
}
