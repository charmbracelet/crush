// Package discord implements the /rc remote-control bridge: a Discord
// thread bound to a live Crush session. Session events are mirrored into
// the thread, and messages typed in the thread are injected back into the
// session, so the same session can be driven from the terminal or from
// Discord.
package discord

// ModeChangedMsg reports that remote control switched the session's
// input mode. The TUI handles it by updating the mode badge and editor
// prompt, which otherwise only change through the TUI's own mode paths.
// Published with app.SendEvent.
type ModeChangedMsg struct {
	// Plan is true when the session switched to plan mode.
	Plan bool
}

// YoloChangedMsg reports that remote control toggled YOLO mode.
// Published with app.SendEvent so the TUI updates its badge, cache, and
// editor prompt.
type YoloChangedMsg struct {
	On bool
}

// ModelChangedMsg reports that remote control changed the preferred
// large model and rebuilt the agent's models. Published with
// app.SendEvent so the TUI re-probes its memoized model state.
type ModelChangedMsg struct {
	Provider string
	Model    string
}

// StatusNoteMsg carries a bridge notice (gateway trouble, injection
// failures) for the TUI status line. Published with app.SendEvent when
// the bridge runs inside the TUI process.
type StatusNoteMsg struct {
	Text  string
	IsErr bool
}

// SessionClearedMsg reports that remote control cleared the context by
// starting a new session and moving the thread to it (same thread).
// Published with app.SendEvent so the TUI follows when it is still
// showing the cleared session. The bridge pointer identifies which
// bridge cleared it when several were started over a session's lifetime.
type SessionClearedMsg struct {
	Bridge *Bridge
	From   string
	To     string
}

// StoppedMsg reports that a bridge shut itself down (for example via
// "/rc off" in Discord). Published with app.SendEvent so the TUI clears
// its reference; the bridge pointer identifies which bridge stopped when
// several were started over a session's lifetime.
type StoppedMsg struct {
	Bridge *Bridge
}

// AutomationCommandMsg asks the TUI to run a /goal or /loop command on
// behalf of the Discord bridge, which does not own the per-session
// automation state or its scheduler. The TUI replies on Reply with the
// text to post in the thread. Published with app.SendEvent.
type AutomationCommandMsg struct {
	// SessionID is the bridge's session. The TUI only runs the command
	// when it is showing that session, because the command starts a
	// turn in the open session.
	SessionID string
	// Name is "goal" or "loop".
	Name string
	// Args is the raw argument text after the command name.
	Args string
	// Reply receives the text to post. It is buffered by the sender.
	Reply chan string
}
