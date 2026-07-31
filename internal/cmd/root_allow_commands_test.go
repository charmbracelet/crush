package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func newAllowCommandsTestCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("allow-commands", nil, "")
	cmd.Flags().Bool("allow-all-commands", false, "")
	return cmd
}

func TestResolveAllowCommands_EnvTrimsWhitespaceAndDropsEmpties(t *testing.T) {
	t.Setenv("CRUSH_ALLOW_COMMANDS", "ssh, curl ,,scp ")
	t.Setenv("CRUSH_ALLOW_ALL_COMMANDS", "")

	allow, all := resolveAllowCommands(newAllowCommandsTestCmd())
	require.False(t, all)
	require.Equal(t, []string{"ssh", "curl", "scp"}, allow)
}

func TestResolveAllowCommands_FlagsTakePrecedenceOverEnv(t *testing.T) {
	t.Setenv("CRUSH_ALLOW_COMMANDS", "curl")
	t.Setenv("CRUSH_ALLOW_ALL_COMMANDS", "true")

	cmd := newAllowCommandsTestCmd()
	require.NoError(t, cmd.Flags().Set("allow-commands", "ssh"))

	allow, all := resolveAllowCommands(cmd)
	// Flag was provided, so env is ignored for allow-commands.
	require.Equal(t, []string{"ssh"}, allow)
	// allow-all-commands flag was not set, so it falls back to the env var.
	require.True(t, all)
}

func TestResolveAllowCommands_AllCommandsEnvVariants(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
	} {
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("CRUSH_ALLOW_ALL_COMMANDS", tc.val)
			t.Setenv("CRUSH_ALLOW_COMMANDS", "")
			_, all := resolveAllowCommands(newAllowCommandsTestCmd())
			require.Equal(t, tc.want, all)
		})
	}
}
