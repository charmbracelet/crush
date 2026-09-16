package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/charmbracelet/crush/internal/pinentry"
	"github.com/spf13/cobra"
)

// pinentryGpgCmd is the hidden subcommand that git invokes as its gpg
// program (via the wrapper script pointed to by the environment-scoped
// gpg.program config). It runs the real gpg through loopback pinentry
// and collects the passphrase/PIN over the host Crush process's prompt
// socket, so the prompt is rendered by the TUI instead of an external
// terminal pinentry. No GPG or git configuration is modified.
var pinentryGpgCmd = &cobra.Command{
	Use:    "__pinentry-gpg",
	Short:  "Internal: gpg wrapper for the integrated pinentry",
	Hidden: true,
	// Disable flag parsing: every argument belongs to gpg.
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPinentryGpg(cmd.Context(), args)
	},
}

func init() {
	rootCmd.AddCommand(pinentryGpgCmd)
}

func runPinentryGpg(ctx context.Context, args []string) error {
	// Prepend the gpg argv so the runner sees a full command line.
	gpgArgs := append([]string{"gpg"}, args...)

	socketPath := os.Getenv("CRUSH_PINENTRY_SOCK")
	var runner *pinentry.Runner
	if socketPath != "" {
		runner = &pinentry.Runner{
			Prompter: pinentry.SocketPrompter(socketPath),
		}
	}

	if runner != nil {
		err := runner.Run(ctx, pinentry.RunOptions{
			Args:   gpgArgs,
			Env:    os.Environ(),
			Dir:    cwd(),
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		})
		if !errors.Is(err, pinentry.ErrFallback) {
			return exitWithStatus(err)
		}
		// Loopback pinentry is unavailable (old GPG, agent without
		// loopback support, no host listening): fall back to plain
		// gpg, which triggers the terminal-handover watcher.
	}

	return runPlainGpg(gpgArgs)
}

// runPlainGpg execs gpg as-is, preserving stdin/stdout/stderr and the
// exit status.
func runPlainGpg(args []string) error {
	bin, err := exec.LookPath(args[0])
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// exitWithStatus maps the runner's errors to process exit codes so git
// sees the same status gpg would have produced.
func exitWithStatus(err error) error {
	if err == nil {
		return nil
	}
	var ee *pinentry.ExitError
	switch {
	case errors.Is(err, pinentry.ErrCancelled):
		os.Exit(2)
	case errors.As(err, &ee):
		os.Exit(ee.Code)
	default:
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return nil
}

// cwd returns the current working directory or ".".
func cwd() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
