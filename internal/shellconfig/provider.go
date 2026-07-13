package shellconfig

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/crush/internal/shell"
)

// handleProvider implements the `provider` builtin.
//
// Usage: provider <id> [--name NAME] [--type TYPE] [--api-key KEY]
//
//	[--base-url URL] [--disable true|false] [--flat-rate true|false]
//	[--system-prompt-prefix TEXT] [--extra-header KEY VALUE]
//
// Each call appends a provider fragment to the ConfigBuilder. Multiple calls
// with the same ID will be deep-merged by the config loader.
func handleProvider(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := shell.ConfigBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: provider <id> [--name NAME] [--type TYPE] [--api-key KEY] [--base-url URL] [--disable true|false] [--flat-rate true|false] [--system-prompt-prefix TEXT] [--extra-header KEY VALUE]")
	}

	id := args[1]
	f := newFragmentBuilder()
	f.setNested("providers", id, map[string]any{})

	i := 2
	for i < len(args) {
		switch args[i] {
		case "--name":
			v, err := flagStr(args, &i, "name")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.setNested("providers", id, mergeInto(f.m["providers"].(map[string]any), "name", v))
		case "--type":
			v, err := flagStr(args, &i, "type")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.setNested("providers", id, mergeInto(f.m["providers"].(map[string]any), "type", v))
		case "--api-key":
			v, err := flagStr(args, &i, "api-key")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.setNested("providers", id, mergeInto(f.m["providers"].(map[string]any), "api_key", v))
		case "--base-url":
			v, err := flagStr(args, &i, "base-url")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.setNested("providers", id, mergeInto(f.m["providers"].(map[string]any), "base_url", v))
		case "--disable":
			v, err := flagBool(args, &i, "disable")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.setNested("providers", id, mergeInto(f.m["providers"].(map[string]any), "disable", v))
		case "--flat-rate":
			v, err := flagBool(args, &i, "flat-rate")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.setNested("providers", id, mergeInto(f.m["providers"].(map[string]any), "flat_rate", v))
		case "--system-prompt-prefix":
			v, err := flagStr(args, &i, "system-prompt-prefix")
			if err != nil {
				return usage(stderr, err.Error())
			}
			f.setNested("providers", id, mergeInto(f.m["providers"].(map[string]any), "system_prompt_prefix", v))
		case "--extra-header":
			k, v, err := flagKeyValue(args, &i, "extra-header")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p := f.m["providers"].(map[string]any)
			eh, ok := p["extra_headers"].(map[string]any)
			if !ok {
				eh = make(map[string]any)
				p["extra_headers"] = eh
			}
			eh[k] = v
		default:
			return usage(stderr, fmt.Sprintf("provider: unknown flag %s", args[i]))
		}
	}

	return f.append(b)
}
