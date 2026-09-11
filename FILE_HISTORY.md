# Files and conversation history

## Rewind in the terminal

Checkpoints are saved automatically before new prompts. In the chat input, type
`/rewind`, select a prompt with the arrow keys, press Enter, and confirm. Both
workspace files and the conversation return to before that prompt. `/redo`
restores both to before the last rewind; repeated redo walks successive rewinds.
A new prompt clears redo. Esc cancels the selector.

Wait for the agent to finish before restoring. If a restore is interrupted, run
`/rewind-recover` before continuing. Recovery is journaled before file writes;
conversation changes follow successful file restoration. New combined history
starts with prompts captured by this build, not legacy file-only checkpoints.

Capture is bounded. Ignored, remote, or uncaptured files are not protected.
Known local write/edit paths are declared before mutation, including absent files;
binary files are restored as bytes. Shell/MCP writes need pre-existing capture
coverage. These commands do not call a model.

Automatic capture is enabled on Linux, macOS, and Windows (amd64/arm64),
unless `option file-history false` is set. Release builds carry the official
filesnap 0.5.0 archives and extract the matching executable automatically. No
separate CLI installation, Node/npm, Rust compiler, CGO, PATH entry, or
`FILESNAP_BIN` is required.

Plain `go build` / `go install` builds download the same pinned official npm
artifact on first use. The archive's SHA-512 and executable's SHA-256 must match
`internal/filehistory/binary/platforms.json`. The verified executable is cached
outside the workspace, with an atomic installation protected by a process lock.
Subsequent use works offline; failed downloads never start a protected prompt.
Release builds work offline from their first use.

For a local standalone build, run `go run ./internal/filehistory/cmd/bundle --platform host`, then `go build -tags=filesnap_bundle .` (or `task build`). The
release workflow prepares all six platform archives. Archives are generated,
ignored build inputs; the license and attribution notice are included in the
executable and extracted beside filesnap.

BSD, Android, and 32-bit platforms currently have no matching official filesnap
package. Automatic file history is disabled there so normal chat still works;
explicit rewind requests report that the platform is unsupported. An explicit
`option filesnap-binary` remains available for custom integrations.

The TUI command palette also exposes Rewind, Redo, and interrupted recovery.
Local and HTTP workspaces use the same server-owned transaction. SQLite messages
retain their IDs, tool results, and summary cursor; the next prompt reads the
restored history. A workspace lease excludes concurrent captured turns.

The older `crush file-history <session> ...` commands remain file-only diagnostics.
Use the in-chat commands for combined restoration.
