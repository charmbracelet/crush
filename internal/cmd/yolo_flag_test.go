package cmd

import "testing"

// TestYoloFlagReachesSubcommands guards --yolo being a persistent root flag.
// setupLocalWorkspace and the TUI read it with cmd.Flags().GetBool("yolo");
// if the flag were local to rootCmd, `crush run --yolo` and any other
// subcommand would reject it or silently run without YOLO.
func TestYoloFlagReachesSubcommands(t *testing.T) {
	if rootCmd.PersistentFlags().Lookup("yolo") == nil {
		t.Fatal("yolo must be a persistent flag on the root command")
	}
	for _, c := range rootCmd.Commands() {
		if c.InheritedFlags().Lookup("yolo") == nil {
			t.Errorf("subcommand %q does not inherit --yolo", c.Name())
		}
	}
}
