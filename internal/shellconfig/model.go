package shellconfig

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/crush/internal/shell"
)

// handleModel implements the `model` builtin.
//
// Usage: model <large|small> --provider <id> --model <name>
//
//	[--think] [--reasoning-effort low|medium|high]
//	[--max-tokens N] [--temperature F]
func handleModel(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := shell.ConfigBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	if len(args) < 2 {
		return usage(stderr, "usage: model <large|small> --provider <id> --model <name> [--think] [--reasoning-effort low|medium|high] [--max-tokens N] [--temperature F]")
	}

	modelType := args[1]
	switch modelType {
	case "large", "small":
	default:
		return usage(stderr, fmt.Sprintf("model: type must be 'large' or 'small', got %q", modelType))
	}

	f := newFragmentBuilder()
	if f.m["models"] == nil {
		f.m["models"] = make(map[string]any)
	}
	models := f.m["models"].(map[string]any)
	m := make(map[string]any)
	models[modelType] = m

	i := 2
	for i < len(args) {
		switch args[i] {
		case "--provider":
			v, err := flagStr(args, &i, "provider")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["provider"] = v
		case "--model":
			v, err := flagStr(args, &i, "model")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["model"] = v
		case "--think":
			m["think"] = true
			i++
		case "--reasoning-effort":
			v, err := flagStr(args, &i, "reasoning-effort")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["reasoning_effort"] = v
		case "--max-tokens":
			v, err := flagInt(args, &i, "max-tokens")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["max_tokens"] = v
		case "--temperature":
			v, err := flagFloat64(args, &i, "temperature")
			if err != nil {
				return usage(stderr, err.Error())
			}
			m["temperature"] = v
		default:
			return usage(stderr, fmt.Sprintf("model: unknown flag %s", args[i]))
		}
	}

	if _, ok := m["model"]; !ok {
		return usage(stderr, "model: --model is required")
	}
	if _, ok := m["provider"]; !ok {
		return usage(stderr, "model: --provider is required")
	}

	return f.append(b)
}
