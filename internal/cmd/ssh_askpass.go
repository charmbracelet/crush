package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/crush/internal/sshaskpass"
	"github.com/spf13/cobra"
)

// sshAskpassCmd is the hidden subcommand that OpenSSH invokes as its
// SSH_ASKPASS program (via the wrapper script pointed to by the
// environment-scoped SSH_ASKPASS variable). It forwards the prompt to
// the host Crush process's prompt socket so the question is rendered by
// the TUI instead of a plain terminal, and prints the answer (a secret,
// or "yes" for confirmations) on stdout. No ssh configuration or files
// are modified.
var sshAskpassCmd = &cobra.Command{
	Use:    "__ssh-askpass",
	Short:  "Internal: ssh askpass for the integrated SSH prompts",
	Hidden: true,
	// Disable flag parsing: the prompt is passed as positional text.
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSSHAskpass(cmd.Context(), args)
	},
}

func init() {
	rootCmd.AddCommand(sshAskpassCmd)
}

// runSSHAskpass handles a single askpass invocation. OpenSSH execs this
// with the prompt text as its argument and expects the secret (or an
// answer such as "yes") on stdout.
func runSSHAskpass(ctx context.Context, args []string) error {
	prompt := strings.Join(args, " ")

	socketPath := os.Getenv("CRUSH_SSH_SOCK")
	if socketPath == "" {
		return errors.New("crush: no ssh prompt socket configured (askpass unavailable)")
	}

	kind, keyInfo := sshaskpass.ClassifyPrompt(prompt)
	secret, err := sshaskpass.AskCredentialOverSocket(ctx, socketPath, sshaskpass.PromptRequest{
		Prompt:  prompt,
		KeyInfo: keyInfo,
		Kind:    kind,
	})
	if err != nil {
		// OpenSSH aborts the authentication attempt when askpass exits
		// non-zero, which is the correct outcome for a cancelled or
		// unanswered prompt.
		return fmt.Errorf("crush: ssh askpass: %w", err)
	}

	fmt.Fprintln(os.Stdout, secret)
	return nil
}
