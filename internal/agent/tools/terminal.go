package tools

import (
	"cmp"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/terminal"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	TerminalToolName = "terminal"

	// TerminalMaxReadWait bounds how long a read may block waiting for a
	// session to exit.
	TerminalMaxReadWait = 5 * time.Minute

	// TerminalStartWait bounds how long a waiting start blocks before
	// handing control back to the agent while the user keeps working. The
	// agent resumes waiting with a follow-up read.
	TerminalStartWait = TerminalMaxReadWait

	// terminalKillWait bounds how long a kill waits for the process to be
	// reaped so the final screen can be captured.
	terminalKillWait = 5 * time.Second
)

//go:embed terminal.md
var terminalDescription string

// Terminal actions.
const (
	terminalActionStart    = "start"
	terminalActionRead     = "read"
	terminalActionWrite    = "write"
	terminalActionPaste    = "paste"
	terminalActionKill     = "kill"
	terminalActionHandover = "handover"
	terminalActionTake     = "take"
)

type TerminalParams struct {
	Action string `json:"action" description:"What to do: start (run a command), read (see the screen), write (send keystrokes), paste (send text as a paste), kill (end the session), handover (give the keyboard to the user), or take (ask the user for the keyboard)"`

	// start
	Command     string `json:"command,omitempty" description:"The command to run in the terminal (start only)"`
	WorkingDir  string `json:"working_dir,omitempty" description:"The working directory for the command (start only, defaults to the project directory)"`
	Description string `json:"description,omitempty" description:"start: a brief description of what the command does; handover: what the user should do in the terminal. Try to keep it under 30 characters or so"`
	Wait        *bool  `json:"wait,omitempty" description:"Wait for the user to finish before returning (start and handover, default true). The user holds the keyboard and can type in the terminal; the call blocks until the command exits, the user hands the terminal back or closes it, or the wait budget elapses. Set false on start when you drive the terminal: the user sees a read-only view they cannot type into, and you drive it with write/read or let it run"`

	// write
	Text     string   `json:"text,omitempty" description:"The exact text/keystrokes to send (write/paste). Use \\r for Enter, \\u0003 for Ctrl+C, and escape sequences for arrow keys (\\u001b[A/B/C/D)"`
	Keys     []string `json:"keys,omitempty" description:"Named keys to send instead of text (write only): \"enter\", \"up\"/\"down\"/\"left\"/\"right\", \"tab\", \"esc\", \"ctrl+c\", \"f5\", \"pageup\"/\"pagedown\", \"home\"/\"end\". Prefer this for full-screen apps, whose arrow keys only work when the terminal encodes them for the mode the app enabled"`
	SettleMs int      `json:"settle_ms,omitempty" description:"How long to wait for the session to redraw after writing or pasting before reporting the screen (default 250)"`

	// read
	IncludeScrollback bool `json:"include_scrollback,omitempty" description:"Also return everything that scrolled off the visible screen (read only)"`
	WaitSeconds       int  `json:"wait_seconds,omitempty" description:"Block until the session exits, the keyboard changes hands, or this many seconds elapse (read only, 0 returns immediately). Prefer this over polling in a loop"`

	// write (mouse)
	Mouse *TerminalMouse `json:"mouse,omitempty" description:"Send a mouse event instead of keys (write only). Programs that enabled mouse tracking receive it; others ignore it"`

	// read, write, paste, kill, handover, take
	SessionID string `json:"session_id,omitempty" description:"The terminal session ID (every action but start). Defaults to the most recent session"`
}

// TerminalMouse is a mouse event to send to a session. Coordinates are
// 1-based, matching the cursor position a read reports.
type TerminalMouse struct {
	Action string `json:"action" description:"One of: click, release, wheel, motion"`
	Button string `json:"button,omitempty" description:"left, middle, right, wheelup, or wheeldown (required for click, release, and wheel)"`
	X      int    `json:"x" description:"Column, 1-based"`
	Y      int    `json:"y" description:"Row, 1-based"`
}

type TerminalPermissionsParams struct {
	Description string `json:"description"`
	Command     string `json:"command"`
	WorkingDir  string `json:"working_dir"`
}

// TerminalResponseMetadata describes the session an action affected.
type TerminalResponseMetadata struct {
	Action     string `json:"action"`
	SessionID  string `json:"session_id"`
	Command    string `json:"command,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
	Exited     bool   `json:"exited,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	// UserControlled reports that the user holds the keyboard, so the
	// agent must take it before typing.
	UserControlled bool `json:"user_controlled,omitempty"`
}

// terminalOpInspect describes a session the agent wants to type into or
// take, so the permission prompt can name what runs there. It is not a
// tool action.
const terminalOpInspect = "inspect"

const terminalUnavailableMessage = "the terminal tool is unavailable in this environment: no terminal is attached. Run the command with bash instead, or ask the user to run it themselves"

func NewTerminalTool(permissions permission.Service, terminals terminal.Service, workingDir, spillDir string, perms *config.Permissions) fantasy.AgentTool {
	blocked := resolveBlockedCommands(perms)
	return fantasy.NewAgentTool(
		TerminalToolName,
		terminalDescription,
		func(ctx context.Context, params TerminalParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			tool := terminalTool{
				permissions: permissions,
				terminals:   terminals,
				workingDir:  workingDir,
				spillDir:    spillDir,
				call:        call,
			}
			switch params.Action {
			case terminalActionStart:
				return tool.start(ctx, params, blocked)
			case terminalActionRead, terminalActionKill, terminalActionHandover:
				// Handing the keyboard to the user only takes power away
				// from the agent, so it needs no permission.
				return tool.response(tool.run(ctx, params.Action, params))
			case terminalActionWrite, terminalActionPaste:
				return tool.input(ctx, params)
			case terminalActionTake:
				return tool.take(ctx, params)
			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf(
					"unknown action %q; expected one of: %s",
					params.Action, strings.Join([]string{
						terminalActionStart, terminalActionRead, terminalActionWrite, terminalActionPaste,
						terminalActionKill, terminalActionHandover, terminalActionTake,
					}, ", "),
				)), nil
			}
		},
	)
}

// terminalTool is the agent's half of the terminal tool: it validates a
// call and asks for permission, then has the process that owns the PTYs
// run it (see [ExecTerminal]). That is this process for a local TUI, or
// the client's when the agent runs on a server.
type terminalTool struct {
	permissions permission.Service
	terminals   terminal.Service
	workingDir  string
	spillDir    string
	call        fantasy.ToolCall
}

// start checks and approves a command, then spawns it in the embedded
// terminal.
func (t terminalTool) start(ctx context.Context, params TerminalParams, blocked []string) (fantasy.ToolResponse, error) {
	if params.Command == "" {
		return fantasy.NewTextErrorResponse("missing command"), nil
	}

	// The interactive session spawns a real shell, so the deny-list that
	// normally runs inside the interpreter never sees this command; check
	// it here instead.
	if err := shell.CheckBlocked(params.Command, blockFuncs(blocked)); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf(
			"%v. This command is blocked for safety; run a non-interactive alternative instead",
			err,
		)), nil
	}

	execWorkingDir := cmp.Or(params.WorkingDir, t.workingDir)

	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for starting a terminal session")
	}

	p, err := t.permissions.Request(
		ctx,
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        execWorkingDir,
			ToolCallID:  t.call.ID,
			ToolName:    TerminalToolName,
			Action:      "execute",
			Description: fmt.Sprintf("Run in an interactive terminal: %s", params.Command),
			Params: TerminalPermissionsParams{
				Description: params.Description,
				Command:     params.Command,
				WorkingDir:  execWorkingDir,
			},
		},
	)
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if !p {
		return NewPermissionDeniedResponse(), nil
	}

	return t.response(t.run(ctx, terminalActionStart, params))
}

// input asks the user before the agent types or pastes into a running
// session: whatever runs there (a shell, a REPL) executes the input with
// no further checks.
func (t terminalTool) input(ctx context.Context, params TerminalParams) (fantasy.ToolResponse, error) {
	info, err := t.run(ctx, terminalOpInspect, params)
	if err != nil || info.IsError {
		return t.response(info, err)
	}
	var session TerminalResponseMetadata
	if err := json.Unmarshal([]byte(info.Metadata), &session); err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("could not decode terminal session: %w", err)
	}

	verb, done := "Type into", "sent"
	if params.Action == terminalActionPaste {
		verb, done = "Paste into", "pasted"
	}
	if session.Exited {
		return fantasy.NewTextErrorResponse(fmt.Sprintf(
			"session %s has already exited; nothing was %s", session.SessionID, done,
		)), nil
	}

	input := params.Text
	if params.Action == terminalActionPaste {
		if params.Text == "" {
			return fantasy.NewTextErrorResponse("nothing to paste: provide text"), nil
		}
	} else {
		if params.Text == "" && len(params.Keys) == 0 && params.Mouse == nil {
			return fantasy.NewTextErrorResponse("nothing to send: provide text, keys, or mouse"), nil
		}
		if len(params.Keys) > 0 {
			input += fmt.Sprintf(" [keys: %s]", strings.Join(params.Keys, ", "))
		}
		if params.Mouse != nil {
			input += fmt.Sprintf(" [mouse %s %s at %d,%d]", params.Mouse.Action, params.Mouse.Button, params.Mouse.X, params.Mouse.Y)
		}
	}

	granted, err := t.permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   GetSessionFromContext(ctx),
		Path:        session.WorkingDir,
		ToolCallID:  t.call.ID,
		ToolName:    TerminalToolName,
		Action:      terminalActionWrite,
		Description: fmt.Sprintf("%s the interactive terminal running %s: %q", verb, commandLabel(session.Command), input),
		Params: TerminalPermissionsParams{
			Description: verb + " the running terminal",
			Command:     input,
			WorkingDir:  session.WorkingDir,
		},
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if !granted {
		return NewPermissionDeniedResponse(), nil
	}

	// Act on the session that was approved, not whichever is newest now.
	params.SessionID = session.SessionID
	return t.response(t.run(ctx, params.Action, params))
}

// take asks for the keyboard of a terminal the user holds: one they opened
// themselves, or one the agent handed them. Moving control to the agent is
// the user's call, so it goes through a permission prompt.
func (t terminalTool) take(ctx context.Context, params TerminalParams) (fantasy.ToolResponse, error) {
	info, err := t.run(ctx, terminalOpInspect, params)
	if err != nil || info.IsError {
		return t.response(info, err)
	}
	var session TerminalResponseMetadata
	if err := json.Unmarshal([]byte(info.Metadata), &session); err != nil {
		return fantasy.ToolResponse{}, fmt.Errorf("could not decode terminal session: %w", err)
	}
	if session.Exited {
		return fantasy.NewTextErrorResponse(fmt.Sprintf(
			"session %s has already exited; there is nothing to take", session.SessionID,
		)), nil
	}
	if !session.UserControlled {
		return fantasy.NewTextResponse(fmt.Sprintf(
			"You already hold the keyboard of terminal session %s.", session.SessionID,
		)), nil
	}

	granted, err := t.permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   GetSessionFromContext(ctx),
		Path:        session.WorkingDir,
		ToolCallID:  t.call.ID,
		ToolName:    TerminalToolName,
		Action:      terminalActionTake,
		Description: fmt.Sprintf("Take over the interactive terminal running %s", commandLabel(session.Command)),
		Params: TerminalPermissionsParams{
			Description: "Hand your terminal to the agent",
			Command:     session.Command,
			WorkingDir:  session.WorkingDir,
		},
	})
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if !granted {
		return NewPermissionDeniedResponse(), nil
	}

	params.SessionID = session.SessionID
	return t.response(t.run(ctx, terminalActionTake, params))
}

// commandLabel names a session's command for a prompt; an empty command is
// a bare shell.
func commandLabel(command string) string {
	if command == "" {
		return "an interactive shell"
	}
	return strconv.Quote(command)
}

// run executes op on the process that owns the terminals: a client, when
// the service forwards calls, or this one.
func (t terminalTool) run(ctx context.Context, op string, params TerminalParams) (terminal.Result, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return terminal.Result{}, err
	}
	call := terminal.Call{
		SessionID:  GetSessionFromContext(ctx),
		ToolCallID: t.call.ID,
		Op:         op,
		Params:     raw,
	}

	result, remote, err := t.terminals.Exec(ctx, call)
	switch {
	case !remote:
		return ExecTerminal(ctx, t.terminals, t.workingDir, call)
	case err == nil:
		return result, nil
	case ctx.Err() != nil:
		return terminal.Result{}, ctx.Err()
	case errors.Is(err, terminal.ErrUnavailable):
		return terminalError(terminalUnavailableMessage), nil
	default:
		return terminalError(fmt.Sprintf("terminal %s failed: %v", op, err)), nil
	}
}

// response turns a terminal result into the tool's response. Oversized
// output spills here, on the agent's machine, where the agent can read
// the spilled file back.
func (t terminalTool) response(result terminal.Result, err error) (fantasy.ToolResponse, error) {
	if err != nil {
		return fantasy.ToolResponse{}, err
	}
	if result.IsError {
		return fantasy.NewTextErrorResponse(result.Content), nil
	}
	resp := fantasy.NewTextResponse(TruncateOutput(result.Content, t.spillDir))
	resp.Metadata = result.Metadata
	return resp, nil
}

// ExecTerminal runs a terminal call against this process's interactive
// sessions. It is the half of the terminal tool that runs where the PTYs
// live, after the agent's half validated the call and got permission.
// workingDir is the default directory for new sessions, and terminals
// shows them to the TUI of this process.
func ExecTerminal(ctx context.Context, terminals terminal.Service, workingDir string, call terminal.Call) (terminal.Result, error) {
	var params TerminalParams
	if err := json.Unmarshal(call.Params, &params); err != nil {
		return terminalError(fmt.Sprintf("invalid terminal parameters: %v", err)), nil
	}

	host := terminalHost{
		terminals:  terminals,
		workingDir: workingDir,
		chat:       call.SessionID,
		toolCallID: call.ToolCallID,
	}
	switch call.Op {
	case terminalOpInspect:
		return host.inspect(params)
	case terminalActionStart:
		return host.start(ctx, params)
	case terminalActionRead:
		return host.read(ctx, params)
	case terminalActionWrite:
		return host.write(ctx, params)
	case terminalActionPaste:
		return host.paste(ctx, params)
	case terminalActionKill:
		return host.kill(ctx, params)
	case terminalActionHandover:
		return host.handover(ctx, params)
	case terminalActionTake:
		return host.take(ctx, params)
	default:
		return terminalError(fmt.Sprintf("unknown terminal operation %q", call.Op)), nil
	}
}

// terminalHost runs terminal calls for one chat session against the
// interactive sessions of this process.
type terminalHost struct {
	terminals  terminal.Service
	workingDir string
	chat       string
	toolCallID string
}

func terminalError(content string) terminal.Result {
	return terminal.Result{Content: content, IsError: true}
}

func terminalResult(content string, metadata TerminalResponseMetadata) terminal.Result {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return terminal.Result{Content: content}
	}
	return terminal.Result{Content: content, Metadata: string(raw)}
}

// inspect describes a session the agent wants to type into, which needs
// the keyboard, or to take, which needs the user to hold it.
func (h terminalHost) inspect(params TerminalParams) (terminal.Result, error) {
	session, err := resolveInteractiveSession(h.chat, params.SessionID, params.Action != terminalActionTake)
	if err != nil {
		return terminalError(err.Error()), nil
	}
	return terminalResult("", TerminalResponseMetadata{
		Action:         terminalOpInspect,
		SessionID:      session.ID(),
		Command:        session.Command(),
		WorkingDir:     session.WorkingDir(),
		Exited:         session.Exited(),
		ExitCode:       session.ExitCode(),
		UserControlled: !agentControls(session, h.chat),
	}), nil
}

// agentControls reports whether the agent of chat holds the keyboard.
func agentControls(session *shell.InteractiveSession, chat string) bool {
	return session.Owner() == chat && session.AgentDriven()
}

// keyboardHolder tells the agent who may type into the session.
func keyboardHolder(session *shell.InteractiveSession, chat string) string {
	if agentControls(session, chat) {
		return "You hold the keyboard."
	}
	return "The user holds the keyboard; ask for it with take before typing."
}

// start spawns a command in the embedded terminal and shows it to the
// user.
func (h terminalHost) start(ctx context.Context, params TerminalParams) (terminal.Result, error) {
	if h.terminals == nil || !h.terminals.Available() {
		return terminalError(terminalUnavailableMessage), nil
	}

	execWorkingDir := cmp.Or(params.WorkingDir, h.workingDir)

	manager := shell.GetInteractiveSessionManager()
	session, err := manager.Start(shell.InteractiveSessionOptions{
		Command:    params.Command,
		WorkingDir: execWorkingDir,
		Cols:       shell.DefaultInteractiveCols,
		Rows:       shell.DefaultInteractiveRows,
		Owner:      h.chat,
		// A waiting start puts the user at the keyboard.
		AgentDriven: !terminalStartWaits(params),
	})
	if err != nil {
		if errors.Is(err, shell.ErrInteractiveActive) {
			return terminalError(fmt.Sprintf(
				"%v. If you hold its keyboard, drive it with read/write or end it with kill first; if the user holds it, wait for them (read with wait_seconds) or ask for the keyboard with take",
				err,
			)), nil
		}
		return terminalError(fmt.Sprintf("could not start terminal session: %v", err)), nil
	}

	if _, err := h.terminals.Show(ctx, terminal.Request{
		SessionID:   h.chat,
		ToolCallID:  h.toolCallID,
		Command:     params.Command,
		WorkingDir:  execWorkingDir,
		Description: params.Description,
		AgentDriven: !terminalStartWaits(params),
		Session:     session,
	}); err != nil {
		// Nobody can display it; do not leave a stray process behind.
		_ = session.Kill()
		_ = session.Close()
		manager.Remove(session.ID())
		return terminalError(fmt.Sprintf("could not open terminal: %v", err)), nil
	}

	metadata := TerminalResponseMetadata{
		Action:    terminalActionStart,
		SessionID: session.ID(),
		Command:   params.Command,
	}

	// By default a start waits for the user: the agent pauses here until
	// the user finishes their work in the terminal.
	if terminalStartWaits(params) {
		return terminalWaitForUser(ctx, session, h.chat, metadata)
	}

	output := fmt.Sprintf(
		"Terminal session %s started with command: %s\n\n"+
			"The terminal is open on the user's screen in read-only mode: they can watch but not type into it.\n\n"+
			"Next steps: read to see the current screen (set wait_seconds to block until it exits instead of polling), "+
			"write to send keystrokes (for example %q to answer a prompt), handover when the user must type (a password, a choice only they can make), "+
			"and kill to end the session.\n\n"+
			"The session keeps running until the command exits or it is killed.",
		session.ID(), params.Command, "y\\r",
	)

	return terminalResult(output, metadata), nil
}

// terminalStartWaits reports whether a start call should block until the
// user finishes in the terminal. Waiting is the default: the agent should
// pause whenever a human is the one working in the session.
func terminalStartWaits(params TerminalParams) bool {
	return params.Wait == nil || *params.Wait
}

// terminalStartWaitBudget is the cut-off for a waiting start or handover.
// It is a var so tests can shorten it.
var terminalStartWaitBudget = TerminalStartWait

// terminalWaitForUser pauses the agent while the user works in the
// terminal. The call returns once the command exits, the user hands the
// keyboard to the agent or closes the terminal (which kills the session),
// the wait budget elapses, or the run is canceled.
func terminalWaitForUser(
	ctx context.Context,
	session *shell.InteractiveSession,
	chat string,
	metadata TerminalResponseMetadata,
) (terminal.Result, error) {
	if err := awaitUser(ctx, session, chat, terminalStartWaitBudget); err != nil {
		return terminal.Result{}, err
	}

	metadata.Exited = session.Exited()
	metadata.ExitCode = session.ExitCode()

	var output string
	switch {
	case session.Exited():
		transcript := session.CaptureText()
		if transcript == "" {
			transcript = BashNoOutput
		}
		output = fmt.Sprintf(
			"Terminal session %s finished (command: %s, exit code %d).\n\n<transcript>\n%s\n</transcript>",
			session.ID(), session.Command(), session.ExitCode(), transcript,
		)
	case agentControls(session, chat):
		output = fmt.Sprintf(
			"The user handed you the keyboard of terminal session %s (command: %s): drive it with write or paste, hand it back with handover, or end it with kill.\n\n"+
				"<screen>\n%s\n</screen>\n\n%s",
			session.ID(), session.Command(), screenText(session), sessionGeometry(session),
		)
	default:
		// The wait budget elapsed with the user still working. Hand control
		// back with the current screen so the agent can decide to keep
		// waiting (read wait_seconds) or ask for the keyboard (take).
		metadata.UserControlled = true
		output = fmt.Sprintf(
			"Terminal session %s (command: %s) is still running after %s; the user may still be working in it.\n\n"+
				"Keep waiting with read (wait_seconds), or ask the user for the keyboard with take. The session ends when the command exits or it is killed.\n\n"+
				"<screen>\n%s\n</screen>",
			session.ID(), session.Command(), terminalStartWaitBudget, screenText(session),
		)
	}

	return terminalResult(output, metadata), nil
}

// awaitUser blocks while the user holds the keyboard of a running
// session: until the command exits, the user hands the keyboard to the
// agent of chat, wait elapses, or ctx ends.
func awaitUser(ctx context.Context, session *shell.InteractiveSession, chat string, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		// Take the channel before checking, so a handover in between is
		// not missed.
		changed := session.ControlChanged()
		if session.Exited() || agentControls(session, chat) {
			return nil
		}
		select {
		case <-session.Done():
			return nil
		case <-changed:
		case <-timer.C:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// screenText returns the visible screen, or a placeholder when it is
// blank.
func screenText(session *shell.InteractiveSession) string {
	if screen := session.ScreenText(); screen != "" {
		return screen
	}
	return BashNoOutput
}

// handover gives the user the keyboard of a session the agent drives, and
// by default waits for them to finish.
func (h terminalHost) handover(ctx context.Context, params TerminalParams) (terminal.Result, error) {
	if h.terminals == nil || !h.terminals.Available() {
		return terminalError(terminalUnavailableMessage), nil
	}
	session, err := resolveInteractiveSession(h.chat, params.SessionID, true)
	if err != nil {
		return terminalError(err.Error()), nil
	}
	if session.Exited() {
		return terminalError(fmt.Sprintf("session %s has already exited; there is nothing to hand over", session.ID())), nil
	}

	session.HandToUser()
	if _, err := h.terminals.Show(ctx, terminal.Request{
		SessionID:   h.chat,
		ToolCallID:  h.toolCallID,
		Command:     session.Command(),
		WorkingDir:  session.WorkingDir(),
		Description: params.Description,
		Session:     session,
	}); err != nil {
		session.HandToAgent(h.chat)
		return terminalError(fmt.Sprintf("could not hand the terminal over: %v", err)), nil
	}

	metadata := TerminalResponseMetadata{
		Action:    terminalActionHandover,
		SessionID: session.ID(),
		Command:   session.Command(),
	}
	if terminalStartWaits(params) {
		return terminalWaitForUser(ctx, session, h.chat, metadata)
	}

	metadata.UserControlled = true
	return terminalResult(fmt.Sprintf(
		"The user now holds the keyboard of terminal session %s; you cannot type into it until they hand it back or you take it with take. "+
			"Read it to follow along: wait_seconds blocks until the keyboard changes hands or the command exits.",
		session.ID(),
	), metadata), nil
}

// take gives the agent the keyboard of a session the user holds, once
// the user allowed it.
func (h terminalHost) take(ctx context.Context, params TerminalParams) (terminal.Result, error) {
	session, err := resolveInteractiveSession(h.chat, params.SessionID, false)
	if err != nil {
		return terminalError(err.Error()), nil
	}
	if session.Exited() {
		return terminalError(fmt.Sprintf("session %s has already exited; there is nothing to take", session.ID())), nil
	}

	session.HandToAgent(h.chat)
	if h.terminals != nil {
		// Turn the panel read-only. Without a UI there is nothing to
		// update.
		_, _ = h.terminals.Show(ctx, terminal.Request{
			SessionID:   h.chat,
			ToolCallID:  h.toolCallID,
			Command:     session.Command(),
			WorkingDir:  session.WorkingDir(),
			AgentDriven: true,
			Session:     session,
		})
	}

	return terminalResult(fmt.Sprintf(
		"You now hold the keyboard of terminal session %s; the user watches a read-only view. Drive it with write or paste, hand it back with handover, or end it with kill.\n\n"+
			"<screen>\n%s\n</screen>\n\n%s",
		session.ID(), screenText(session), sessionGeometry(session),
	), TerminalResponseMetadata{
		Action:    terminalActionTake,
		SessionID: session.ID(),
		Command:   session.Command(),
	}), nil
}

// read returns the session's current screen, optionally waiting for it to
// exit.
func (h terminalHost) read(ctx context.Context, params TerminalParams) (terminal.Result, error) {
	session, err := resolveInteractiveSession(h.chat, params.SessionID, false)
	if err != nil {
		return terminalError(err.Error()), nil
	}

	if params.WaitSeconds > 0 && !session.Exited() {
		wait := min(time.Duration(params.WaitSeconds)*time.Second, TerminalMaxReadWait)
		changed := session.ControlChanged()
		select {
		case <-session.Done():
		case <-changed:
		case <-time.After(wait):
		case <-ctx.Done():
			return terminal.Result{}, ctx.Err()
		}
	}

	var parts []string
	if params.IncludeScrollback {
		if scrollback := session.ScrollbackText(); scrollback != "" {
			parts = append(parts, "<scrollback>\n"+scrollback+"\n</scrollback>")
		}
	}

	screen := session.ScreenText()
	if screen == "" {
		screen = BashNoOutput
	}
	parts = append(parts, "<screen>\n"+screen+"\n</screen>")
	status := fmt.Sprintf("Session %s: %s.", session.ID(), sessionStatus(session))
	if !session.Exited() {
		status += " " + keyboardHolder(session, h.chat)
	}
	parts = append(parts, status)
	parts = append(parts, sessionGeometry(session))

	return terminalResult(strings.Join(parts, "\n\n"), TerminalResponseMetadata{
		Action:         terminalActionRead,
		SessionID:      session.ID(),
		Command:        session.Command(),
		Exited:         session.Exited(),
		ExitCode:       session.ExitCode(),
		UserControlled: !session.Exited() && !agentControls(session, h.chat),
	}), nil
}

// sessionStatus describes whether the session still runs.
func sessionStatus(session *shell.InteractiveSession) string {
	if !session.Exited() {
		return "running"
	}
	if exitCode := session.ExitCode(); exitCode != 0 {
		return fmt.Sprintf("exited (exit code %d)", exitCode)
	}
	return "exited"
}

// sessionGeometry reports the session's state an agent needs between
// keystrokes: its logical size (which can be larger than the panel the
// user watches), the cursor position (even when hidden, since apps still
// move it while drawing), the working directory the child last reported,
// and whether a full-screen TUI is drawing.
func sessionGeometry(session *shell.InteractiveSession) string {
	cols, rows := session.Size()
	geom := fmt.Sprintf("Screen is %d columns by %d rows", cols, rows)
	if x, y, hidden := session.Cursor(); x >= 0 {
		cursor := fmt.Sprintf("Cursor at row %d, column %d", y+1, x+1)
		if hidden {
			cursor += " (the program hid its cursor)"
		}
		geom += ". " + cursor
	}
	if dir := session.WorkingDir(); dir != "" {
		geom += ". Working dir: " + dir
	}
	if session.InAltScreen() {
		geom += ". The program is drawing a full-screen TUI (alternate screen)"
	}
	if bell := session.LastBell(); !bell.IsZero() && time.Since(bell) < 10*time.Second {
		geom += ". The program rang the terminal bell just now"
	}
	return geom
}

// defaultTerminalWriteSettle is how long a write waits for the session to
// redraw before the screen is reported.
const defaultTerminalWriteSettle = 250 * time.Millisecond

// write sends text and keystrokes to the session and, after letting it
// redraw, returns the updated screen.
func (h terminalHost) write(ctx context.Context, params TerminalParams) (terminal.Result, error) {
	session, err := resolveInteractiveSession(h.chat, params.SessionID, true)
	if err != nil {
		return terminalError(err.Error()), nil
	}
	if session.Exited() {
		return terminalError(fmt.Sprintf("session %s has already exited; nothing was sent", session.ID())), nil
	}

	if params.Text != "" {
		if err := session.Write([]byte(params.Text)); err != nil {
			return terminalError(fmt.Sprintf("could not write to session %s: %v", session.ID(), err)), nil
		}
	}
	for _, name := range params.Keys {
		key, err := shell.ParseKey(name)
		if err != nil {
			return terminalError(err.Error()), nil
		}
		if err := session.SendKey(key); err != nil {
			return terminalError(err.Error()), nil
		}
	}
	if params.Mouse != nil {
		event, err := terminalMouseEvent(*params.Mouse)
		if err != nil {
			return terminalError(err.Error()), nil
		}
		if err := session.SendMouse(event); err != nil {
			return terminalError(err.Error()), nil
		}
	}

	sent := fmt.Sprintf("Sent %d bytes and %d keystrokes to terminal session %s",
		len(params.Text), len(params.Keys), session.ID())
	return terminalWriteResult(ctx, session, terminalActionWrite, sent, params)
}

// paste sends text as a paste: when the child enabled bracketed paste mode
// the text is wrapped in paste markers, so multi-line pastes are inserted
// into an editor or command line instead of executed line by line.
func (h terminalHost) paste(ctx context.Context, params TerminalParams) (terminal.Result, error) {
	session, err := resolveInteractiveSession(h.chat, params.SessionID, true)
	if err != nil {
		return terminalError(err.Error()), nil
	}
	if session.Exited() {
		return terminalError(fmt.Sprintf("session %s has already exited; nothing was pasted", session.ID())), nil
	}

	if err := session.Paste(params.Text); err != nil {
		return terminalError(fmt.Sprintf("could not paste to session %s: %v", session.ID(), err)), nil
	}
	return terminalWriteResult(ctx, session, terminalActionPaste, fmt.Sprintf("Pasted %d characters into terminal session %s", len(params.Text), session.ID()), params)
}

// terminalWriteResult waits for the session to redraw and reports the
// resulting screen.
func terminalWriteResult(ctx context.Context, session *shell.InteractiveSession, action, sent string, params TerminalParams) (terminal.Result, error) {
	// Let the session redraw before reporting the screen.
	settle := defaultTerminalWriteSettle
	if params.SettleMs > 0 {
		settle = min(time.Duration(params.SettleMs)*time.Millisecond, 10*time.Second)
	}
	select {
	case <-time.After(settle):
	case <-ctx.Done():
		return terminal.Result{}, ctx.Err()
	}

	screen := session.ScreenText()
	if screen == "" {
		screen = BashNoOutput
	}

	response := fmt.Sprintf(
		"%s. Session: %s.\n\n<screen>\n%s\n</screen>\n\n%s",
		sent, sessionStatus(session), screen, sessionGeometry(session),
	)
	return terminalResult(response, TerminalResponseMetadata{
		Action:    action,
		SessionID: session.ID(),
		Command:   session.Command(),
		Exited:    session.Exited(),
		ExitCode:  session.ExitCode(),
	}), nil
}

// kill ends the session and returns its final transcript.
func (h terminalHost) kill(ctx context.Context, params TerminalParams) (terminal.Result, error) {
	session, err := resolveInteractiveSession(h.chat, params.SessionID, true)
	if err != nil {
		return terminalError(err.Error()), nil
	}

	wasRunning := !session.Exited()
	killErr := session.Kill()
	if wasRunning {
		select {
		case <-session.Done():
		case <-time.After(terminalKillWait):
		case <-ctx.Done():
			return terminal.Result{}, ctx.Err()
		}
	}

	output := session.CaptureText()
	if output == "" {
		output = BashNoOutput
	}

	status := "The session had already exited."
	if wasRunning {
		status = "Session terminated."
		if killErr != nil {
			status = fmt.Sprintf("Session terminated (kill reported: %v).", killErr)
		}
	}

	exitCode := session.ExitCode()
	if exitCode != 0 {
		status += fmt.Sprintf(" Exit code %d.", exitCode)
	}

	result := fmt.Sprintf("%s\n\n<transcript>\n%s\n</transcript>", status, output)
	return terminalResult(result, TerminalResponseMetadata{
		Action:    terminalActionKill,
		SessionID: session.ID(),
		Command:   session.Command(),
		Exited:    true,
		ExitCode:  exitCode,
	}), nil
}

// resolveInteractiveSession finds the session named by id, or the running
// session (or most recent) when id is empty. Sessions another chat started
// are invisible. With mustControl, the agent must hold the keyboard: the
// user's own terminals and the ones handed to them are readable, but the
// agent may only type into or kill them after taking the keyboard.
func resolveInteractiveSession(chat, id string, mustControl bool) (*shell.InteractiveSession, error) {
	manager := shell.GetInteractiveSessionManager()
	visible := func(s *shell.InteractiveSession) bool {
		return s.Owner() == "" || s.Owner() == chat
	}

	var session *shell.InteractiveSession
	if id == "" {
		for _, sid := range manager.List() {
			s, ok := manager.Get(sid)
			if !ok || !visible(s) {
				continue
			}
			// Prefer a running session, then the newest one.
			if session == nil || (!s.Exited() && session.Exited()) ||
				(s.Exited() == session.Exited() && s.ID() > session.ID()) {
				session = s
			}
		}
		if session == nil {
			return nil, fmt.Errorf("no terminal session is running")
		}
	} else {
		s, ok := manager.Get(id)
		if !ok || !visible(s) {
			return nil, fmt.Errorf("terminal session not found: %s", id)
		}
		session = s
	}

	// The keyboard of a finished session no longer matters.
	if mustControl && !session.Exited() && !agentControls(session, chat) {
		if session.Owner() == "" {
			return nil, fmt.Errorf(
				"terminal session %s belongs to the user; you can read it, but to type into it or kill it, ask for the keyboard with action take",
				session.ID(),
			)
		}
		return nil, fmt.Errorf(
			"the user holds the keyboard of terminal session %s; wait for them to hand it back (read with wait_seconds) or ask for it with action take",
			session.ID(),
		)
	}
	return session, nil
}

// terminalMouseEvent converts a tool mouse event into an emulator event.
// Input coordinates are 1-based; the emulator (and the SGR encoding) use
// 0-based positions.
func terminalMouseEvent(in TerminalMouse) (uv.MouseEvent, error) {
	button, err := terminalMouseButton(in.Button)
	if err != nil {
		return nil, err
	}

	mouse := uv.Mouse{
		X:      max(in.X-1, 0),
		Y:      max(in.Y-1, 0),
		Button: button,
	}

	switch strings.ToLower(in.Action) {
	case "click":
		if button == uv.MouseNone {
			return nil, errors.New("mouse button is required for a click")
		}
		return uv.MouseClickEvent(mouse), nil
	case "release":
		if button == uv.MouseNone {
			return nil, errors.New("mouse button is required for a release")
		}
		return uv.MouseReleaseEvent(mouse), nil
	case "wheel":
		if button != uv.MouseWheelUp && button != uv.MouseWheelDown &&
			button != uv.MouseWheelLeft && button != uv.MouseWheelRight {
			return nil, fmt.Errorf("wheel needs a wheel button (wheelup or wheeldown), got %q", in.Button)
		}
		return uv.MouseWheelEvent(mouse), nil
	case "motion":
		return uv.MouseMotionEvent(mouse), nil
	case "":
		return nil, errors.New("mouse action is required (click, release, wheel, or motion)")
	default:
		return nil, fmt.Errorf("unknown mouse action %q", in.Action)
	}
}

// terminalMouseButton maps a button name onto an emulator button.
func terminalMouseButton(name string) (uv.MouseButton, error) {
	switch strings.ToLower(name) {
	case "":
		return uv.MouseNone, nil
	case "left":
		return uv.MouseLeft, nil
	case "middle":
		return uv.MouseMiddle, nil
	case "right":
		return uv.MouseRight, nil
	case "wheelup", "wheel-up", "scrollup":
		return uv.MouseWheelUp, nil
	case "wheeldown", "wheel-down", "scrolldown":
		return uv.MouseWheelDown, nil
	case "wheelleft":
		return uv.MouseWheelLeft, nil
	case "wheelright":
		return uv.MouseWheelRight, nil
	default:
		return uv.MouseNone, fmt.Errorf("unknown mouse button %q", name)
	}
}
