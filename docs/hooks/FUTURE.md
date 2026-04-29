# Hooks: Future Work

This document tracks planned features and design notes for hooks that are not
yet implemented. Nothing here is part of the current contract. Treat it as a
scratchpad for what's next, not as documentation of current behavior.

> [!NOTE]
> This document was largely LLM-generated.

## `context_files`

**Status:** planned, not implemented.

### Motivation

Today, a hook that wants to inject reference material into the agent's context
has exactly one knob: `context` (string or array of strings). Whatever the hook
puts there is concatenated into what the model sees. That's fine for short notes
("current branch: main", "scrubbed secrets") but it scales badly:

- Dumping a whole `README.md` or `package.json` into `context` burns tokens on
  every tool call where the hook fires.
- The model sees the file contents even if it doesn't need them.
- Large files can push the turn past the context window.

`context_files` is the lazy alternative: the hook returns **paths**, not
contents. Crush tells the agent the files exist and are relevant, and the agent
decides whether to open them with its existing `view` tool.

### Proposed shape

Additive envelope field. Accepts a list of strings:

```jsonc
{
  "decision": "allow",
  "context": "Scrubbed one secret",
  "context_files": ["README.md", "docs/ARCHITECTURE.md"],
}
```

Paths are resolved relative to `CRUSH_CWD`. Non-existent paths are dropped with
a debug log (don't fail the hook over a missing file).

### How the agent sees it

Crush appends a short note to the turn's context along the lines of:

```
## Referenced files
- README.md
- docs/ARCHITECTURE.md
```

No file contents are inlined. The agent opens them with `view` if it decides
they're relevant. This keeps cost proportional to need.

### Aggregation

Matches the existing rules for lists:

- Concatenates across matching hooks in config order.
- Deduplicates paths (same file referenced by two hooks → listed once).
- Dropped entirely if the final decision is `deny` or `halt`.

### Backwards compatibility

Purely additive. Hooks that don't emit `context_files` are unaffected. Existing
envelopes keep working unchanged. No version bump required.

### Open questions

- Should `context_files` paths be constrained to `CRUSH_PROJECT_DIR`? Probably
  yes, to avoid hooks smuggling in arbitrary filesystem reads.
- Do we want a per-file line range (`"README.md:1-40"`) or keep it dead simple
  (whole-file references only)? Start simple; add ranges only if asked for.
- Should we annotate "why this file is relevant" per entry? An object form
  (`{"path": "...", "reason": "..."}`) would allow that but complicates the
  schema. Defer until there's a real user need.

## Sub-agent opt-in

**Status:** not implemented.

### Background

Today hooks fire **only** on the top-level agent's tool calls. Sub-agents
(`agent` task tool, `agentic_fetch`, future delegated loops) run without hook
interception so a single delegated turn doesn't trigger the user's hook N times.

The outer sub-agent tool call itself is hooked, so blanket policy like "never
spawn sub-agents" or "rewrite prompts sent to the task agent" still works from
the coder's side. The sub-agent's inner loop is the part that's exempt.

### Why users might want the escape hatch

- Audit logging of every tool call, including delegated ones.
- Redaction hooks that want to apply uniformly regardless of who called the
  tool.
- Policy that cares about the _tool_ not the _caller_: "never fetch from this
  domain, even in `agentic_fetch`."

Until someone actually asks, don't ship this. YAGNI.

### Proposed shape

Additive, per-hook. Zero-value matches current default (skip sub-agents):

```jsonc
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "^bash$",
        "command": "./hooks/audit.sh",
        "include_sub_agents": true, // default false
      },
    ],
  },
}
```

Implementation changes where `wrapToolsWithHooks` decides to skip. Instead of a
single `isSubAgent` bailout, the runner filters per-hook matches by the hook's
`include_sub_agents` flag. Hooks that opt in get wrapped into sub-agent tool
slices too; everything else stays skipped.

### Backwards Compatibility

Purely additive. Hooks that don't set `include_sub_agents` get the default
(`false` = skip sub-agents). No wire format change, no version bump. The initial
transition from "hooks fire everywhere" to "hooks skip sub-agents by default"
was a one-time behavior change; adding the opt-in is pure addition.

### Side benefit: payload awareness

Extend the stdin payload with `"is_sub_agent": true|false` so hook scripts that
opt in can branch on caller type ("audit top-level and sub-agent calls
differently"). Also purely additive — hooks that don't read the field are
unaffected.

### Open questions

- Per-hook flag (above) vs a global `hooks.include_sub_agents` default? A global
  toggle is simpler but coarse-grained; per-hook is more flexible and
  composable. Start per-hook; a global default can be layered on later with
  explicit precedence ("per-hook overrides global").
- Does an opt-in hook see hooks from _nested_ sub-agents too (a sub-agent that
  itself calls a sub-agent)? Probably yes — once you've opted in you want the
  full tree. But call it out explicitly in docs so users aren't surprised by N²
  explosions on pathological configs.

## `UserPromptSubmit` event

**Status:** not implemented.

### Motivation

Today Crush supports exactly one hook event, `PreToolUse`. That's enough to gate
and rewrite tool calls but nothing else. The next-most-useful event is
`UserPromptSubmit`: fires after the user hits Enter but before the turn hits the
LLM. Lets hooks inject context, rewrite prompts, or gate on content without the
mutation complexity of `PostToolUse` (output scrubbing, error coercion, size
limits — all rabbit holes).

### Use cases

- Prepend project context the user didn't think to include ("current branch:
  `feat/x`; last commit: `<sha> <title>`").
- Point at reference files via `context_files` (when that lands) so the agent
  knows where to look without being force-fed contents.
- Redact secrets out of the prompt before it leaves the machine.
- Refuse prompts matching a policy ("don't send anything mentioning
  `production.env`") — with `deny` and a reason the user sees.
- Expand shorthand (`@TODO` → "please address the TODO in …").

### Proposed shape

Stdin payload extends the common envelope with the prompt:

```jsonc
{
  "event": "UserPromptSubmit",
  "session_id": "…",
  "cwd": "/home/user/project",
  "prompt": "fix the login flow",
  "attachments": ["screenshot.png"],
}
```

Output envelope reuses common fields plus one new per-event field,
`updated_prompt`:

```jsonc
{
  "decision": "allow", // optional; deny blocks the submission entirely
  "reason": "includes a production secret", // shown to the user when denying
  "context": "Current branch: feat/login",
  "updated_prompt": "fix the login flow\n\n(from @TODO on line 42)",
}
```

`updated_prompt` is a **full replacement** — not a merge patch — because a
prompt is a single string with no natural key structure. If multiple hooks emit
`updated_prompt`, later hooks in config order win.

### Aggregation

Reuses the universal rules:

- `halt` is sticky. Halts the whole turn before the LLM is called.
- `context` concatenates in config order.
- `updated_prompt`: last writer wins.
- `decision: "deny"` blocks the submission. The user sees `reason`; the turn
  never reaches the LLM.

### Differences from `PreToolUse`

- No `updated_input`: there are no tool inputs at this point.
- No permission-prompt bypass: there's no permission prompt for a user prompt.
- `decision: "allow"` is functionally identical to silence. It exists only for
  symmetry with `PreToolUse` and to give hook authors a consistent vocabulary.
  (Could be argued both ways — consider dropping it here.)
- Fires on every user submission, including follow-ups in the same session.
  Hooks should be fast; no subprocess-per-keystroke scenarios but the per-turn
  overhead is real.

### Implementation sketch

- New event constant `EventUserPromptSubmit` in `internal/hooks/hooks.go`.
- `Runner.Run` already takes an event name; no interface change.
- A new call site in `sessionAgent.Run` (or the coordinator's Run path) that
  fires hooks after creating the user message but before the first LLM call. If
  the aggregate decision is `deny` or `halt`, abort the turn and surface
  `reason` to the user.
- If hooks return `context`, prepend it to the prompt seen by the LLM (or attach
  as a system-message-level note — decide based on how the prompt is threaded
  through fantasy).
- If hooks return `updated_prompt`, replace the prompt body before the first LLM
  call. The message row in the DB should still store the _original_ prompt so
  the user sees what they typed; only the outbound version is rewritten. (Or:
  store both, show the original, send the rewritten — mirror how `updated_input`
  is handled today.)

### Open questions

- Store original vs rewritten prompt? Probably both, with UI showing original
  and a subtle indicator that a hook modified it.
- Do hooks fire on queued prompts too, or only when actually dispatched? If the
  user queues three prompts and the hook blocks the second, what happens to the
  third? Simplest rule: fire when dispatched; denial skips to the next queued
  prompt with a visible note.
- What about the `/commands` prefix? Does `UserPromptSubmit` fire for slash
  commands, or are those intercepted earlier? Probably earlier — hooks see only
  freeform prompts that would actually reach the LLM.

## Cross-platform shell (Windows support)

**Status:** not implemented.

### Problem

Today the hook runner uses `exec.Command("sh", "-c", hook.Command)`. On Windows
this fails without WSL or Git Bash on PATH. Even with `sh.exe` available,
Windows has no kernel shebang handling — `./hooks/foo.sh` can't be exec'd
directly the way it can on Unix. Hooks are effectively Unix-only.

### Approach

Keep the `command` field as a string. Tokenize it shell-style, examine
`argv[0]`, and branch:

- If `argv[0]` starts with `./`, `../`, `/`, or `~/` — treat it as a **file
  invocation**. Read the first ≤128 bytes, parse a shebang if present, and
  dispatch to the named interpreter via `os/exec`. Extra args from the command
  string pass through to the interpreter.
- Otherwise — treat the whole string as **shell code** and hand it to mvdan's
  in-process interpreter. mvdan resolves `node`, `bash`, `jq`, builtins,
  pipelines, redirects, etc. via its own exec handler.

No sentinel: a script with no shebang defaults to mvdan. A script with an
explicit shebang (`#!/bin/bash`, `#!/usr/bin/env python3`, etc.) uses the named
interpreter, which the user is responsible for having on PATH. Same contract on
every platform.

### Dispatch examples

| `command`                                | `argv[0]`      | Route                    |
| ---------------------------------------- | -------------- | ------------------------ |
| `ls -la`                                 | `ls`           | mvdan                    |
| `bash -c 'ls'`                           | `bash`         | mvdan (which execs bash) |
| `node ./script.js`                       | `node`         | mvdan (which execs node) |
| `./script.sh` (no shebang)               | `./script.sh`  | mvdan, fed the file      |
| `./script.sh` (`#!/bin/bash`)            | `./script.sh`  | `bash ./script.sh`       |
| `./script.py` (`#!/usr/bin/env python3`) | `./script.py`  | `python3 ./script.py`    |
| `./script.exe`                           | `./script.exe` | `os/exec` direct         |

### Contract on Windows

- Inline shell runs through mvdan natively. No external dependency.
- Shebang-dispatched scripts require the named interpreter on PATH (`bash.exe`,
  `python.exe`, `node.exe`, etc.). Crush does the dispatch that the Windows
  kernel won't.
- Shebang-less scripts run through mvdan regardless of extension. CRLF line
  endings are tolerated.

### Implementation sketch

- New function
  `dispatch(ctx, cmd string, env []string, stdin io.Reader) (stdout, stderr string, exitCode int, err error)`
  in `internal/hooks/`.
- Tokenize using mvdan's parser (already a dep) for consistent quoting/escape
  behavior with shell intuition.
- Path-prefix check on `argv[0]`; if path, read shebang with a bounded
  `io.LimitReader` and parse. Support:
  - `#!/absolute/interpreter args…`
  - `#!/usr/bin/env NAME` → resolve `NAME` on PATH
  - `#!/usr/bin/env -S NAME args…` → treat as above; `-S` is common enough to
    handle. Other `env` flags can error.
- Unified exit-code helper. mvdan's `interp.ExitStatus` and `os/exec`'s
  `ProcessState.ExitCode()` both become a single `int`.
- Context cancellation: mvdan's exec handler uses `exec.CommandContext` for its
  children, so a cancelled hook kills both the interpreter and any children.
  Verify with a `sleep 60` test.
- One fresh `interp.Runner` per hook invocation (parallel hooks must not share
  state).

### Swap the call site

`Runner.runOne` in `internal/hooks/runner.go` replaces its
`exec.Command("sh", "-c", …)` with a call to `dispatch(…)`. Everything
downstream (exit-code 2 / 49 / other dispatch, stdout JSON parsing,
stderr-as-reason) stays identical.

### Tests

Cross-platform matrix:

- Inline: `echo hi`; `exit 2`; pipelines; redirections.
- File, no shebang: treated as shell source through mvdan.
- File, `#!/bin/bash` on Unix — invokes system bash.
- File, `#!/usr/bin/env python3` — invokes python if present, skips if not.
- File, `#!/usr/bin/env -S node --foo` — extra flags preserved.
- File with CRLF line endings in the shebang.
- `./missing-file` — non-blocking error, hook proceeds as "no opinion".
- Timeout: hook that sleeps past its timeout gets killed; context cancellation
  kills the interpreter and its children.
- Concurrency: 10 hooks in parallel don't leak env/cwd/state between runners.
- Windows-specific: `./script.exe` exec'd directly; bash-shebang script fails
  gracefully when bash isn't on PATH.

### Pitfalls to watch

- **Userland shebang parsing is now our problem.** Edge cases around `env`
  flags, args with spaces, CRLF, missing interpreter. Well-trodden but needs
  real tests.
- **The path-prefix heuristic is a heuristic.** `relative/path.sh` (no leading
  `./`) gets mvdan'd, not file-dispatched. Matches shell intuition — at a bash
  prompt, `relative/path.sh` doesn't run unless `.` is on PATH — but worth
  documenting.
- **Kernel shebang handling is bypassed on Unix.** Today a chmod+x'd script is
  exec'd by the kernel; after this change, by our parser. Behavior should be
  byte-identical; verify with tests.
- **Two code paths.** mvdan vs direct-exec. Exit codes, stdin, signal
  propagation, env inheritance must be aligned. Discipline, not cleverness.

### Explicit non-goals

- No bundled `bash.exe` or `python.exe`. Users bring their own interpreters.
- No custom mvdan builtins (`crush_approve` etc.). Hooks stay portable and
  testable under bare `bash`.
- No `.sh`-extension filter on discovery. Hook file shape is driven by shebang,
  not filename.
- No Crush-as-script-interpreter mode (users can't write `#!/usr/bin/env crush`
  and have it mean something). If we want that later, it's an additive feature,
  not a dependency of this work.
