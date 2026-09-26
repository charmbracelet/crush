Run and control a command in a real terminal embedded in Crush's UI.

Use this tool when a command needs a terminal: it prompts for input (logins, tokens, passwords, confirmations) or it opens a full-screen UI you or the user must navigate. The terminal stays open for both of you, and only one session exists at a time.

There are two kinds of terminal, chosen by the wait parameter on start:

- User-owned (default): the user acts in it — writing a commit message, logging in, finishing a TUI. You wait: the start call does not return until the command exits, the user closes the terminal, or a 5-minute budget elapses, and the response is the transcript. The session runs at the size of the panel the user sees.
- Agent-owned (wait: false): you drive it with write/read while the user watches. The panel is read-only for them: their keystrokes and mouse events do not reach the command, so they cannot interfere with what you are doing. The session runs full-window height (a ghost fullscreen) while the user's panel shows only the top slice of it — you see far more rows than the panel displays, and the geometry line in every result tells you the real size. The user can focus the panel and press ctrl+f to see the whole logical screen fullscreen.

<actions>
- start: run a command in a new terminal session. Blocks for the user by default (see above); with wait: false it returns a session ID immediately and the user gets a read-only view.
- read: show the session's current screen, its status (running / exited with code), its size, and the cursor position (reported even when the program hid its cursor), plus the working directory it last reported and whether a full-screen TUI is drawing. include_scrollback adds everything that scrolled off; wait_seconds blocks until the session exits.
- write: drive the session. Send literal text (text), named keys (keys), and/or a mouse event (mouse), then it returns the screen after the session has redrawn (settle_ms, default 250). Writes are not echoed back on their own.
- paste: send text as a paste. When the program enabled bracketed paste mode the text is wrapped in paste markers, so a multi-line paste is inserted into an editor or command line instead of being executed line by line. Prefer this over write for multi-line text.
- kill: terminate the session and get its final transcript (screen, scrollback, exit code). The terminal closes in the UI.
</actions>

<keys>
Send named keys instead of raw escape sequences; they are encoded for whatever mode the program enabled, which raw byte sequences cannot know:

- Navigation: "up", "down", "left", "right", "home", "end", "pageup", "pagedown", "tab", "shift+tab"
- Actions: "enter", "esc", "backspace", "delete", "insert", "space"
- Control: "ctrl+c", "ctrl+d", "ctrl+z", "ctrl+l", and any other "ctrl+letter"
- Function keys: "f1".."f12"
- Modifiers combine: "alt+enter", "ctrl+shift+left"

For literal text, use text (for example typing a filename or answering a password prompt); to press Enter after it, add "enter" to keys or use \r.
</keys>

<mouse>
Programs that ask for mouse tracking (many file managers, dashboards, and editors) also accept mouse events:

- mouse: { "action": "click", "button": "left", "x": 12, "y": 4 } — click at a screen cell. Coordinates are 1-based and match the cursor position a read reports.
- actions: click, release, wheel, motion; buttons: left, middle, right, wheelup, wheeldown.
- wheel: { "action": "wheel", "button": "wheeldown", "x": 1, "y": 1 } scrolls a list.
- Programs that never enabled mouse tracking ignore these events, exactly like a real terminal.
</mouse>

<browsing_tuis>
Full-screen programs are usually driven one key at a time, and each write already returns the updated screen, so a navigation step costs a single call. Start a TUI you will drive with wait: false, then:

1. read (or the screen from your last write) to see where the cursor/highlight is — programs print their key hints somewhere on screen, use them
2. write with the key that moves you toward the target (for example { "action": "write", "keys": ["down", "down", "enter"] })
3. repeat until you reach the screen you need, then read it

The user can also be driving the same terminal at the same time: re-read before assuming state, and prefer small, deliberate steps over long key sequences.
</browsing_tuis>

<usage_notes>
- Start a command for the user to act in (writing a commit message, logging in, finishing a TUI) without wait: false — the call pauses until they are done, and you get the transcript they produced
- When a waiting start reports the session is still running, the user is likely still working: wait again with { "action": "read", "wait_seconds": 300 } rather than taking over
- session_id is optional for read/write/kill: it defaults to the running session
- The user can end a session with ctrl+q, which kills the command; a later read reports the exit
- Results are plain text (not ANSI bytes) and truncated like bash output
- Commands that do not need a terminal belong in bash: it is faster and does not open a dialog
</usage_notes>

<examples>
- Ask the user to write a commit message: { "action": "start", "command": "git commit", "description": "User writes the commit message" }
- Log in with the user in control: { "action": "start", "command": "gh auth login", "description": "GitHub login" }
- Drive a prompt yourself: { "action": "start", "command": "gh auth login", "wait": false } then { "action": "write", "text": "y", "keys": ["enter"] }
- Browse a full-screen list: { "action": "write", "keys": ["down", "down", "down"] }
- Interrupt a running command: { "action": "write", "keys": ["ctrl+c"] }
- Click an item: { "action": "write", "mouse": { "action": "click", "button": "left", "x": 12, "y": 4 } }
- Wait for a slow command: { "action": "read", "wait_seconds": 120 }
- End it and read the transcript: { "action": "kill" }
</examples>
