package cmd

import (
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCmd_SystemPromptFlag(t *testing.T) {
	flag := rootCmd.PersistentFlags().Lookup("sys-prompt")
	require.NotNil(t, flag)
	require.Equal(t, "p", flag.Shorthand)
	require.Equal(t, "", flag.DefValue)
}

func TestRunCmd_SystemPromptInheritedFlagLookup(t *testing.T) {
	t.Parallel()

	systemPromptPath := filepath.Join(t.TempDir(), "system.md")
	root := &cobra.Command{Use: "crush"}
	run := &cobra.Command{
		Use: "run [prompt...]",
		RunE: func(cmd *cobra.Command, args []string) error {
			got, err := cmd.Flags().GetString("sys-prompt")
			require.NoError(t, err)
			require.Equal(t, systemPromptPath, got)
			return nil
		},
	}
	root.PersistentFlags().StringP("sys-prompt", "p", "", "Use a custom system prompt file")
	root.AddCommand(run)
	root.SetArgs([]string{"--sys-prompt", systemPromptPath, "run", "prompt"})

	require.NoError(t, root.Execute())
}

func TestApplyWorkspaceOverrides_SystemPromptOverride(t *testing.T) {
	systemPromptPath := filepath.Join("testdata", "system.md")
	store := config.NewTestStore(&config.Config{
		Options: &config.Options{},
	})

	applyWorkspaceOverrides(store, false, systemPromptPath)

	require.Equal(t, systemPromptPath, store.Overrides().SystemPromptPath)
	require.Empty(t, store.Config().Options.SystemPromptPath)
}
