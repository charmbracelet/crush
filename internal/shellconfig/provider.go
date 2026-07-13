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
	p := f.nestedMap("providers", id)

	i := 2
	for i < len(args) {
		switch args[i] {
		case "--name":
			v, err := flagStr(args, &i, "name")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p["name"] = v
		case "--type":
			v, err := flagStr(args, &i, "type")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p["type"] = v
		case "--api-key":
			v, err := flagStr(args, &i, "api-key")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p["api_key"] = v
		case "--base-url":
			v, err := flagStr(args, &i, "base-url")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p["base_url"] = v
		case "--disable":
			v, err := flagBool(args, &i, "disable")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p["disable"] = v
		case "--flat-rate":
			v, err := flagBool(args, &i, "flat-rate")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p["flat_rate"] = v
		case "--system-prompt-prefix":
			v, err := flagStr(args, &i, "system-prompt-prefix")
			if err != nil {
				return usage(stderr, err.Error())
			}
			p["system_prompt_prefix"] = v
		case "--extra-header":
			k, v, err := flagKeyValue(args, &i, "extra-header")
			if err != nil {
				return usage(stderr, err.Error())
			}
			eh, _ := p["extra_headers"].(map[string]any)
			if eh == nil {
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
