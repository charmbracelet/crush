package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
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
	terminalActionStart = "start"
	terminalActionRead  = "read"
	terminalActionWrite = "write"
	terminalActionKill  = "kill"
)

type TerminalParams struct {
	Action string `json:"action" description:"What to do: start (run a command), read (see the screen), write (send keystrokes), or kill (end the session)"`

	// start
	Command     string `json:"command,omitempty" description:"The command to run in the terminal (start only)"`
	WorkingDir  string `json:"working_dir,omitempty" description:"The working directory for the command (start only, defaults to the project directory)"`
	Description string `json:"description,omitempty" description:"A brief description of what the command does (start only, try to keep it under 30 characters or so)"`
	Wait        *bool  `json:"wait,omitempty" description:"Wait for the user to finish before returning (start only, default true). The user owns the terminal and can type in it; the call blocks until the command exits, the user closes the terminal, or the wait budget elapses, and returns the transcript. Set false when you own the terminal: the user sees a read-only view they cannot type into, and you drive it with write/read or let it run"`

	// write
	Text     string   `json:"text,omitempty" description:"The exact text/keystrokes to send (write only). Use \\r for Enter, \\u0003 for Ctrl+C, and escape sequences for arrow keys (\\u001b[A/B/C/D)"`
	Keys     []string `json:"keys,omitempty" description:"Named keys to send instead of text (write only): \"enter\", \"up\"/\"down\"/\"left\"/\"right\", \"tab\", \"esc\", \"ctrl+c\", \"f5\", \"pageup\"/\"pagedown\", \"home\"/\"end\". Prefer this for full-screen apps, whose arrow keys only work when the terminal encodes them for the mode the app enabled"`
	SettleMs int      `json:"settle_ms,omitempty" description:"How long to wait for the session to redraw after writing before reporting the screen (write only, default 250)"`

	// read
	IncludeScrollback bool `json:"include_scrollback,omitempty" description:"Also return everything that scrolled off the visible screen (read only)"`
	WaitSeconds       int  `json:"wait_seconds,omitempty" description:"Block until the session exits or this many seconds elapse (read only, 0 returns immediately). Prefer this over polling in a loop"`

	// write (mouse)
	Mouse *TerminalMouse `json:"mouse,omitempty" description:"Send a mouse event instead of keys (write only). Programs that enabled mouse tracking receive it; others ignore it"`

	// read, write, kill
	SessionID string `json:"session_id,omitempty" description:"The terminal session ID (read/write/kill). Defaults to the most recent session"`
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
	Action    string `json:"action"`
	SessionID string `json:"session_id"`
	Command   string `json:"command,omitempty"`
	Exited    bool   `json:"exited,omitempty"`
	ExitCode  int    `json:"exit_code,omitempty"`
}

func NewTerminalTool(permissions permission.Service, terminals terminal.Service, workingDir, spillDir string) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		TerminalToolName,
		terminalDescription,
		func(ctx context.Context, params TerminalParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			switch params.Action {
			case terminalActionStart:
				return terminalStart(ctx, permissions, terminals, params, call, workingDir, spillDir)
			case terminalActionRead:
				return terminalRead(ctx, params, spillDir)
			case terminalActionWrite:
				return terminalWrite(ctx, params)
			case terminalActionKill:
				return terminalKill(ctx, params, spillDir)
			default:
				return fantasy.NewTextErrorResponse(fmt.Sprintf(
					"unknown action %q; expected one of: %s, %s, %s, %s",
					params.Action, terminalActionStart, terminalActionRead, terminalActionWrite, terminalActionKill,
				)), nil
			}
		},
	)
}

// terminalStart spawns a command in the embedded terminal and returns its
// session ID immediately.
func terminalStart(
	ctx context.Context,
	permissions permission.Service,
	terminals terminal.Service,
	params TerminalParams,
	call fantasy.ToolCall,
	workingDir string,
	spillDir string,
) (fantasy.ToolResponse, error) {
	if params.Command == "" {
		return fantasy.NewTextErrorResponse("missing command"), nil
	}

	if terminals == nil || !terminals.Available() {
		return fantasy.NewTextErrorResponse(
			"the terminal tool is unavailable in this environment: no terminal is attached. Run the command with bash instead, or ask the user to run it themselves",
		), nil
	}

	// The interactive session spawns a real shell, so the deny-list that
	// normally runs inside the interpreter never sees this command; check
	// it here instead.
	if err := shell.CheckBlocked(params.Command, blockFuncs()); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf(
			"%v. This command is blocked for safety; run a non-interactive alternative instead",
			err,
		)), nil
	}

	execWorkingDir := params.WorkingDir
	if execWorkingDir == "" {
		execWorkingDir = workingDir
	}

	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for starting a terminal session")
	}

	p, err := permissions.Request(
		ctx,
		permission.CreatePermissionRequest{
			SessionID:   sessionID,
			Path:        execWorkingDir,
			ToolCallID:  call.ID,
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

	manager := shell.GetInteractiveSessionManager()
	session, err := manager.Start(shell.InteractiveSessionOptions{
		Command:    params.Command,
		WorkingDir: execWorkingDir,
		Cols:       shell.DefaultInteractiveCols,
		Rows:       shell.DefaultInteractiveRows,
	})
	if err != nil {
		if errors.Is(err, shell.ErrInteractiveActive) {
			return fantasy.NewTextErrorResponse(fmt.Sprintf(
				"%v. Drive the running session with action read/write, or end it with action kill first",
				err,
			)), nil
		}
		return fantasy.NewTextErrorResponse(fmt.Sprintf("could not start terminal session: %v", err)), nil
	}

	if _, err := terminals.Show(ctx, terminal.Request{
		SessionID:   sessionID,
		ToolCallID:  call.ID,
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
		return fantasy.NewTextErrorResponse(fmt.Sprintf("could not open terminal: %v", err)), nil
	}

	metadata := TerminalResponseMetadata{
		Action:    terminalActionStart,
		SessionID: session.ID(),
		Command:   params.Command,
	}

	// By default a start waits for the user: the agent pauses here until
	// the user finishes their work in the terminal.
	if terminalStartWaits(params) {
		return terminalWaitForUser(ctx, session, spillDir, metadata)
	}

	output := fmt.Sprintf(
		"Terminal session %s started with command: %s\n\n"+
			"The terminal is open on the user's screen in read-only mode: they can watch but not type into it.\n\n"+
			"Next steps: read to see the current screen (set wait_seconds to block until it exits instead of polling), "+
			"write to send keystrokes (for example %q to answer a prompt), and kill to end the session.\n\n"+
			"The session keeps running until the command exits or it is killed.",
		session.ID(), params.Command, "y\\r",
	)

	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(output), metadata), nil
}

// terminalStartWaits reports whether a start call should block until the
// user finishes in the terminal. Waiting is the default: the agent should
// pause whenever a human is the one working in the session.
func terminalStartWaits(params TerminalParams) bool {
	return params.Wait == nil || *params.Wait
}

// terminalStartWaitBudget is the cut-off for a waiting start. It is a var
// so tests can shorten it.
var terminalStartWaitBudget = TerminalStartWait

// terminalWaitForUser pauses the agent while the user works in the
// terminal. The call returns once the command exits, the user closes the
// terminal (which kills the session), the wait budget elapses, or the run
// is canceled.
func terminalWaitForUser(
	ctx context.Context,
	session *shell.InteractiveSession,
	spillDir string,
	metadata TerminalResponseMetadata,
) (fantasy.ToolResponse, error) {
	if !session.Exited() {
		select {
		case <-session.Done():
		case <-time.After(terminalStartWaitBudget):
		case <-ctx.Done():
			return fantasy.ToolResponse{}, ctx.Err()
		}
	}

	metadata.Exited = session.Exited()
	metadata.ExitCode = session.ExitCode()

	var output string
	if session.Exited() {
		transcript := TruncateOutput(session.CaptureText(), spillDir)
		if transcript == "" {
			transcript = BashNoOutput
		}
		output = fmt.Sprintf(
			"Terminal session %s finished (command: %s, exit code %d).\n\n<transcript>\n%s\n</transcript>",
			session.ID(), session.Command(), session.ExitCode(), transcript,
		)
	} else {
		// The wait budget elapsed with the user still working. Hand control
		// back with the current screen so the agent can decide to keep
		// waiting (read wait_seconds) or take over (write).
		screen := session.ScreenText()
		if screen == "" {
			screen = BashNoOutput
		}
		output = fmt.Sprintf(
			"Terminal session %s (command: %s) is still running after %s; the user may still be working in it.\n\n"+
				"Keep waiting with read (wait_seconds) or take over with write. The session ends when the command exits or it is killed.\n\n"+
				"<screen>\n%s\n</screen>",
			session.ID(), session.Command(), terminalStartWaitBudget, screen,
		)
	}

	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(output), metadata), nil
}

// terminalRead returns the session's current screen, optionally waiting for
// it to exit.
func terminalRead(ctx context.Context, params TerminalParams, spillDir string) (fantasy.ToolResponse, error) {
	session, err := resolveInteractiveSession(params.SessionID)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	if params.WaitSeconds > 0 && !session.Exited() {
		wait := min(time.Duration(params.WaitSeconds)*time.Second, TerminalMaxReadWait)
		select {
		case <-session.Done():
		case <-time.After(wait):
		case <-ctx.Done():
			return fantasy.ToolResponse{}, ctx.Err()
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

	status := "running"
	if session.Exited() {
		exitCode := session.ExitCode()
		status = "exited"
		if exitCode != 0 {
			status = fmt.Sprintf("exited (exit code %d)", exitCode)
		}
	}
	parts = append(parts, fmt.Sprintf("Session %s: %s", session.ID(), status))

	if x, y, hidden := session.Cursor(); !hidden {
		parts = append(parts, fmt.Sprintf("Cursor at row %d, column %d", y+1, x+1))
	}

	output := TruncateOutput(strings.Join(parts, "\n\n"), spillDir)

	metadata := TerminalResponseMetadata{
		Action:    terminalActionRead,
		SessionID: session.ID(),
		Command:   session.Command(),
		Exited:    session.Exited(),
		ExitCode:  session.ExitCode(),
	}

	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(output), metadata), nil
}

// defaultTerminalWriteSettle is how long a write waits for the session to
// redraw before the screen is reported.
const defaultTerminalWriteSettle = 250 * time.Millisecond

// terminalWrite sends text and keystrokes to the session and, after letting
// it redraw, returns the updated screen.
func terminalWrite(ctx context.Context, params TerminalParams) (fantasy.ToolResponse, error) {
	session, err := resolveInteractiveSession(params.SessionID)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	if session.Exited() {
		return fantasy.NewTextErrorResponse(fmt.Sprintf(
			"session %s has already exited; nothing was sent", session.ID(),
		)), nil
	}

	if params.Text == "" && len(params.Keys) == 0 && params.Mouse == nil {
		return fantasy.NewTextErrorResponse("nothing to send: provide text, keys, or mouse"), nil
	}

	if params.Text != "" {
		if err := session.Write([]byte(params.Text)); err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("could not write to session %s: %v", session.ID(), err)), nil
		}
	}
	for _, name := range params.Keys {
		key, err := shell.ParseKey(name)
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		if err := session.SendKey(key); err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
	}
	if params.Mouse != nil {
		event, err := terminalMouseEvent(*params.Mouse)
		if err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
		if err := session.SendMouse(event); err != nil {
			return fantasy.NewTextErrorResponse(err.Error()), nil
		}
	}

	// Let the session redraw before reporting the screen.
	settle := defaultTerminalWriteSettle
	if params.SettleMs > 0 {
		settle = min(time.Duration(params.SettleMs)*time.Millisecond, 10*time.Second)
	}
	select {
	case <-time.After(settle):
	case <-ctx.Done():
		return fantasy.ToolResponse{}, ctx.Err()
	}

	screen := session.ScreenText()
	if screen == "" {
		screen = BashNoOutput
	}

	status := "running"
	if session.Exited() {
		exitCode := session.ExitCode()
		status = "exited"
		if exitCode != 0 {
			status = fmt.Sprintf("exited (exit code %d)", exitCode)
		}
	}

	metadata := TerminalResponseMetadata{
		Action:    terminalActionWrite,
		SessionID: session.ID(),
		Command:   session.Command(),
		Exited:    session.Exited(),
		ExitCode:  session.ExitCode(),
	}

	response := fmt.Sprintf(
		"Sent %d bytes to terminal session %s. Session: %s.\n\n<screen>\n%s\n</screen>",
		len(params.Text), session.ID(), status, screen,
	)
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(response), metadata), nil
}

// terminalKill ends the session and returns its final transcript.
func terminalKill(ctx context.Context, params TerminalParams, spillDir string) (fantasy.ToolResponse, error) {
	session, err := resolveInteractiveSession(params.SessionID)
	if err != nil {
		return fantasy.NewTextErrorResponse(err.Error()), nil
	}

	wasRunning := !session.Exited()
	killErr := session.Kill()
	if wasRunning {
		select {
		case <-session.Done():
		case <-time.After(terminalKillWait):
		case <-ctx.Done():
			return fantasy.ToolResponse{}, ctx.Err()
		}
	}

	output := TruncateOutput(session.CaptureText(), spillDir)
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

	metadata := TerminalResponseMetadata{
		Action:    terminalActionKill,
		SessionID: session.ID(),
		Command:   session.Command(),
		Exited:    true,
		ExitCode:  exitCode,
	}

	result := fmt.Sprintf("%s\n\n<transcript>\n%s\n</transcript>", status, output)
	return fantasy.WithResponseMetadata(fantasy.NewTextResponse(result), metadata), nil
}

// resolveInteractiveSession finds the session named by id, or the running
// session (or most recent) when id is empty.
func resolveInteractiveSession(id string) (*shell.InteractiveSession, error) {
	manager := shell.GetInteractiveSessionManager()

	if id == "" {
		if session, ok := manager.Active(); ok {
			return session, nil
		}
		session, ok := manager.MostRecent()
		if !ok {
			return nil, fmt.Errorf("no terminal session is running")
		}
		return session, nil
	}

	session, ok := manager.Get(id)
	if !ok {
		return nil, fmt.Errorf("terminal session not found: %s (known sessions: %s)", id, strings.Join(manager.List(), ", "))
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
