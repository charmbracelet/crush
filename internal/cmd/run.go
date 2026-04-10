package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/format"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Aliases: []string{"r"},
	Use:     "run [prompt...]",
	Short:   "Run a single non-interactive prompt",
	Long: `Run a single prompt in non-interactive mode and exit.
The prompt can be provided as arguments or piped from stdin.`,
	Example: `
# Run a simple prompt
crush run "Guess my 5 favorite Pokémon"

# Pipe input from stdin
curl https://charm.land | crush run "Summarize this website"

# Read from a file
crush run "What is this code doing?" <<< prrr.go

# Redirect output to a file
crush run "Generate a hot README for this project" > MY_HOT_README.md

# Run in quiet mode (hide the spinner)
crush run --quiet "Generate a README for this project"

# Run in verbose mode (show logs)
crush run --verbose "Generate a README for this project"

# Continue a previous session
crush run --session {session-id} "Follow up on your last response"

# Continue the most recent session
crush run --continue "Follow up on your last response"

  `,
	RunE: func(cmd *cobra.Command, args []string) error {
		var (
			quiet, _      = cmd.Flags().GetBool("quiet")
			verbose, _    = cmd.Flags().GetBool("verbose")
			largeModel, _ = cmd.Flags().GetString("model")
			smallModel, _ = cmd.Flags().GetString("small-model")
			sessionID, _  = cmd.Flags().GetString("session")
			useLast, _    = cmd.Flags().GetBool("continue")
		)

		// Cancel on SIGINT or SIGTERM.
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
		defer cancel()

		prompt := strings.Join(args, " ")

		prompt, err := MaybePrependStdin(prompt)
		if err != nil {
			slog.Error("Failed to read from stdin", "error", err)
			return err
		}

		if prompt == "" {
			return fmt.Errorf("no prompt provided")
		}

		event.SetNonInteractive(true)

		switch {
		case sessionID != "":
			event.SetContinueBySessionID(true)
		case useLast:
			event.SetContinueLastSession(true)
		}

		if useClientServer() {
			c, ws, cleanup, err := connectToServer(cmd)
			if err != nil {
				return err
			}
			defer cleanup()

			event.AppInitialized()

			if sessionID != "" {
				sess, err := resolveSessionByID(ctx, c, ws.ID, sessionID)
				if err != nil {
					return err
				}
				sessionID = sess.ID
			}

			if !ws.Config.IsConfigured() {
				return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
			}

			if verbose {
				slog.SetDefault(slog.New(log.New(os.Stderr)))
			}

			return runNonInteractive(ctx, c, ws, prompt, largeModel, smallModel, quiet || verbose, sessionID, useLast)
		}

		ws, cleanup, err := setupLocalWorkspace(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		event.AppInitialized()

		if !ws.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'crush' to set up a provider interactively")
		}

		if verbose {
			slog.SetDefault(slog.New(log.New(os.Stderr)))
		}

		appWs := ws.(*workspace.AppWorkspace)
		return appWs.App().RunNonInteractive(ctx, os.Stdout, prompt, largeModel, smallModel, quiet || verbose, sessionID, useLast)
	},
}

func init() {
	runCmd.Flags().BoolP("quiet", "q", false, "Hide spinner")
	runCmd.Flags().BoolP("verbose", "v", false, "Show logs")
	runCmd.Flags().StringP("model", "m", "", "Model to use. Accepts 'model' or 'provider/model' to disambiguate models with the same name across providers")
	runCmd.Flags().String("small-model", "", "Small model to use. If not provided, uses the default small model for the provider")
	runCmd.Flags().StringP("session", "s", "", "Continue a previous session by ID")
	runCmd.Flags().BoolP("continue", "C", false, "Continue the most recent session")
	runCmd.MarkFlagsMutuallyExclusive("session", "continue")
}

// runNonInteractive executes the agent via the server and streams output
// to stdout.
func runNonInteractive(
	ctx context.Context,
	c *client.Client,
	ws *proto.Workspace,
	prompt, largeModel, smallModel string,
	hideSpinner bool,
	continueSessionID string,
	useLast bool,
) error {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if largeModel != "" || smallModel != "" {
		if err := overrideModels(ctx, c, ws, largeModel, smallModel); err != nil {
			return fmt.Errorf("failed to override models: %w", err)
		}
	}

	var (
		spinner   *format.Spinner
		stdoutTTY bool
		stderrTTY bool
		stdinTTY  bool
		progress  bool
	)

	stdoutTTY = term.IsTerminal(os.Stdout.Fd())
	stderrTTY = term.IsTerminal(os.Stderr.Fd())
	stdinTTY = term.IsTerminal(os.Stdin.Fd())
	progress = ws.Config.Options.Progress == nil || *ws.Config.Options.Progress

	if !hideSpinner && stderrTTY {
		t := common.StylesFromConfig(ws.Config)

		hasDarkBG := true
		if stdinTTY && stdoutTTY {
			hasDarkBG = lipgloss.HasDarkBackground(os.Stdin, os.Stdout)
		}
		defaultFG := lipgloss.LightDark(hasDarkBG)(charmtone.Pepper, t.FgBase)

		spinner = format.NewSpinner(ctx, cancel, anim.Settings{
			Size:        10,
			Label:       "Generating",
			LabelColor:  defaultFG,
			GradColorA:  t.Primary,
			GradColorB:  t.Secondary,
			CycleColors: true,
		})
		spinner.Start()
	}

	stopSpinner := func() {
		if !hideSpinner && spinner != nil {
			spinner.Stop()
			spinner = nil
		}
	}

	// Wait for the agent to become ready (MCP init, etc).
	if err := waitForAgent(ctx, c, ws.ID); err != nil {
		stopSpinner()
		return fmt.Errorf("agent not ready: %w", err)
	}

	// Force-update agent models so MCP tools are loaded.
	if err := c.UpdateAgent(ctx, ws.ID); err != nil {
		slog.Warn("Failed to update agent", "error", err)
	}

	defer stopSpinner()

	sess, err := resolveSession(ctx, c, ws.ID, continueSessionID, useLast)
	if err != nil {
		return fmt.Errorf("failed to resolve session: %w", err)
	}
	if continueSessionID != "" || useLast {
		slog.Info("Continuing session for non-interactive run", "session_id", sess.ID)
	} else {
		slog.Info("Created session for non-interactive run", "session_id", sess.ID)
	}

	events, err := c.SubscribeEvents(ctx, ws.ID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	if err := c.SendMessage(ctx, ws.ID, sess.ID, prompt); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	messageReadBytes := make(map[string]int)
	var printed bool

	defer func() {
		if progress && stderrTTY {
			_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
		}
		_, _ = fmt.Fprintln(os.Stdout)
	}()

	for {
		if progress && stderrTTY {
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
		}

		select {
		case ev, ok := <-events:
			if !ok {
				stopSpinner()
				return nil
			}

			switch e := ev.(type) {
			case pubsub.Event[proto.Message]:
				msg := e.Payload
				if msg.SessionID != sess.ID || msg.Role != proto.Assistant || len(msg.Parts) == 0 {
					continue
				}
				stopSpinner()

				content := msg.Content().String()
				readBytes := messageReadBytes[msg.ID]

				if len(content) < readBytes {
					slog.Error("Non-interactive: message content shorter than read bytes",
						"message_length", len(content), "read_bytes", readBytes)
					return fmt.Errorf("message content is shorter than read bytes: %d < %d", len(content), readBytes)
				}

				part := content[readBytes:]
				if readBytes == 0 {
					part = strings.TrimLeft(part, " \t")
				}
				if printed || strings.TrimSpace(part) != "" {
					printed = true
					fmt.Fprint(os.Stdout, part)
				}
				messageReadBytes[msg.ID] = len(content)

				if msg.IsFinished() {
					return nil
				}

			case pubsub.Event[proto.AgentEvent]:
				if e.Payload.Error != nil {
					stopSpinner()
					return fmt.Errorf("agent error: %w", e.Payload.Error)
				}
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}

// waitForAgent polls GetAgentInfo until the agent is ready, with a
// timeout.
func waitForAgent(ctx context.Context, c *client.Client, wsID string) error {
	timeout := time.After(30 * time.Second)
	for {
		info, err := c.GetAgentInfo(ctx, wsID)
		if err == nil && info.IsReady {
			return nil
		}
		select {
		case <-timeout:
			if err != nil {
				return fmt.Errorf("timeout waiting for agent: %w", err)
			}
			return fmt.Errorf("timeout waiting for agent readiness")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// overrideModels resolves model strings and updates the workspace
// configuration via the server.
func overrideModels(
	ctx context.Context,
	c *client.Client,
	ws *proto.Workspace,
	largeModel, smallModel string,
) error {
	cfg, err := c.GetConfig(ctx, ws.ID)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	providers := cfg.Providers.Copy()

	largeMatches, smallMatches := findModelMatches(providers, largeModel, smallModel)

	var largeProviderID string

	if largeModel != "" {
		found, err := validateModelMatches(largeMatches, largeModel, "large")
		if err != nil {
			return err
		}
		largeProviderID = found.provider
		slog.Info("Overriding large model", "provider", found.provider, "model", found.modelID)
		if err := c.UpdatePreferredModel(ctx, ws.ID, config.ScopeWorkspace, config.SelectedModelTypeLarge, config.SelectedModel{
			Provider: found.provider,
			Model:    found.modelID,
		}); err != nil {
			return fmt.Errorf("failed to set large model: %w", err)
		}
	}

	switch {
	case smallModel != "":
		found, err := validateModelMatches(smallMatches, smallModel, "small")
		if err != nil {
			return err
		}
		slog.Info("Overriding small model", "provider", found.provider, "model", found.modelID)
		if err := c.UpdatePreferredModel(ctx, ws.ID, config.ScopeWorkspace, config.SelectedModelTypeSmall, config.SelectedModel{
			Provider: found.provider,
			Model:    found.modelID,
		}); err != nil {
			return fmt.Errorf("failed to set small model: %w", err)
		}

	case largeModel != "":
		sm, err := c.GetDefaultSmallModel(ctx, ws.ID, largeProviderID)
		if err != nil {
			slog.Warn("Failed to get default small model", "error", err)
		} else if sm != nil {
			if err := c.UpdatePreferredModel(ctx, ws.ID, config.ScopeWorkspace, config.SelectedModelTypeSmall, *sm); err != nil {
				return fmt.Errorf("failed to set small model: %w", err)
			}
		}
	}

	return c.UpdateAgent(ctx, ws.ID)
}

type modelMatch struct {
	provider string
	modelID  string
}

// findModelMatches searches providers for matching large/small model
// strings.
func findModelMatches(providers map[string]config.ProviderConfig, largeModel, smallModel string) ([]modelMatch, []modelMatch) {
	largeFilter, largeID := parseModelString(largeModel)
	smallFilter, smallID := parseModelString(smallModel)

	var largeMatches, smallMatches []modelMatch
	for name, provider := range providers {
		if provider.Disable {
			continue
		}
		for _, m := range provider.Models {
			if matchesModel(largeID, largeFilter, m.ID, name) {
				largeMatches = append(largeMatches, modelMatch{provider: name, modelID: m.ID})
			}
			if matchesModel(smallID, smallFilter, m.ID, name) {
				smallMatches = append(smallMatches, modelMatch{provider: name, modelID: m.ID})
			}
		}
	}
	return largeMatches, smallMatches
}

// parseModelString splits "provider/model" into (provider, model) or
// ("", model).
func parseModelString(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	if idx := strings.Index(s, "/"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return "", s
}

// matchesModel returns true if the model ID matches the filter
// criteria.
func matchesModel(wantID, wantProvider, modelID, providerName string) bool {
	if wantID == "" {
		return false
	}
	if wantProvider != "" && wantProvider != providerName {
		return false
	}
	return strings.EqualFold(modelID, wantID)
}

// validateModelMatches ensures exactly one match exists.
func validateModelMatches(matches []modelMatch, modelID, label string) (modelMatch, error) {
	switch {
	case len(matches) == 0:
		return modelMatch{}, fmt.Errorf("%s model %q not found", label, modelID)
	case len(matches) > 1:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.provider
		}
		return modelMatch{}, fmt.Errorf(
			"%s model: model %q found in multiple providers: %s. Please specify provider using 'provider/model' format",
			label, modelID, strings.Join(names, ", "),
		)
	}
	return matches[0], nil
}

// resolveSession returns the session to use for a non-interactive run.
// If continueSessionID is set it fetches that session; if useLast is set it
// returns the most recently updated top-level session; otherwise it creates a
// new one.
func resolveSession(ctx context.Context, c *client.Client, wsID, continueSessionID string, useLast bool) (*proto.Session, error) {
	switch {
	case continueSessionID != "":
		sess, err := c.GetSession(ctx, wsID, continueSessionID)
		if err != nil {
			return nil, fmt.Errorf("session not found: %s", continueSessionID)
		}
		if sess.ParentSessionID != "" {
			return nil, fmt.Errorf("cannot continue a child session: %s", continueSessionID)
		}
		return sess, nil

	case useLast:
		sessions, err := c.ListSessions(ctx, wsID)
		if err != nil || len(sessions) == 0 {
			return nil, fmt.Errorf("no sessions found to continue")
		}
		last := sessions[0]
		for _, s := range sessions[1:] {
			if s.UpdatedAt > last.UpdatedAt && s.ParentSessionID == "" {
				last = s
			}
		}
		return &last, nil

	default:
		return c.CreateSession(ctx, wsID, "non-interactive")
	}
}

// resolveSessionByID resolves a session ID that may be a full UUID or a hash
// prefix returned by crush session list.
func resolveSessionByID(ctx context.Context, c *client.Client, wsID, id string) (*proto.Session, error) {
	if sess, err := c.GetSession(ctx, wsID, id); err == nil {
		return sess, nil
	}

	sessions, err := c.ListSessions(ctx, wsID)
	if err != nil {
		return nil, err
	}

	var matches []proto.Session
	for _, s := range sessions {
		hash := session.HashID(s.ID)
		if hash == id || strings.HasPrefix(hash, id) {
			matches = append(matches, s)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("session %q not found", id)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("session ID %q is ambiguous (%d matches)", id, len(matches))
	}
}
