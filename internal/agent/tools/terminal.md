Run and control a command in a real terminal embedded in Crush's UI.

Use this tool when a command needs a terminal: it prompts for input (logins, tokens, passwords, confirmations) or it opens a full-screen UI you or the user must navigate. The terminal stays open for both of you, and only one session exists at a time.

<actions>
- start: run a command in a new terminal session. Returns a session ID immediately; the command keeps running until it exits or you kill it.
- read: show the session's current screen, its status (running / exited with code), and the cursor position. include_scrollback adds everything that scrolled off; wait_seconds blocks until the session exits.
- write: drive the session. Send literal text (text), named keys (keys), and/or a mouse event (mouse), then it returns the screen after the session has redrawn (settle_ms, default 250). Writes are not echoed back on their own.
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
Full-screen programs are usually driven one key at a time, and each write already returns the updated screen, so a navigation step costs a single call:

1. read (or the screen from your last write) to see where the cursor/highlight is — programs print their key hints somewhere on screen, use them
2. write with the key that moves you toward the target (for example { "action": "write", "keys": ["down", "down", "enter"] })
3. repeat until you reach the screen you need, then read it

The user can also be driving the same terminal at the same time: re-read before assuming state, and prefer small, deliberate steps over long key sequences.
</browsing_tuis>

<usage_notes>
- session_id is optional for read/write/kill: it defaults to the running session
- The user can end a session with ctrl+q, which kills the command; a later read reports the exit
- Results are plain text (not ANSI bytes) and truncated like bash output
- Commands that do not need a terminal belong in bash: it is faster and does not open a dialog
</usage_notes>

<examples>
- Log in: { "action": "start", "command": "gh auth login", "description": "GitHub login" }
- Answer a prompt: { "action": "write", "text": "y", "keys": ["enter"] }
- Browse a full-screen list: { "action": "write", "keys": ["down", "down", "down"] }
- Interrupt a running command: { "action": "write", "keys": ["ctrl+c"] }
- Click an item: { "action": "write", "mouse": { "action": "click", "button": "left", "x": 12, "y": 4 } }
- Wait for a slow command: { "action": "read", "wait_seconds": 120 }
- End it and read the transcript: { "action": "kill" }
</examples>
