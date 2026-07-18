package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// handleProvider implements the `provider` builtin.
//
// Usage:
//
//	provider add <id> [--name NAME] [--type TYPE] [--api-key KEY]
//	    [--base-url URL] [--disable true|false] [--flat-rate true|false]
//	    [--system-prompt-prefix TEXT] [--extra-header KEY VALUE]
//	provider remove <id>   (alias: rm)
//
// "add" defines or updates a provider; repeated calls with the same <id>
// update the same entry. "remove" removes a provider and all its children.
func handleProvider(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := configBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: provider add <id> [flags] | provider remove <id>")
	}

	switch args[1] {
	case "add":
		return providerAdd(b, args, stderr)
	case "remove", "rm":
		return providerRemove(b, args, stderr)
	default:
		return usage(stderr, fmt.Sprintf("provider: unknown subcommand %q (expected add or remove)", args[1]))
	}
}

func providerAdd(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: provider add <id> [--name NAME] [--type TYPE] [--api-key KEY] [--base-url URL] [--disable true|false] [--flat-rate true|false] [--system-prompt-prefix TEXT] [--extra-header KEY VALUE]")
	}
	id := args[2]
	slog.Info("Provider defined in shell config", "provider", id)
	p := childMap(b.section("providers"), id)

	i := 3
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
			childMap(p, "extra_headers")[k] = v
		default:
			return usage(stderr, fmt.Sprintf("provider add: unknown flag %s", args[i]))
		}
	}

	slog.Debug("Provider recorded", "provider", id)
	return nil
}

func providerRemove(b *ConfigBuilder, args []string, stderr io.Writer) error {
	if len(args) < 3 {
		return usage(stderr, "usage: provider remove <id>")
	}
	id := args[2]
	delete(b.section("providers"), id)
	slog.Info("Provider removed in shell config", "provider", id)
	return nil
}
