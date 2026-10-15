package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"charm.land/log/v2"
	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/format"
	"github.com/charmbracelet/crush/internal/herdr"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/google/uuid"
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
agent-cli run "Guess my 5 favorite Pokémon"

# Pipe input from stdin
curl https://charm.land | agent-cli run "Summarize this website"

# Read from a file
agent-cli run "What is this code doing?" <<< prrr.go

# Redirect output to a file
agent-cli run "Generate a hot README for this project" > MY_HOT_README.md

# Run in quiet mode (hide the spinner)
agent-cli run --quiet "Generate a README for this project"

# Run in verbose mode (show logs)
agent-cli run --verbose "Generate a README for this project"

# Use a specific reasoning effort
# Levels depend on the model, unsupported values are rejected
# with the accepted values listed
agent-cli run --reasoning-effort high "What is the meaning of life?"

# Continue a previous session
agent-cli run --session {session-id} "Follow up on your last response"

# Continue the most recent session
agent-cli run --continue "Follow up on your last response"

# Machine-readable result: one JSON object on stdout with session_id,
# text, error, prompt_tokens, completion_tokens and cost
agent-cli run --json "Summarize this repo"

# run auto-approves every tool permission request of its own session, both
# locally and via the server. It has no question tool: nobody could answer.
# It uses the project's current agent mode: if a TUI switched the project to
# plan mode, run can only read and plan.
#
# Ctrl+C, SIGTERM or closing the terminal cancels this run's own turn, also
# with --session/--continue (other turns and queued prompts of the session
# are left alone). If run is killed outright, the server cancels its turn
# once the client is gone (after a short reconnect grace).
#
# The fields are read from the session after the run: cost is the session's
# running total (so with --session/--continue it includes earlier runs);
# prompt_tokens and completion_tokens are the latest step's usage, and on a
# fresh session may also include title generation.
#
# Exit codes: 0 success, 1 runtime error (--json still prints the object
# with "error" set, also when the turn was cancelled from elsewhere), 2 bad
# usage (no prompt, bad flags).

  `,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			quiet, _           = cmd.Flags().GetBool("quiet")
			verbose, _         = cmd.Flags().GetBool("verbose")
			largeModel, _      = cmd.Flags().GetString("model")
			smallModel, _      = cmd.Flags().GetString("small-model")
			reasoningEffort, _ = cmd.Flags().GetString("reasoning-effort")
			sessionID, _       = cmd.Flags().GetString("session")
			useLast, _         = cmd.Flags().GetBool("continue")
			jsonOut, _         = cmd.Flags().GetBool("json")
		)

		// JSON mode owns stdout: no spinner, and no --verbose logs.
		if jsonOut {
			quiet, verbose = true, false
		}

		// Failures before the run starts (config, server, bad --session)
		// must still honour "exit 1 prints one JSON object".
		var jsonPrinted bool
		defer func() {
			var ue *usageError
			if jsonOut && !jsonPrinted && err != nil && !errors.As(err, &ue) {
				_ = json.NewEncoder(os.Stdout).Encode(runJSONResult{Error: err.Error()})
			}
		}()
		runOrJSON := func(
			ctx context.Context,
			asJSON bool,
			usage func(ctx context.Context, sessionID string) (sessionUsage, error),
			run func(out io.Writer, onSession func(string)) error,
		) error {
			jsonPrinted = asJSON
			return runMaybeJSON(ctx, asJSON, usage, run)
		}

		// Cancel on SIGINT, SIGTERM or SIGHUP (terminal đóng, SSH rớt): không
		// bắt SIGHUP thì tiến trình chết ngay và run phía server sống tiếp
		// không ai giám sát. Dưới nohup thì không bắt SIGHUP (stopSignals).
		ctx, cancel := signal.NotifyContext(context.Background(), stopSignals()...)
		defer cancel()

		prompt := strings.Join(args, " ")

		prompt, err = MaybePrependStdin(prompt)
		if err != nil {
			slog.Error("Failed to read from stdin", "error", err)
			return err
		}

		if prompt == "" {
			return &usageError{errors.New("no prompt provided")}
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

			if !ws.Config.IsConfigured() {
				return fmt.Errorf("no providers configured - please run 'agent-cli' to set up a provider interactively")
			}

			// Re-initialising a ready agent replaces the server's
			// coordinator under any open TUI, so only start one if missing.
			if err := ensureAgent(ctx, c, ws.ID); err != nil {
				return err
			}

			if sessionID != "" {
				sess, err := resolveSessionByID(ctx, c, ws.ID, sessionID)
				if err != nil {
					return err
				}
				sessionID = sess.ID
			}

			if verbose {
				slog.SetDefault(slog.New(log.New(os.Stderr)))
			}

			return runOrJSON(ctx, jsonOut,
				func(ctx context.Context, id string) (sessionUsage, error) {
					sess, err := c.GetSession(ctx, ws.ID, id)
					if err != nil {
						return sessionUsage{}, err
					}
					return sessionUsage{sess.PromptTokens, sess.CompletionTokens, sess.Cost}, nil
				},
				func(out io.Writer, onSession func(string)) error {
					return runNonInteractive(ctx, c, ws, out, onSession, prompt, largeModel, smallModel, reasoningEffort, quiet || verbose, sessionID, useLast)
				})
		}

		ws, cleanup, err := setupLocalWorkspace(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		event.AppInitialized()

		if !ws.Config().IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'agent-cli' to set up a provider interactively")
		}

		if verbose {
			slog.SetDefault(slog.New(log.New(os.Stderr)))
		}

		appWs := ws.(*workspace.AppWorkspace)

		if sessionID != "" {
			sess, err := resolveSessionID(ctx, appWs.App().Sessions, sessionID)
			if err != nil {
				return err
			}
			sessionID = sess.ID
		}

		return runOrJSON(ctx, jsonOut,
			func(ctx context.Context, id string) (sessionUsage, error) {
				sess, err := appWs.App().Sessions.Get(ctx, id)
				if err != nil {
					return sessionUsage{}, err
				}
				return sessionUsage{sess.PromptTokens, sess.CompletionTokens, sess.Cost}, nil
			},
			func(out io.Writer, onSession func(string)) error {
				return appWs.App().RunNonInteractive(ctx, out, onSession, prompt, largeModel, smallModel, reasoningEffort, quiet || verbose, sessionID, useLast)
			})
	},
}

func init() {
	runCmd.Flags().BoolP("quiet", "q", false, "Hide spinner")
	runCmd.Flags().BoolP("verbose", "v", false, "Show logs")
	runCmd.Flags().StringP("model", "m", "", "Model to use. Accepts 'model' or 'provider/model' to disambiguate models with the same name across providers")
	runCmd.Flags().String("small-model", "", "Small model to use. If not provided, uses the default small model for the provider")
	runCmd.Flags().String("reasoning-effort", "", "Reasoning effort for the model (e.g. low, medium, high). Levels depend on the model; unsupported values are rejected with the accepted values listed")
	runCmd.Flags().StringP("session", "s", "", "Continue a previous session by ID")
	runCmd.Flags().BoolP("continue", "C", false, "Continue the most recent session")
	runCmd.Flags().Bool("json", false, "Print one JSON object (session_id, text, error, token usage) to stdout instead of streaming text")
	runCmd.MarkFlagsMutuallyExclusive("session", "continue")
	runCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err}
	})
}

// usageError marks a bad invocation (missing prompt, bad flags); Execute
// turns it into exit code 2 instead of the generic 1.
type usageError struct{ error }

// runJSONResult is the single object `crush run --json` prints to stdout.
type runJSONResult struct {
	SessionID        string  `json:"session_id"`
	Text             string  `json:"text"`
	Error            string  `json:"error,omitempty"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	Cost             float64 `json:"cost"`
}

type sessionUsage struct {
	promptTokens, completionTokens int64
	cost                           float64
}

// runMaybeJSON runs the prompt, streaming to stdout, or in JSON mode
// capturing it into a single JSON object.
func runMaybeJSON(
	ctx context.Context,
	asJSON bool,
	usage func(ctx context.Context, sessionID string) (sessionUsage, error),
	run func(out io.Writer, onSession func(string)) error,
) error {
	if !asJSON {
		return run(os.Stdout, nil)
	}
	return runJSON(ctx, os.Stdout, usage, run)
}

// runJSON captures run's output and the session ID it reports through
// onSession (never guessed from "latest session": concurrent processes
// share a project), reads the usage of exactly that session afterwards and
// writes one JSON object to w. The run error is returned after printing so
// the process still exits 1.
func runJSON(
	ctx context.Context,
	w io.Writer,
	usage func(ctx context.Context, sessionID string) (sessionUsage, error),
	run func(out io.Writer, onSession func(string)) error,
) error {
	var (
		buf bytes.Buffer
		res runJSONResult
	)
	runErr := run(&buf, func(id string) { res.SessionID = id })
	// The local path swallows a cancelled run as success; a JSON consumer
	// still needs to see that it was cut short.
	if runErr == nil {
		runErr = ctx.Err()
	}
	// Both run paths end their output with one extra newline.
	res.Text = strings.TrimSuffix(buf.String(), "\n")
	err := runErr
	if res.SessionID != "" {
		// A cancelled ctx must not stop us reporting what was spent, but a
		// hung server must not hang the exit either.
		usageCtx, cancelUsage := context.WithTimeout(context.WithoutCancel(ctx), cancelRunTimeout)
		u, uerr := usage(usageCtx, res.SessionID)
		cancelUsage()
		if uerr != nil && err == nil {
			err = fmt.Errorf("failed to read session usage: %w", uerr)
		}
		res.PromptTokens, res.CompletionTokens, res.Cost = u.promptTokens, u.completionTokens, u.cost
	}
	if err != nil {
		res.Error = err.Error()
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if encErr := enc.Encode(res); encErr != nil && err == nil {
		err = encErr
	}
	return err
}

// cancelRunTimeout bounds the cancel request sent for an interrupted or
// blocked run, so a hung server cannot hang Ctrl+C. runStderr is where the
// timeout warning goes. Both are variables so tests can swap them.
var (
	cancelRunTimeout           = 5 * time.Second
	runStderr        io.Writer = os.Stderr
)

// runNonInteractive executes the agent via the server and streams output
// to out. onSession, when non-nil, receives the ID of the session used or
// created, as soon as it is resolved.
func runNonInteractive(
	ctx context.Context,
	c *client.Client,
	ws *proto.Workspace,
	out io.Writer,
	onSession func(sessionID string),
	prompt, largeModel, smallModel, reasoningEffort string,
	hideSpinner bool,
	continueSessionID string,
	useLast bool,
) (err error) {
	slog.Info("Running in non-interactive mode")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if largeModel != "" || smallModel != "" {
		if err := overrideModels(ctx, c, ws, largeModel, smallModel); err != nil {
			return fmt.Errorf("failed to override models: %w", err)
		}
	}

	// The reasoning effort applies to the model that will actually run.
	// On a continued session without an explicit model override, the
	// model is resolved later from the session's last assistant message,
	// so the override is applied after that restore instead.
	deferredEffort := (continueSessionID != "" || useLast) && largeModel == "" && smallModel == ""
	if reasoningEffort != "" && !deferredEffort {
		if err := overrideReasoningEffort(ctx, c, ws.ID, reasoningEffort); err != nil {
			return err
		}
	}

	var (
		spinner   *format.Spinner
		stderrTTY bool
		progress  bool
	)

	stderrTTY = term.IsTerminal(os.Stderr.Fd())
	progress = ws.Config.Options.Progress == nil || *ws.Config.Options.Progress

	if !hideSpinner && stderrTTY {
		t := common.ThemeStylesFromConfig(ws.Config)

		spinner = format.NewSpinner(ctx, cancel, anim.Settings{
			Size:        10,
			Label:       "Generating",
			GradColorA:  t.WorkingGradFromColor,
			GradColorB:  t.WorkingGradToColor,
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
	if onSession != nil {
		onSession(sess.ID)
	}
	if continueSessionID != "" || useLast {
		slog.Info("Continuing session for non-interactive run", "session_id", sess.ID)
		// If no explicit model override was requested, restore the
		// model/provider from the last assistant message in the
		// session, provided it is still available.
		if largeModel == "" && smallModel == "" {
			if err := restoreModelFromSession(ctx, c, ws, sess.ID); err != nil {
				slog.Warn("Failed to restore model from session", "error", err)
			}
		}
	} else {
		slog.Info("Created session for non-interactive run", "session_id", sess.ID)
	}

	if reasoningEffort != "" && deferredEffort {
		if err := overrideReasoningEffort(ctx, c, ws.ID, reasoningEffort); err != nil {
			return err
		}
	}

	events, err := c.SubscribeEvents(ctx, ws.ID)
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Mint a per-call RunID so we can correlate the terminal
	// RunComplete with *this* SendMessage even if the session was
	// busy and another turn finished first. Without it the stream
	// loop would exit on whichever RunComplete arrived first for
	// the same session and drop the queued prompt's output.
	runID := uuid.New().String()
	// The server keeps a run going after its client leaves, so Ctrl+C
	// (SIGTERM, SIGHUP) cancels this run's own turn there, by RunID: on a
	// continued session (-s/-C) the other turns and queued prompts are left
	// alone, and a cancel that overtakes the SendMessage still in flight
	// cancels the prompt when it arrives.
	ownSession := continueSessionID == "" && !useLast
	defer func() {
		if ctx.Err() == nil {
			return
		}
		if cerr := cancelOwnRun(ctx, c, ws.ID, sess.ID, runID, ownSession); cerr != nil {
			err = cerr
		}
	}()
	// run là lệnh đồng bộ: lượt của nó gắn vào đời của client này, nên run
	// chết bất thường (kill -9, OOM) thì server huỷ lượt thay vì để nó chạy
	// tiếp không ai giám sát. send thì cố ý không gắn.
	if err := c.SendMessage(client.WithNonInteractiveRun(client.WithRunBoundToClient(ctx)), ws.ID, sess.ID, runID, "", prompt); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	stream := &runStream{
		sessionID: sess.ID,
		runID:     runID,
		out:       out,
		read:      make(map[string]int),
		rootOf: func(id string) (string, error) {
			return session.RootID(id, func(id string) (string, error) {
				sess, err := c.GetSession(ctx, ws.ID, id)
				if err != nil {
					return "", err
				}
				return sess.ParentSessionID, nil
			})
		},
		selfCancelled: func() bool { return ctx.Err() != nil },
		grant: func(req proto.PermissionRequest) error {
			_, err := c.GrantPermission(ctx, ws.ID, proto.PermissionGrant{Permission: req, Action: proto.PermissionAllow})
			return err
		},
	}
	// On a continued session our prompt may be queued behind another turn
	// (e.g. the TUI's), whose prompts are not ours to approve or fail on.
	if continueSessionID != "" || useLast {
		stream.queued = promptQueued(ctx, c, ws.ID, sess.ID, prompt)
	}

	// Start herdr integration when running inside a herdr pane.
	hc := herdr.Init()
	hc.SetSessionID(sess.ID)
	defer hc.Close()

	defer func() {
		if progress && stderrTTY {
			_, _ = fmt.Fprintf(os.Stderr, ansi.ResetProgressBar)
		}
		_, _ = fmt.Fprintln(out)
	}()

	for {
		if progress && stderrTTY {
			_, _ = fmt.Fprintf(os.Stderr, ansi.SetIndeterminateProgressBar)
		}

		select {
		case ev, ok := <-events:
			if !ok {
				stopSpinner()
				// The stream also closes when ctx is cancelled.
				if err := ctx.Err(); err != nil {
					return err
				}
				return errEventStreamClosed
			}

			// Forward events to herdr if running inside a herdr pane.
			if hev := herdr.Translate(ev); hev != nil {
				hc.HandleEvent(hev)
			}

			done, err := stream.handle(ev, stopSpinner)
			if err != nil {
				var (
					ne *needsInputError
					ae *abandonError
				)
				if errors.As(err, &ne) || errors.As(err, &ae) {
					// Nobody can unblock it, so don't leave the run blocked
					// on the server (it would also block the session's TUI).
					if cerr := cancelOwnRun(ctx, c, ws.ID, sess.ID, runID, ownSession); cerr != nil {
						return fmt.Errorf("%w; %w", err, cerr)
					}
				}
				return err
			}
			if done {
				return nil
			}

		case <-ctx.Done():
			stopSpinner()
			return ctx.Err()
		}
	}
}

// runCanceller là phần client mà cancelOwnRun cần.
type runCanceller interface {
	CancelAgentRun(ctx context.Context, id, sessionID, runID string) error
	CancelAgentSession(ctx context.Context, id, sessionID string) error
}

// cancelOwnRun huỷ lượt runID của run này trên server, trên ctx riêng (ctx có
// thể đã bị huỷ) có hạn cancelRunTimeout. Server cũ không huỷ được theo
// RunID: session do run tạo (ownSession) thì huỷ cả session, vì trên đó chỉ
// có lượt của run; session có sẵn (-s/-C) thì không huỷ được mà không đụng
// lượt của người khác, nên trả lỗi nói run vẫn chạy.
func cancelOwnRun(ctx context.Context, c runCanceller, wsID, sessionID, runID string, ownSession bool) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cancelRunTimeout)
	defer cancel()
	err := c.CancelAgentRun(ctx, wsID, sessionID, runID)
	if errors.Is(err, client.ErrCancelRunUnsupported) {
		if !ownSession {
			return fmt.Errorf("stopped waiting; the run keeps going on session %s (this server cannot cancel a single run)", sessionID)
		}
		err = c.CancelAgentSession(ctx, wsID, sessionID)
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		fmt.Fprintf(runStderr, "cancel request timed out; the run may still be going on session %s\n", sessionID)
	case err != nil:
		slog.Warn("Failed to cancel run", "session_id", sessionID, "error", err)
	}
	return nil
}

// runStream tracks the per-message stdout cursor and the
// reconciliation state used by [runNonInteractive] to translate
// streaming SSE events into a final, complete stdout for `crush run`.
// It is split out so the state machine can be exercised in unit tests
// without spinning up the full server/client harness.
//
// runID, when non-empty, is the authoritative correlator for the
// terminal RunComplete event: the stream suppresses live message
// events and only exits on a RunComplete whose RunID matches, so a
// turn that finishes first on the same session (e.g. when our prompt
// was queued behind a busy session) cannot contaminate stdout or
// terminate us prematurely. When empty (older servers, tests that
// don't supply one) the stream falls back to SessionID-only matching
// and live message streaming, which is still correct for the
// single-turn case.
type runStream struct {
	sessionID string
	runID     string
	out       io.Writer
	read      map[string]int
	printed   bool
	// rootOf resolves a (sub-agent) session to its top-level session.
	// Nil means only exact session matches count.
	rootOf func(sessionID string) (string, error)
	// selfCancelled reports whether this process cancelled the run itself
	// (Ctrl+C, SIGTERM, SIGHUP). A cancelled RunComplete is a clean exit only
	// then; cancelled from elsewhere (Esc in the TUI, server shutdown) the
	// run did not finish and must not look like it did.
	selfCancelled func() bool
	// grant approves a tool permission request of this run, like the local
	// path's AutoApproveSession. Nil means such requests fail the run.
	grant func(proto.PermissionRequest) error
	// queued reports whether this run's prompt still waits in the session
	// queue, so the active turn is someone else's. Nil means the active
	// turn is this run's (a session the run created).
	queued func() bool
}

// needsInputError reports that the run is blocked on a prompt a
// non-interactive run cannot answer.
type needsInputError struct{ what string }

func (e *needsInputError) Error() string {
	return e.what + ", but `run` cannot answer it non-interactively"
}

// abandonError: run không đi tiếp được và không ai gỡ được (duyệt quyền
// thất bại, không biết yêu cầu là của ai). Như needsInputError, run tự huỷ
// lượt của nó trên server thay vì để lượt treo ở đó.
type abandonError struct{ err error }

func (e *abandonError) Error() string { return e.err.Error() }
func (e *abandonError) Unwrap() error { return e.err }

// errRunCancelledElsewhere: lượt bị huỷ không phải do run này (Esc ở TUI,
// server tắt, prompt bị gỡ khỏi hàng đợi), nên chưa xong.
var errRunCancelledElsewhere = errors.New("run cancelled before it finished (by another client or the server)")

// ownsSession reports whether a prompt raised for sessionID belongs to
// this run. The run client never reports a current session, so the server
// delivers prompts of every session in the workspace to it. An error means
// ownership could not be resolved: guessing would either approve another
// session's tool or leave this run blocked forever.
func (s *runStream) ownsSession(sessionID string) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	if sessionID == s.sessionID {
		return true, nil
	}
	if s.rootOf == nil {
		return false, nil
	}
	root, err := s.rootOf(sessionID)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Run đang dừng (Ctrl+C, hết giờ): không phải lỗi tra cứu, vòng lặp
		// sẽ thoát theo ctx.
		return false, nil
	}
	if err != nil {
		return false, &abandonError{fmt.Errorf("cannot tell whether a request from session %s belongs to this run: %w", sessionID, err)}
	}
	return root == s.sessionID, nil
}

// promptQueued returns a check of whether prompt still waits in sessionID's
// queue, i.e. the session's active turn is someone else's (e.g. the TUI's).
// Unknown counts as queued: acting on another turn's prompt would approve
// its tool or cancel it.
func promptQueued(ctx context.Context, c sendClient, wsID, sessionID, prompt string) func() bool {
	return func() bool {
		list, err := c.GetAgentSessionQueuedPromptsList(ctx, wsID, sessionID)
		return err != nil || slices.Contains(list, prompt)
	}
}

// othersTurn reports whether a prompt raised in sessionID belongs to another
// turn because our prompt is still queued (nil queued: the turn is ours); the
// caller then leaves it alone, keeps waiting and says so on stderr.
// shortcut: requests carry no RunID, so ownership is read from the queue by
// prompt text: an identical prompt queued by someone else, or a foreign
// prompt in the instant before ours is enqueued, is misread. Upgrade: put the
// RunID on permission/question requests.
func othersTurn(queued func() bool, what, sessionID string) bool {
	if queued == nil || !queued() {
		return false
	}
	fmt.Fprintf(runStderr, "%s from another turn is pending on session %s; this run keeps waiting behind it\n", what, sessionID)
	return true
}

// handle processes one SSE event. Returns done=true when the run
// loop should exit (RunComplete observed); returns an error only
// when the agent run failed (not on context cancel — that path is
// handled by the caller's select). stopSpinner is called on the
// first observable assistant output and on completion; passing nil
// is safe for tests.
func (s *runStream) handle(ev any, stopSpinner func()) (done bool, err error) {
	stop := func() {
		if stopSpinner != nil {
			stopSpinner()
		}
	}
	switch e := ev.(type) {
	case pubsub.Event[proto.Message]:
		msg := e.Payload
		if msg.SessionID != s.sessionID || msg.Role != proto.Assistant || len(msg.Parts) == 0 {
			return false, nil
		}
		if s.runID != "" {
			return false, nil
		}
		stop()

		content := msg.Content().String()
		readBytes := s.read[msg.ID]
		if len(content) < readBytes {
			slog.Error("Non-interactive: message content shorter than read bytes",
				"message_length", len(content), "read_bytes", readBytes)
			return false, fmt.Errorf("message content is shorter than read bytes: %d < %d", len(content), readBytes)
		}

		part := content[readBytes:]
		if readBytes == 0 {
			part = strings.TrimLeft(part, " \t")
		}
		if s.printed || strings.TrimSpace(part) != "" {
			s.printed = true
			fmt.Fprint(s.out, part)
		}
		s.read[msg.ID] = len(content)
		return false, nil

	case pubsub.Event[proto.RunComplete]:
		// RunComplete is the authoritative end-of-run signal. We
		// exit on it instead of guessing from message finish parts,
		// which fire on every tool-call step too and were the
		// source of the regression where `crush run` exited
		// mid-turn on finish.reason == tool_use.
		//
		// Correlation:
		//   - if we minted a RunID for this SendMessage, only the
		//     event whose RunID matches is ours; any other turn
		//     finishing first on the same session (busy-session
		//     queue path) must be ignored.
		//   - if we have no RunID (older server, tests), fall back
		//     to SessionID matching.
		if s.runID != "" {
			if e.Payload.RunID != s.runID {
				return false, nil
			}
		} else if e.Payload.SessionID != s.sessionID {
			return false, nil
		}
		stop()
		if e.Payload.Error != "" && !e.Payload.Cancelled {
			return true, fmt.Errorf("agent run failed: %s", e.Payload.Error)
		}
		// Lượt bị huỷ mà không phải do chính run này: in phần đã có rồi
		// báo lỗi (dưới), để script không tưởng là xong.
		cancelledElsewhere := e.Payload.Cancelled && (s.selfCancelled == nil || !s.selfCancelled())
		// Reconcile stdout against the authoritative final
		// assistant text carried in the event. The pubsub fan-in
		// does not serialize publishes across upstream brokers, so
		// the final message event may not have reached this loop
		// yet; the embedded Text field is the backstop that
		// guarantees the full final text always appears on stdout.
		if e.Payload.MessageID != "" {
			full := e.Payload.Text
			readBytes := s.read[e.Payload.MessageID]
			if readBytes < len(full) {
				tail := full[readBytes:]
				if readBytes == 0 {
					tail = strings.TrimLeft(tail, " \t")
				}
				if s.printed || strings.TrimSpace(tail) != "" {
					s.printed = true
					fmt.Fprint(s.out, tail)
				}
			}
		}
		if cancelledElsewhere {
			return true, errRunCancelledElsewhere
		}
		return true, nil

	case pubsub.Event[proto.PermissionRequest]:
		owned, err := s.ownsSession(e.Payload.SessionID)
		if err != nil {
			stop()
			return true, err
		}
		if !owned || othersTurn(s.queued, "a permission request", s.sessionID) {
			return false, nil
		}
		// Same as the local path: a non-interactive run approves every
		// tool permission of its own session. Other sessions' and other
		// turns' prompts are never touched (see ownsSession, othersTurn).
		if s.grant == nil {
			stop()
			return true, &needsInputError{fmt.Sprintf("permission required for tool %q (%s)", e.Payload.ToolName, e.Payload.Action)}
		}
		if err := s.grant(e.Payload); err != nil {
			stop()
			return true, &abandonError{fmt.Errorf("failed to approve tool permission: %w", err)}
		}
		return false, nil

	case pubsub.Event[proto.QuestionRequest]:
		owned, err := s.ownsSession(e.Payload.SessionID)
		if err != nil {
			stop()
			return true, err
		}
		if !owned || othersTurn(s.queued, "a question", s.sessionID) {
			return false, nil
		}
		stop()
		return true, &needsInputError{"the agent asked a question"}

	case pubsub.Event[proto.AgentEvent]:
		if e.Payload.Error == nil {
			return false, nil
		}
		// Attribute the error to our run before treating it as
		// fatal. Async errors from an unrelated workspace run share
		// this channel, so a foreign failure must not abort us:
		//   - if the event carries a RunID, it is the authoritative
		//     correlator: it must match our run exactly, otherwise it
		//     belongs to a different request and we ignore it.
		//   - if the event carries no RunID (older server), fall back
		//     to SessionID: it must be present and match our session,
		//     otherwise we ignore it.
		if e.Payload.RunID != "" {
			if e.Payload.RunID != s.runID {
				return false, nil
			}
		} else if e.Payload.SessionID == "" || e.Payload.SessionID != s.sessionID {
			return false, nil
		}
		stop()
		return true, fmt.Errorf("agent error: %w", e.Payload.Error)
	}
	return false, nil
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

// restoreModelFromSession reads the last assistant message in the
// session and, if it used a different provider/model than the current
// config, updates the preferred model on the server provided the
// provider/model is still available. This ensures that continuing a
// session uses the same model that produced the last response.
func restoreModelFromSession(ctx context.Context, c *client.Client, ws *proto.Workspace, sessionID string) error {
	msgs, err := c.ListMessages(ctx, ws.ID, sessionID)
	if err != nil {
		return fmt.Errorf("failed to list messages: %w", err)
	}

	var lastAssistant *proto.Message
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == proto.Assistant && !msgs[i].IsSummaryMessage {
			lastAssistant = &msgs[i]
			break
		}
	}
	if lastAssistant == nil || lastAssistant.Provider == "" || lastAssistant.Model == "" {
		return nil
	}

	cfg := ws.Config
	currentLarge := cfg.Models[config.SelectedModelTypeLarge]
	if currentLarge.Provider == lastAssistant.Provider && currentLarge.Model == lastAssistant.Model {
		return nil
	}

	if !cfg.IsModelAvailable(lastAssistant.Provider, lastAssistant.Model) {
		slog.Debug("Skipping model restoration: provider/model not available",
			"provider", lastAssistant.Provider,
			"model", lastAssistant.Model)
		return nil
	}

	selectedModel := config.SelectedModel{
		Provider: lastAssistant.Provider,
		Model:    lastAssistant.Model,
	}
	if err := c.UpdatePreferredModel(ctx, ws.ID, config.ScopeWorkspace, config.SelectedModelTypeLarge, selectedModel); err != nil {
		return fmt.Errorf("failed to set large model: %w", err)
	}

	if _, ok := cfg.Models[config.SelectedModelTypeSmall]; !ok {
		sm, err := c.GetDefaultSmallModel(ctx, ws.ID, lastAssistant.Provider)
		if err != nil {
			slog.Warn("Failed to get default small model", "error", err)
		} else if sm != nil {
			if err := c.UpdatePreferredModel(ctx, ws.ID, config.ScopeWorkspace, config.SelectedModelTypeSmall, *sm); err != nil {
				slog.Warn("Failed to set small model during session restore", "error", err)
			}
		}
	}

	return c.UpdateAgent(ctx, ws.ID)
}

// overrideReasoningEffort validates the requested reasoning effort against
// the large model in effect for this run (which may have been overridden by
// --model or restored from a continued session) and applies it on the
// server.
func overrideReasoningEffort(ctx context.Context, c *client.Client, wsID, reasoningEffort string) error {
	cfg, err := c.GetConfig(ctx, wsID)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	selected, ok := cfg.Models[config.SelectedModelTypeLarge]
	if !ok {
		return fmt.Errorf("no large model selected; set one with the --model flag or 'model large'")
	}
	if err := cfg.ValidateReasoningEffort(selected.Provider, selected.Model, reasoningEffort); err != nil {
		return err
	}
	selected.ReasoningEffort = reasoningEffort
	slog.Info("Overriding reasoning effort for non-interactive run",
		"provider", selected.Provider,
		"model", selected.Model,
		"reasoning_effort", reasoningEffort)
	if err := c.UpdatePreferredModel(ctx, wsID, config.ScopeWorkspace, config.SelectedModelTypeLarge, selected); err != nil {
		return fmt.Errorf("failed to set reasoning effort: %w", err)
	}
	return c.UpdateAgent(ctx, wsID)
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

// sessionLookup is what resolveSessionByID needs from the client.
type sessionLookup interface {
	GetSession(ctx context.Context, id, sessionID string) (*proto.Session, error)
	ListSessions(ctx context.Context, id string) ([]proto.Session, error)
}

// resolveSessionByID resolves a session ID that may be a full UUID or a hash
// prefix returned by crush session list.
func resolveSessionByID(ctx context.Context, c sessionLookup, wsID, id string) (*proto.Session, error) {
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
