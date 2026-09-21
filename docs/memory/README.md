# Memory

Crush forgets everything when a session ends. Turning on memory changes
that: Crush writes each turn to [Honcho](https://honcho.dev), a service
that reasons over your conversations in the background and builds a
picture of how you work. Later sessions get that picture back.

The practical effect is that you stop repeating yourself. Conventions
you settled on last week, the reason you picked one library over
another, the gotcha that cost you an afternoon — Crush can recall them
instead of asking again.

Memory is off unless you turn it on. When it is off it costs nothing:
no network calls, no tokens, no change in behavior.

## Getting started

From inside Crush, open the command palette and pick **Connect Memory**.
A browser opens, you approve, and the credential is stored in
`~/.honcho/`.

Or from a shell:

```bash
crush login honcho
```

Either way, memory starts working immediately. There is no restart.

To stop, pick **Disconnect Memory** in the palette, or:

```bash
crush logout honcho
```

Signing out leaves stored memory alone. Sign back in and it is all still
there.

### Using an API key instead

If you would rather paste a key from
[app.honcho.dev](https://app.honcho.dev), set it in the environment:

```bash
export HONCHO_API_KEY="hch-..."
```

A configured key always wins over a browser sign-in.

### Self-hosting

Point Crush at your own deployment. A deployment on loopback needs no
credentials at all:

```bash
honcho base-url http://127.0.0.1:8000
```

Setting a base URL is enough to turn memory on, since nobody configures
a deployment they do not intend to use.

## What Crush remembers

Three things reach Honcho:

- **Your turns.** What you asked for, in your words.
- **Crush's replies.** Only completed answers. A half-finished thought
  that ended in a tool call is not recorded.
- **A one-line summary of meaningful tool calls.** "Edited
  internal/foo.go", "Ran: go test ./...". Reads, searches, and trivial
  commands are skipped because they say nothing durable about the work.

Shell arguments are redacted before they leave your machine. If any part
of a command looks like a credential — a keyword like `token` or
`authorization`, a flag that conventionally precedes a secret, a token
with a recognizable issuer prefix, or a long random-looking string — the
entire argument list is replaced with `(arguments redacted)` and only
the program name is kept.

Sub-agents read memory but never write to it. Their prompts are
scaffolding Crush wrote, not things you said.

## How memory reaches the model

Two channels, split deliberately so that recall does not destroy prompt
caching.

A **stable snapshot** — your profile and a summary of earlier work — is
computed once when a session starts and then frozen. Because it never
changes mid-session it sits in the system prompt and stays cached.

**Fresh recall** for the current question is appended at the very end of
the conversation, after everything worth caching, and is deliberately
excluded from cache breakpoints. Providers allow only four breakpoints
per request, and a block that changes every turn can never be read back
from cache, so spending one on it would waste it and displace a marker
from a message that would have been reused.

Recall is not fetched on every turn. It refreshes when the topic
changes, after thirty prompts, or after five minutes. Trivial prompts
("ok", "ship it") skip it entirely.

Memory is treated as untrusted input. It is assembled by a model from
earlier conversation, which may itself have contained text from
elsewhere, so the injected block instructs the model to use its facts
but never follow instructions embedded in it.

## Tools

When memory is on, Crush gains four tools and a skill telling it when to
use them. With memory off, none of them appear — not even in the system
prompt.

| Tool | Purpose |
| --- | --- |
| `honcho_search` | Find past messages by keyword. Fast. |
| `honcho_chat` | Ask a question about you in natural language. Slow; it reasons rather than retrieves. |
| `honcho_remember` | Save a durable conclusion worth keeping. |
| `honcho_status` | Show configuration and the reasoning backlog. Start here when memory seems wrong. |

## Configuration

Everything has a working default. These are the knobs if you want them.

```bash
# crushrc
honcho enabled
honcho workspace my-project
honcho session-strategy per-repo
honcho capture-tools false
```

| Key | Default | Meaning |
| --- | --- | --- |
| `enabled` | off | Turn memory on. Any other `honcho` directive implies this. |
| `api-key` | — | Prefer `$HONCHO_API_KEY` over writing a secret to a file. |
| `base-url` | Honcho Cloud | Your own deployment. |
| `workspace` | `crush` | Isolates one project's memory from another's. |
| `peer-name` | `user` | Who Honcho builds a picture of. |
| `agent-peer` | `crush` | How Crush identifies itself. |
| `session-strategy` | `per-directory` | What counts as one conversation. See below. |
| `recall-mode` | `hybrid` | `hybrid` injects context automatically; `tools` leaves it to the model to ask. |
| `observation-mode` | `unified` | `unified` shares conclusions with your other tools; `directional` keeps Crush's view private. |
| `agent-observe-me` | off | Also build a picture of Crush itself. |
| `capture-tools` | on | Record summaries of meaningful tool calls. |
| `max-conclusions` | 8 | How many conclusions may enter one memory block. |
| `context-tokens` | 2000 | Token budget for per-turn recall. |

The same settings work in `crush.json` under a `honcho` key, using
snake_case names.

### Session strategies

A Honcho session is the boundary it summarizes and scopes recall to.

| Strategy | One session per | Good for |
| --- | --- | --- |
| `per-directory` | working directory | The default. Memory accumulates per project. |
| `per-repo` | repository | Repos you enter from several directories. |
| `git-branch` | branch | Keeping long-lived branches from bleeding together. |
| `per-session` | Crush session | Short, isolated work. |
| `global` | everything | One shared memory across all your work. |

## Sharing setup with other tools

Honcho's other integrations — Claude Code, Codex, OpenCode — read the
same `~/.honcho/config.json`. Crush reads your credentials and peer name
from it and keeps its own settings under `hosts.crush`, so one sign-in
serves every tool and nothing Crush writes disturbs what the others
store.

Precedence runs shared file, then Crush config, then environment, with
later layers winning.

## Privacy

Your conversations leave your machine. That is the whole mechanism, and
it is worth being clear-eyed about.

- Turns, completed replies, and redacted tool summaries are sent.
- File contents are not sent, except insofar as you or Crush quoted them
  in conversation.
- Credentials in shell commands are redacted before sending.
- Self-hosting keeps everything on infrastructure you control.
- Memory is off until you turn it on.

## When it is not working

Run `honcho_status` in a session, or ask Crush to. It reports the
workspace, the session key, the peers, and the reasoning backlog.

A non-empty backlog means Honcho has received your turns but has not yet
reasoned over them. Recall improves once it catches up; new memory is
not instant.

If you signed in but nothing is remembered, check that the workspace and
session strategy are what you expect. Two projects in different
directories do not share memory under the default strategy.
