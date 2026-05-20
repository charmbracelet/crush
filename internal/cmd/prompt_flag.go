package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// addPromptFlag registers --prompt on flags.
func addPromptFlag(flags *pflag.FlagSet) {
	flags.StringArray("prompt", nil, "Replace a built-in prompt for this run (repeatable), e.g. --prompt coder=./coder.md.tpl")
}

// promptOverrides parses the repeatable --prompt name=path flag. Paths are
// made absolute against the invoking shell's cwd, which is what someone
// typing a relative path on the command line means, and which survives
// being sent to a server whose cwd is somewhere else.
func promptOverrides(cmd *cobra.Command) (config.PromptPaths, error) {
	values, _ := cmd.Flags().GetStringArray("prompt")
	if len(values) == 0 {
		return nil, nil
	}
	prompts := make(config.PromptPaths, len(values))
	for _, value := range values {
		name, path, ok := strings.Cut(value, "=")
		if !ok || name == "" || path == "" {
			return nil, fmt.Errorf("invalid --prompt %q, expected name=path", value)
		}
		abs, err := filepath.Abs(home.Long(path))
		if err != nil {
			return nil, fmt.Errorf("resolving --prompt %s: %w", name, err)
		}
		prompts[name] = abs
	}
	if err := config.ValidatePromptNames(prompts); err != nil {
		return nil, fmt.Errorf("invalid --prompt: %w", err)
	}
	return prompts, nil
}
