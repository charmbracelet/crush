package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootCmd_SystemPromptFlag(t *testing.T) {
	t.Parallel()

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

func TestSetupLocalWorkspace_SystemPromptOverride(t *testing.T) {
	workingDir := t.TempDir()
	systemPromptPath := filepath.Join(workingDir, "system.md")
	writeLocalWorkspaceConfig(t, workingDir)
	isolateCmdTestEnvironment(t)

	cmd := newWorkspaceTestCommand(workingDir)
	require.NoError(t, cmd.Flags().Set("sys-prompt", systemPromptPath))

	ws, cleanup, err := setupLocalWorkspace(cmd)
	require.NoError(t, err)
	defer cleanup()

	storeProvider, ok := ws.(interface{ Store() *config.ConfigStore })
	require.True(t, ok)
	require.Equal(t, systemPromptPath, storeProvider.Store().Overrides().SystemPromptPath)
	require.Empty(t, storeProvider.Store().Config().Options.SystemPromptPath)
}

func newWorkspaceTestCommand(workingDir string) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("cwd", "c", workingDir, "")
	cmd.Flags().StringP("data-dir", "D", filepath.Join(workingDir, ".crush"), "")
	cmd.Flags().BoolP("debug", "d", false, "")
	cmd.Flags().BoolP("yolo", "y", false, "")
	cmd.Flags().StringP("sys-prompt", "p", "", "")
	return cmd
}

func writeLocalWorkspaceConfig(t *testing.T, workingDir string) {
	t.Helper()

	content := `{"options":{"disable_metrics":true,"context_paths":[],"disabled_skills":["crush-config","crush-hooks","jq"]}}`
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "crush.json"), []byte(content), 0o644))
}

func isolateCmdTestEnvironment(t *testing.T) {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(homeDir, ".cache"))
	t.Setenv("CRUSH_SKILLS_DIR", t.TempDir())
	t.Setenv("CRUSH_DISABLE_PROVIDER_AUTO_UPDATE", "1")
	t.Setenv("CRUSH_DISABLE_METRICS", "1")
}
