package shellconfig

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/charmbracelet/crush/internal/shell"
)

// handleOptions implements the `options` builtin.
//
// Usage: options [--data-directory PATH] [--context-path PATH ...]
//
//	[--global-context-path PATH ...] [--skills-path PATH ...]
//	[--debug true|false] [--debug-lsp true|false]
//	[--disable-auto-summarize true|false]
//	[--disable-provider-auto-update true|false]
//	[--disable-default-providers true|false]
//	[--disable-metrics true|false]
//	[--disable-notifications true|false]
//	[--initialize-as NAME] [--notification-style STYLE]
//	[--disabled-tools TOOL ...] [--disabled-skills SKILL ...]
//	[--auto-lsp true|false] [--progress true|false]
func handleOptions(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	b := shell.ConfigBuilderFromCtx(ctx)
	if b == nil {
		return nil
	}
	slog.Info("Options defined in shell config")
	f := newFragmentBuilder()
	if f.m["options"] == nil {
		f.m["options"] = make(map[string]any)
	}
	o := f.m["options"].(map[string]any)

	i := 1
	for i < len(args) {
		switch args[i] {
		case "--data-directory":
			v, err := flagStr(args, &i, "data-directory")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["data_directory"] = v
		case "--context-path":
			v, err := flagStr(args, &i, "context-path")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["context_paths"] = appendArr(o, "context_paths", v)
		case "--global-context-path":
			v, err := flagStr(args, &i, "global-context-path")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["global_context_paths"] = appendArr(o, "global_context_paths", v)
		case "--skills-path":
			v, err := flagStr(args, &i, "skills-path")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["skills_paths"] = appendArr(o, "skills_paths", v)
		case "--debug":
			v, err := flagBool(args, &i, "debug")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["debug"] = v
		case "--debug-lsp":
			v, err := flagBool(args, &i, "debug-lsp")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["debug_lsp"] = v
		case "--disable-auto-summarize":
			v, err := flagBool(args, &i, "disable-auto-summarize")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["disable_auto_summarize"] = v
		case "--disable-provider-auto-update":
			v, err := flagBool(args, &i, "disable-provider-auto-update")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["disable_provider_auto_update"] = v
		case "--disable-default-providers":
			v, err := flagBool(args, &i, "disable-default-providers")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["disable_default_providers"] = v
		case "--disable-metrics":
			v, err := flagBool(args, &i, "disable-metrics")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["disable_metrics"] = v
		case "--disable-notifications":
			v, err := flagBool(args, &i, "disable-notifications")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["disable_notifications"] = v
		case "--initialize-as":
			v, err := flagStr(args, &i, "initialize-as")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["initialize_as"] = v
		case "--notification-style":
			v, err := flagStr(args, &i, "notification-style")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["notification_style"] = v
		case "--disabled-tools":
			v, err := flagStr(args, &i, "disabled-tools")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["disabled_tools"] = appendArr(o, "disabled_tools", v)
		case "--disabled-skills":
			v, err := flagStr(args, &i, "disabled-skills")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["disabled_skills"] = appendArr(o, "disabled_skills", v)
		case "--auto-lsp":
			v, err := flagBool(args, &i, "auto-lsp")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["auto_lsp"] = v
		case "--progress":
			v, err := flagBool(args, &i, "progress")
			if err != nil {
				return usage(stderr, err.Error())
			}
			o["progress"] = v
		case "--no-auto-lsp":
			o["auto_lsp"] = false
			i++
		case "--no-progress":
			o["progress"] = false
			i++
		default:
			return usage(stderr, fmt.Sprintf("options: unknown flag %s", args[i]))
		}
	}

	if err := f.append(b); err != nil {
		slog.Error("Failed to append options fragment", "error", err)
		return err
	}
	slog.Debug("Options fragment appended")
	return nil
}
