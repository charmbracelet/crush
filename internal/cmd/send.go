package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/crush/internal/client"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var sendCmd = &cobra.Command{
	Use:   "send [flags] <prompt...>",
	Short: "Send a prompt into a session of a project that is already running",
	Long: `Deliver a prompt to a session through the project's server, so it shows up
in an open TUI for that project like a normal message. Without --wait it
returns as soon as the server accepts the prompt; with --wait it blocks until
that run finishes and prints the final text.

Needs CRUSH_CLIENT_SERVER=1 (the agent-cli launcher sets it).
Exit codes: 0 success, 1 runtime error (--json still prints the object with
"error" set), 2 bad usage.`,
	Example: `
# Continue a session (full ID or the prefix shown by 'session list')
agent-cli send --session 3f2a "Now add tests"

# Start a new session in another project and wait for the answer
agent-cli send -c ~/code/web --new --title review --wait --json "Review the last commit"
`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var (
			sessionID, _ = cmd.Flags().GetString("session")
			isNew, _     = cmd.Flags().GetBool("new")
			title, _     = cmd.Flags().GetString("title")
			wait, _      = cmd.Flags().GetBool("wait")
			jsonOut, _   = cmd.Flags().GetBool("json")
			timeout, _   = cmd.Flags().GetDuration("timeout")
			prompt       = strings.Join(args, " ")
		)
		if err := sendUsage(prompt, sessionID, title, isNew, wait, timeout); err != nil {
			return &usageError{err}
		}
		if !useClientServer() {
			return &usageError{errors.New("send needs the project server: run it through the agent-cli launcher or set CRUSH_CLIENT_SERVER=1")}
		}

		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		var res runJSONResult
		defer func() {
			var ue *usageError
			if jsonOut && err != nil && !errors.As(err, &ue) {
				res.Error = err.Error()
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetEscapeHTML(false)
				_ = enc.Encode(sendJSON(res, wait))
			}
		}()

		c, ws, cleanup, err := connectToServer(cmd)
		if err != nil {
			return err
		}
		defer cleanup()
		if !ws.Config.IsConfigured() {
			return fmt.Errorf("no providers configured - please run 'agent-cli' to set up a provider interactively")
		}
		if err := ensureAgent(ctx, c, ws.ID); err != nil {
			return err
		}

		out := io.Writer(os.Stdout)
		if jsonOut {
			out = io.Discard
		}
		res, err = runSend(ctx, c, ws.ID, sendOptions{
			sessionID: sessionID, newSession: isNew, title: title,
			prompt: prompt, wait: wait, timeout: timeout,
		}, out)
		return err
	},
}

func init() {
	sendCmd.Flags().StringP("session", "s", "", "Session to send to: full ID or hash prefix")
	sendCmd.Flags().Bool("new", false, "Create a new session instead of using --session")
	sendCmd.Flags().String("title", "", "Title for the session created by --new")
	sendCmd.Flags().Bool("wait", false, "Wait until the run finishes and print the final text")
	sendCmd.Flags().Duration("timeout", 0, "With --wait, give up after this long (0 = no limit)")
	sendCmd.Flags().Bool("json", false, "Print one JSON object on stdout (session_id, plus text and usage with --wait)")
	sendCmd.MarkFlagsMutuallyExclusive("session", "new")
	sendCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err}
	})
	rootCmd.AddCommand(sendCmd)
}

// sendUsage rejects bad flag combinations (exit 2).
func sendUsage(prompt, sessionID, title string, isNew, wait bool, timeout time.Duration) error {
	switch {
	case prompt == "":
		return errors.New("no prompt provided")
	case sessionID == "" && !isNew:
		return errors.New("pass --session <id> or --new")
	case title != "" && !isNew:
		return errors.New("--title only applies with --new")
	case timeout != 0 && !wait:
		return errors.New("--timeout only applies with --wait")
	}
	return nil
}

// sendJSON picks the --json object: a plain accept is just the session ID,
// everything else (wait, or any error) is the run's runJSONResult shape.
func sendJSON(res runJSONResult, wait bool) any {
	if wait || res.Error != "" {
		return res
	}
	return struct {
		SessionID string `json:"session_id"`
	}{res.SessionID}
}

// agentInit is what ensureAgent needs from the client.
type agentInit interface {
	GetAgentInfo(ctx context.Context, id string) (*proto.AgentInfo, error)
	InitiateAgentProcessing(ctx context.Context, id string, interactive bool) error
}

// ensureAgent starts the coder agent only when the server has none. Calling
// init on a ready server REPLACES its coordinator, which would drop the open
// TUI's question tool and the busy state of runs in flight.
func ensureAgent(ctx context.Context, c agentInit, wsID string) error {
	if info, err := c.GetAgentInfo(ctx, wsID); err == nil && info.IsReady {
		return nil
	}
	if err := c.InitiateAgentProcessing(ctx, wsID, false); err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}
	return nil
}

// sendClient is the part of client.Client that send uses, so tests can fake it.
type sendClient interface {
	sessionLookup
	SendMessage(ctx context.Context, id, sessionID, runID, channel, prompt string, attachments ...message.Attachment) error
	SubscribeEvents(ctx context.Context, id string) (<-chan any, error)
	CreateSession(ctx context.Context, id, title string) (*proto.Session, error)
	GetAgentSessionQueuedPromptsList(ctx context.Context, id, sessionID string) ([]string, error)
}

var _ sendClient = (*client.Client)(nil)

type sendOptions struct {
	sessionID, title, prompt string
	newSession, wait         bool
	timeout                  time.Duration
}

// runSend delivers the prompt and, with wait, blocks for this run's
// RunComplete. The result carries whatever is known even when err != nil.
func runSend(ctx context.Context, c sendClient, wsID string, o sendOptions, out io.Writer) (res runJSONResult, err error) {
	var sess *proto.Session
	if o.newSession {
		title := o.title
		if title == "" {
			title = "send"
		}
		sess, err = c.CreateSession(ctx, wsID, title)
	} else {
		sess, err = resolveSessionByID(ctx, c, wsID, o.sessionID)
	}
	if err != nil {
		return res, err
	}
	res.SessionID = sess.ID

	if o.wait && o.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.timeout)
		defer cancel()
	}

	// Subscribe before sending so a fast run cannot complete unseen.
	var events <-chan any
	if o.wait {
		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		if events, err = c.SubscribeEvents(subCtx, wsID); err != nil {
			return res, fmt.Errorf("failed to subscribe to events: %w", err)
		}
	}

	// The RunID tells our turn apart from others finishing on the same session.
	runID := uuid.New().String()
	if err := c.SendMessage(ctx, wsID, sess.ID, runID, "", o.prompt); err != nil {
		return res, fmt.Errorf("failed to send message: %w", err)
	}
	if !o.wait {
		res = runJSONResult{SessionID: sess.ID}
		fmt.Fprintln(out, sess.ID)
		return res, nil
	}

	// On an existing session our prompt may be queued behind another turn
	// (e.g. the TUI's), whose prompts are not ours to fail on.
	var queued func() bool
	if !o.newSession {
		queued = promptQueued(ctx, c, wsID, sess.ID, o.prompt)
	}
	runErr := waitForRun(ctx, events, sess.ID, runID, queued, &res)
	// ctx, not runErr: the event stream closes with ctx and may be what
	// waitForRun saw first.
	switch {
	case runErr == nil:
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		runErr = fmt.Errorf("timed out after %s waiting for the run to finish", o.timeout)
	case ctx.Err() != nil:
		// Ctrl+C only stops waiting: send hands the work to the session,
		// and the server keeps the run going without this client.
		runErr = fmt.Errorf("stopped waiting; the run keeps going on session %s", sess.ID)
	}
	// A cancelled ctx must not stop us reporting what was spent.
	if s, uerr := c.GetSession(context.WithoutCancel(ctx), wsID, sess.ID); uerr == nil {
		res.PromptTokens, res.CompletionTokens, res.Cost = s.PromptTokens, s.CompletionTokens, s.Cost
	} else if runErr == nil {
		runErr = fmt.Errorf("failed to read session usage: %w", uerr)
	}
	if runErr == nil {
		fmt.Fprintln(out, res.Text)
	}
	return res, runErr
}

// errEventStreamClosed: the server ended the event stream (crash, restart)
// before the awaited run completed.
var errEventStreamClosed = errors.New("event stream closed before the run completed")

// waitForRun reads events until the RunComplete carrying runID for sessionID,
// storing its final text in res. queued is as in [othersTurn].
func waitForRun(ctx context.Context, events <-chan any, sessionID, runID string, queued func() bool, res *runJSONResult) error {
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return errEventStreamClosed
			}
			switch e := ev.(type) {
			case pubsub.Event[proto.RunComplete]:
				p := e.Payload
				if p.SessionID != sessionID || p.RunID != runID {
					continue
				}
				res.Text = p.Text
				switch {
				case p.Cancelled:
					return errors.New("run cancelled")
				case p.Error != "":
					return fmt.Errorf("agent run failed: %s", p.Error)
				}
				return nil
			// Nobody reports a current session for this run, so the server
			// hands its prompts to us; with no TUI on the session they would
			// block the run forever. shortcut: sub-agent prompts carry the
			// child session ID and are not matched; resolve via
			// GetSession().ParentSessionID if needed.
			case pubsub.Event[proto.PermissionRequest]:
				if e.Payload.SessionID == sessionID && !othersTurn(queued, "a permission request", sessionID) {
					return fmt.Errorf("run is waiting for permission (%s); approve it in the TUI on session %s", e.Payload.ToolName, sessionID)
				}
			case pubsub.Event[proto.QuestionRequest]:
				if e.Payload.SessionID == sessionID && !othersTurn(queued, "a question", sessionID) {
					return fmt.Errorf("run is waiting for an answer to a question; answer it in the TUI on session %s", sessionID)
				}
			case pubsub.Event[proto.AgentEvent]:
				if e.Payload.Error != nil && e.Payload.RunID == runID {
					return fmt.Errorf("agent error: %w", e.Payload.Error)
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
