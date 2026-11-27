# Server API

The Crush server has an HTTP API. In [client/server mode](./README.md), the TUI
talks to the server through it, and other programs can do the same: an editor
plugin that runs Crush in a side panel, a script that hands the agent a task, or
a status bar item that lights up while the agent is working.

This page explains how the API fits together. For the details of each endpoint,
the server has its own reference at `/v1/docs/` (see [Connecting](#connecting)).

## Connecting

The server speaks HTTP/1.1, and HTTP/2 without TLS. By default it listens on a
Unix socket, and [Addresses](README.md#addresses) explains where. To reach it
with curl, use `--unix-socket`:

```bash
dir="${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}"
sock="${dir%/}/crush-$(id -u).sock"
[ ${#sock} -gt 104 ] && sock="/tmp/crush-$(id -u).sock"

curl --unix-socket "$sock" http://crush/v1/version
```

The host part of the URL doesn't matter when you go through a socket, so `crush`
is as good as anything. Every example on this page uses `$sock`.

If you'd rather not depend on the default path, start a server at an address you
pick:

```bash
crush server --host unix:///tmp/crush-dev.sock
```

For the exact body of every request and response, ask the server. `/v1/docs/`
returns an HTML reference, and `/v1/docs/openapi.json` returns an OpenAPI spec
you can feed to code generators. To read the HTML reference in a browser, save
it to a file and open that:

```bash
curl -s --unix-socket "$sock" http://crush/v1/docs/ > crush-api.html
```

If you want working code to read, `crush run` uses this API in client/server
mode. It lives in `internal/cmd/run.go`. The TUI's side is in
`internal/workspace/client_workspace.go`.

## Conventions

- Every route starts with `/v1`.
- Request and response bodies are JSON. The server doesn't look at
  `Content-Type`, so curl's `-d` works without extra headers.
- When a call has nothing to return, it answers `200` with an empty body.
  Sending a prompt answers `202`.
- Errors have a JSON body with a single field:

  ```json
  { "message": "workspace not found" }
  ```

  The one exception is a path or method the server doesn't know. Then you get a
  plain-text `404` or `405`.

These are the status codes you'll see:

| Status | Meaning                                                                                                                                                                    |
| ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `400`  | Something is wrong with the request: bad JSON, a missing or malformed `client_id`, or an agent that isn't set up yet.                                                      |
| `404`  | No such workspace. If it's one you opened, it has closed since then, or the server was replaced. Open it again instead of retrying. An unknown LSP name also gets a `404`. |
| `409`  | The request clashes with the current state: the agent is busy, the workspace is closing, you haven't opened your event stream yet, or your client ID was retired.          |
| `413`  | The body is too big. Only the theme endpoint has a limit, which is 64 KiB.                                                                                                 |
| `503`  | The server is shutting down. Start a new one and try again.                                                                                                                |
| `500`  | Anything else. Some bad requests end up here too, like a prompt with no text or a session ID that doesn't exist.                                                           |

## Client IDs

Pick a random UUID when your program starts, and use it for as long as the
program runs. The server uses it to keep track of who has which workspace open,
and which session each client is looking at. It goes in three places:

- the body of `POST /v1/workspaces`, as `client_id`
- the query string of the event stream, `current-session`, and
  `DELETE /v1/workspaces/{id}`, also as `client_id`
- the path of `DELETE /v1/clients/{client_id}`, when you leave

Don't share an ID between processes, and don't reuse one after you've retired
it. The server won't open workspaces for a retired ID.

## Workspaces

### Opening one

```bash
client_id=$(uuidgen)

curl --unix-socket "$sock" http://crush/v1/workspaces \
  -d "{\"path\": \"$PWD\", \"client_id\": \"$client_id\"}"
```

`path` and `client_id` are required. The response is the workspace. Here's one
without its `config` and `skills` fields, which are long:

```json
{
  "id": "87a8a3cb-354f-4372-a2cf-e1a2674da4fe",
  "path": "/home/me/src/app",
  "permission_mode": "normal",
  "data_dir": "/home/me/src/app/.crush",
  "version": "v0.101.0"
}
```

Hold on to the `id`, since every other workspace call needs it. The examples
below assume it's in `$ws`.

If the directory is already open, you get that workspace back as it is, and the
fields below are ignored, except for `channels`. If it isn't, these fields set
up the new workspace:

| Field             | What it does                                                                                                                     |
| ----------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `permission_mode` | `normal` asks before anything risky. `yolo` never asks, but blocks dangerous commands. `sysadmin` never asks and blocks nothing. |
| `data_dir`        | Where the project's data goes. Defaults to the `data-directory` config option, or `.crush` in the project.                       |
| `debug`           | Turns on the workspace's `debug` option.                                                                                         |
| `env`             | A list of `KEY=value` strings for commands run through the `shell` endpoint.                                                     |
| `version`         | Your Crush version. The server logs a warning if it doesn't match its own.                                                       |
| `channels`        | MCP servers to use as channels. If the directory is already open, this must match its list, or the request fails with a `500`.   |

### Staying connected

Right after opening a workspace, connect to its event stream:

```bash
curl -N --unix-socket "$sock" \
  "http://crush/v1/workspaces/$ws/events?client_id=$client_id"
```

Besides carrying events, the stream tells the server you're still around:

- After opening a workspace, you have 30 seconds to connect to the stream. If
  you don't, the server assumes you're gone.
- As long as your stream is open, the workspace stays open.
- If your stream drops, you have 10 seconds to reconnect before the server stops
  counting you. `CRUSH_SERVER_DETACH_GRACE` changes this (see
  [Options](./README.md#options)).
- Once nobody is left, the workspace closes, and that cancels whatever the agent
  is doing in it.

If a call returns `404` for your workspace, it's gone. Maybe the server
restarted. Send the same `POST /v1/workspaces` again and switch to the ID you
get back. That's what the Crush TUI does.

### Leaving

When you're done, tell the server. This releases every workspace your client has
open, and closes the ones nobody else is using:

```bash
curl --unix-socket "$sock" -X DELETE "http://crush/v1/clients/$client_id"
```

To let go of a single workspace instead, close its event stream and call
`DELETE /v1/workspaces/{id}?client_id=...`. Either way, you only give up your
own claim, so a workspace that other clients are using stays open. Both calls
are safe to repeat.

## Events

The event stream uses Server-Sent Events. Each event is a single `data:` line
followed by a blank line. The server sends no `event:` or `id:` lines and no
keepalives, so a quiet workspace sends nothing at all.

This is what the server sent when a session was created:

```json
data: {"type":"session","payload":{"type":"created","payload":{"id":"bdc714a6-84b5-4c51-af63-4bf598e526ac","parent_session_id":"","title":"Fix the flaky test","message_count":0,"prompt_tokens":0,"completion_tokens":0,"summary_message_id":"","cost":0,"created_at":1794904807,"updated_at":1794904807,"is_busy":false,"attached_clients":0}}}
```

There are two layers. The outer `type` says what the event is about, and the
inner `type` says what happened to it: `created`, `updated`, or `deleted`. The
inner `payload` is the thing itself, in the same shape the matching endpoint
returns. A `session` event, for example, carries the same object as
`GET /v1/workspaces/{id}/sessions/{sid}`. The one difference is that in a
`session` event, `is_busy` is always `false` and `attached_clients` is always
`0`. Fetch the session to get the real values.

### Event types

| Type                           | What it's about                                                                |
| ------------------------------ | ------------------------------------------------------------------------------ |
| `session`                      | A session was created, changed, or deleted.                                    |
| `message`                      | A message was created, changed, or deleted.                                    |
| `run_complete`                 | The agent finished a turn. See [Sending a prompt](#sending-a-prompt).          |
| `agent_event`                  | Notices from the agent, like errors, finished turns, and login requests.       |
| `permission_request`           | A tool call needs permission. See [Permissions](#permissions).                 |
| `permission_notification`      | A permission request was granted, denied, or cancelled.                        |
| `permission_mode_changed`      | Someone switched the workspace's permission mode.                              |
| `question_batch_request`       | The agent has questions for the user. See [Questions](#questions).             |
| `question_batch_notification`  | A batch of questions was answered or cancelled.                                |
| `pinentry_prompt_request`      | A command the agent ran needs a GPG passphrase or a security key PIN.          |
| `pinentry_prompt_notification` | That prompt was answered or cancelled.                                         |
| `ssh_prompt_request`           | A command the agent ran needs an SSH password or passphrase, or a yes or no.   |
| `ssh_prompt_notification`      | That prompt was answered or cancelled.                                         |
| `file`                         | A file the agent changed has a new version in the session's file history.      |
| `lsp_event`                    | An LSP server changed state, or its diagnostics changed.                       |
| `mcp_event`                    | An MCP server changed state, or its tools, prompts, or resources changed.      |
| `skills_event`                 | The set of available skills changed.                                           |
| `config_changed`               | The workspace config changed. Fetch the workspace again to see the new config. |
| `client_session`               | A client switched to another session.                                          |
| `host_theme`                   | A host application set or cleared the theme. See [Theme](#theme).              |
| `update_available`             | A newer version of Crush is out.                                               |

A `message` event always carries the whole message, not just what changed. While
the agent is writing, you'll get one `created` event and then a stream of
`updated` events for the same message ID, so replace your copy each time. A
message has a `role` (`user`, `assistant`, `tool`, or `system`) and a list of
`parts`. Each part has a `type`, such as `text`, `reasoning`, `tool_call`,
`tool_result`, or `finish`, and its content under `data`.

### When you connect

Right after you connect, the server sends the current theme, if a host has set
one, and a `client_session` event for each client that's looking at a session.
Nothing else is replayed, and events from while you were away are gone for good.
So after a reconnect, fetch whatever you show again, like the session list or
the messages in the open session.

### Prompts and sessions

Permission requests and questions, and the events that say how they ended, only
go to clients looking at the session that raised them. Prompts from a subagent
count as coming from the session that started the subagent. Password and
passphrase prompts are different: they go to every client in the workspace.

To tell the server which session you're looking at:

```bash
curl --unix-socket "$sock" \
  "http://crush/v1/workspaces/$ws/current-session?client_id=$client_id" \
  -d "{\"session_id\": \"$sid\"}"
```

An empty `session_id` means you aren't looking at any session, and you'll get no
permission requests or questions. A client that never calls this endpoint gets
all of them, which is usually what a script wants.

You need an open event stream before you can call this endpoint. Without one, it
returns `409`. When you pick a session that has a prompt waiting, the server
sends that prompt out again. Other clients get a copy too, so use the prompt's
`id` to skip ones you already have.

## Sending a prompt

If you don't have a session to add to, create one:

```bash
curl --unix-socket "$sock" "http://crush/v1/workspaces/$ws/sessions" \
  -d '{"title": "Fix the flaky test"}'
```

The response is the new session. Put its `id` in `$sid` and send the prompt:

```bash
curl --unix-socket "$sock" "http://crush/v1/workspaces/$ws/agent" \
  -d "{\"session_id\": \"$sid\", \"run_id\": \"run-1\", \"prompt\": \"Why does TestLogin fail on CI?\"}"
```

The server answers `202` right away, and the agent works in the background. You
follow along on the event stream. The reply comes in as `message` events, and
when the turn is over you get a `run_complete` event. Its payload looks like
this:

```json
{
  "session_id": "bdc714a6-84b5-4c51-af63-4bf598e526ac",
  "run_id": "run-1",
  "message_id": "4f6f2b8e-0d1c-4a57-9d3e-2a7c1b9e8f10",
  "text": "TestLogin compares timestamps from two different clocks..."
}
```

`error` is set when the turn failed, and `cancelled` is `true` when someone
cancelled it.

Set `run_id` to something unique, and match `run_complete` events against it. If
the session is busy when you send your prompt, the prompt waits in line, and the
first `run_complete` you see may belong to the turn ahead of yours.

To attach files, add an `attachments` list. Each attachment has `file_path`,
`file_name`, `mime_type`, and `content`, with the content base64-encoded.

The run belongs to the workspace, not to your HTTP request, so hanging up
doesn't stop it. To stop it, cancel it:

```bash
curl --unix-socket "$sock" -X POST \
  "http://crush/v1/workspaces/$ws/agent/sessions/$sid/cancel"
```

The agent can't do anything until a provider is configured.
`GET /v1/workspaces/{id}/agent` tells you where things stand: `is_ready` is
`true` once the agent is set up, `is_busy` is `true` while it's working, and
`model` is the model it uses. If you configure a provider through the API while
the workspace is open, call `POST /v1/workspaces/{id}/agent/init` to set the
agent up.

## Answering prompts

### Permissions

When a tool call needs permission, you get a `permission_request` event. This
one is for a shell command:

```json
{
  "id": "b8a6e1f2-6d0e-4c4b-9a39-1f0e7c2d5a44",
  "session_id": "bdc714a6-84b5-4c51-af63-4bf598e526ac",
  "tool_call_id": "call_8f2b1c",
  "tool_name": "bash",
  "description": "Execute command: go test ./...",
  "action": "execute",
  "params": {
    "description": "Run the tests",
    "command": "go test ./...",
    "working_dir": "/home/me/src/app",
    "run_in_background": false,
    "auto_background_after": 0
  },
  "path": "/home/me/src/app"
}
```

When a command looks risky, there's also a `danger` field that says why.

To answer, send the request back to
`POST /v1/workspaces/{id}/permissions/grant`, wrapped in an object with an
`action`. With the request (the event's inner `payload`) in `$req`, that's:

```bash
jq -n --argjson req "$req" '{permission: $req, action: "allow"}' |
  curl --unix-socket "$sock" -d @- \
    "http://crush/v1/workspaces/$ws/permissions/grant"
```

Send the whole request as you received it, not just its `id`. A request without
its `params` is rejected.

The action is `allow` for just this call, `allow_session` to allow calls like it
for the rest of the session, or `deny`. The response is `{"resolved": true}`, or
`{"resolved": false}` if another client answered first. Either way, the clients
looking at that session then get a `permission_notification` saying how it went.

### Questions

The agent can ask the user questions. They arrive in batches, as a
`question_batch_request`:

```json
{
  "id": "9b2f0c7e-3e4d-4b8a-8f61-0c5d2e7a1b93",
  "session_id": "bdc714a6-84b5-4c51-af63-4bf598e526ac",
  "tool_call_id": "call_51d0ae",
  "questions": [
    {
      "id": "q1",
      "type": "single_choice",
      "question": "Which database should the tests use?",
      "choices": [
        { "id": "sqlite", "label": "SQLite" },
        { "id": "postgres", "label": "PostgreSQL" }
      ]
    }
  ]
}
```

A question's `type` is `yes_no`, `single_choice`, `multi_choice`, or
`free_text`. Answer the whole batch at once with
`POST /v1/workspaces/{id}/questions/answer`:

```json
{
  "batch_request_id": "9b2f0c7e-3e4d-4b8a-8f61-0c5d2e7a1b93",
  "responses": [{ "request_id": "q1", "selected_ids": ["sqlite"] }]
}
```

Use `selected_ids` for choices, `fill_in_text` for free text, and `yes` for yes
or no questions. The server doesn't check `batch_request_id`. Your answer goes
to whatever batch is waiting at that moment, so don't answer a batch that's
already over. To dismiss a batch instead, call
`POST /v1/workspaces/{id}/questions/cancel` with no body. Both calls return
`{"resolved": true}`, or `false` if someone else got there first.

### Passwords and passphrases

When a command the agent runs asks for a GPG passphrase, a security key PIN, or
an SSH password, Crush catches the prompt and sends it to clients:

- `pinentry_prompt_request` is for GPG. Its `kind` is `passphrase` or `pin`.
- `ssh_prompt_request` is for SSH. Its `kind` is `password`, `confirm`, or
  `touch`.

Answer with the prompt's `id`:

```bash
curl --unix-socket "$sock" "http://crush/v1/workspaces/$ws/ssh/answer" \
  -d '{"request_id": "<id>", "secret": "<password>"}'
```

GPG prompts go to `pinentry/answer` instead. To decline either kind, send
`{"request_id": "<id>"}` to `ssh/cancel` or `pinentry/cancel`.

A `confirm` prompt is a yes or no question, like whether to trust a new host
key. Send `yes` as the secret to accept it, or cancel to decline. A `touch`
prompt means the user needs to touch their security key. The TUI answers those
with `yes` right away and shows the user a notice, and your client should do the
same.

## Host applications

If you run Crush inside another program, like an editor with Crush in one of its
panes, these endpoints help the two work together.

### Theme

`POST /v1/workspaces/{id}/theme` gives Crush a theme from the host, so Crush can
match the editor's colors. Every client of the workspace switches to it.

```bash
curl --unix-socket "$sock" "http://crush/v1/workspaces/$ws/theme" -d '{
  "theme": {
    "name": "Tokyo Night",
    "base": "charmtone-panther",
    "palette": {"primary": "#7aa2f7", "bg_base": "#1a1b26"},
    "syntax": {"keyword": "#bb9af7", "literal_string": "#9ece6a"}
  }
}'
```

- `base` is a built-in or user theme that supplies every color you leave out. It
  defaults to `charmtone-panther`.
- `palette` uses the same keys as a
  [theme file](../../README.md#editing-themes), like `primary` and `bg_base`.
- `syntax` maps code highlighting tokens, like `keyword` and `literal_string`,
  to colors.
- `transparent`, if you set it, overrides the user's transparent background
  setting.
- `name` and `source` are only labels. Put your application's name in `source`.

Colors can be `#rgb`, `#rrggbb`, an ANSI color number from 0 to 255, or a
CharmTone color name. Unknown keys are ignored. An invalid color or an unknown
`base` gets a `400`, and the message says which one. Send `{"theme": null}` to
go back to Crush's own theme.

The server keeps the theme in memory only. Everyone gets a `host_theme` event
when it changes, and clients that connect later get the current theme as soon as
they open their event stream.

### Host context

`POST /v1/workspaces/{id}/host-context` tells the agent what the user has open
in the host:

```json
{
  "open_files": ["internal/server/server.go", "README.md"],
  "active_file": "internal/server/server.go",
  "line": 42
}
```

The agent gets this with every prompt, so when the user says "this file" or
"here", the agent knows what they mean. The TUI's file completions also list the
open files first. Send an empty body to clear it. The agent has to be set up
before you can set the context. Until then, this endpoint returns `400`.

### Notes

`POST /v1/workspaces/{id}/agent/sessions/{sid}/notes` leaves a note for the
agent:

```json
{ "text": "user did not like the edit." }
```

A note doesn't start a run and doesn't show up in the chat. The agent reads it
along with the next prompt in that session. Use it to tell the agent about
things the user did in the host since its last turn.

## Endpoints

These are the endpoints, grouped by what they're for. For request and response
bodies, see `/v1/docs/` on a running server.

Paths that start with `{id}/` are under `/v1/workspaces/`. For example,
`{id}/events` means `/v1/workspaces/{id}/events`.

### Server

| Method   | Path                      | What it does                                                                                                                                                                     |
| -------- | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET`    | `/v1/health`              | Answers `200` when the server is up.                                                                                                                                             |
| `GET`    | `/v1/version`             | Version, commit, build ID, Go version, and platform.                                                                                                                             |
| `POST`   | `/v1/control`             | `{"command": "shutdown_if_idle"}` shuts the server down if no workspaces are open, and returns `409` if any are open or opening. `shutdown` is an older name for the same thing. |
| `GET`    | `/v1/config`              | The server's own config.                                                                                                                                                         |
| `DELETE` | `/v1/clients/{client_id}` | Retires a client and releases everything it holds.                                                                                                                               |
| `GET`    | `/v1/docs/`               | This API as an HTML page.                                                                                                                                                        |
| `GET`    | `/v1/docs/openapi.json`   | This API as an OpenAPI spec.                                                                                                                                                     |

### Workspaces

| Method   | Path                   | What it does                                                         |
| -------- | ---------------------- | -------------------------------------------------------------------- |
| `GET`    | `/v1/workspaces`       | Lists open workspaces.                                               |
| `POST`   | `/v1/workspaces`       | Opens a workspace, or returns the one already open at that path.     |
| `GET`    | `/v1/workspaces/{id}`  | Returns one workspace.                                               |
| `DELETE` | `/v1/workspaces/{id}`  | Releases your claim on a workspace. Needs `client_id`.               |
| `GET`    | `{id}/events`          | The event stream. Needs `client_id`.                                 |
| `POST`   | `{id}/current-session` | Tells the server which session you're looking at. Needs `client_id`. |
| `GET`    | `{id}/config`          | The workspace's resolved config.                                     |
| `GET`    | `{id}/providers`       | The providers available in the workspace.                            |
| `GET`    | `{id}/messages/user`   | The user's messages from every session.                              |
| `GET`    | `{id}/git/branch`      | The project's current Git branch.                                    |

### Sessions

| Method   | Path                                    | What it does                                               |
| -------- | --------------------------------------- | ---------------------------------------------------------- |
| `GET`    | `{id}/sessions`                         | Lists sessions.                                            |
| `POST`   | `{id}/sessions`                         | Creates a session.                                         |
| `GET`    | `{id}/sessions/{sid}`                   | Returns one session.                                       |
| `PUT`    | `{id}/sessions/{sid}`                   | Replaces a session. Send the whole session, with its `id`. |
| `DELETE` | `{id}/sessions/{sid}`                   | Deletes a session.                                         |
| `GET`    | `{id}/sessions/{sid}/messages`          | The session's messages.                                    |
| `GET`    | `{id}/sessions/{sid}/messages/user`     | Only the user's messages.                                  |
| `GET`    | `{id}/sessions/{sid}/history`           | Versions of the files the agent changed.                   |
| `GET`    | `{id}/sessions/{sid}/filetracker/files` | Files the agent has read.                                  |
| `POST`   | `{id}/filetracker/read`                 | Records that a file was read.                              |
| `GET`    | `{id}/filetracker/lastread`             | When a file was last read. Takes `path` and `session_id`.  |

### Agent

| Method | Path                                       | What it does                                                                                    |
| ------ | ------------------------------------------ | ----------------------------------------------------------------------------------------------- |
| `GET`  | `{id}/agent`                               | Whether the agent is ready or busy, and which model it uses.                                    |
| `POST` | `{id}/agent`                               | Sends a prompt. Answers `202`.                                                                  |
| `POST` | `{id}/agent/init`                          | Sets the agent up.                                                                              |
| `POST` | `{id}/agent/update`                        | Reloads the agent's model settings.                                                             |
| `POST` | `{id}/agent/main`                          | Switches between the `coder` and `plan` agents. Returns `409` while the agent is busy.          |
| `GET`  | `{id}/agent/default-small-model`           | The default small model for a provider. Takes `provider_id`.                                    |
| `GET`  | `{id}/agent/sessions/{sid}`                | The session, plus whether the agent is busy in it.                                              |
| `POST` | `{id}/agent/sessions/{sid}/cancel`         | Cancels what the agent is doing in the session.                                                 |
| `GET`  | `{id}/agent/sessions/{sid}/prompts/queued` | How many prompts are waiting.                                                                   |
| `GET`  | `{id}/agent/sessions/{sid}/prompts/list`   | The waiting prompts.                                                                            |
| `POST` | `{id}/agent/sessions/{sid}/prompts/clear`  | Throws the waiting prompts away.                                                                |
| `POST` | `{id}/agent/sessions/{sid}/notes`          | Leaves a note for the next prompt.                                                              |
| `POST` | `{id}/agent/sessions/{sid}/summarize`      | Summarizes the session so it takes up less context.                                             |
| `POST` | `{id}/agent/sessions/{sid}/shell`          | Runs a shell command in the project, like `!` in the TUI, and returns its output and exit code. |

### Permissions and prompts

| Method | Path                     | What it does                                                                                   |
| ------ | ------------------------ | ---------------------------------------------------------------------------------------------- |
| `POST` | `{id}/permissions/grant` | Answers a permission request.                                                                  |
| `POST` | `{id}/permissions/mode`  | Sets the permission mode to `normal`, `yolo`, or `sysadmin`. Anything else counts as `normal`. |
| `POST` | `{id}/questions/answer`  | Answers a batch of questions.                                                                  |
| `POST` | `{id}/questions/cancel`  | Dismisses a batch of questions.                                                                |
| `POST` | `{id}/pinentry/answer`   | Answers a GPG prompt.                                                                          |
| `POST` | `{id}/pinentry/cancel`   | Declines a GPG prompt.                                                                         |
| `POST` | `{id}/ssh/answer`        | Answers an SSH prompt.                                                                         |
| `POST` | `{id}/ssh/cancel`        | Declines an SSH prompt.                                                                        |

### Host

| Method        | Path                | What it does                                     |
| ------------- | ------------------- | ------------------------------------------------ |
| `GET`, `POST` | `{id}/theme`        | Gets or sets the host theme.                     |
| `GET`, `POST` | `{id}/host-context` | Gets or sets what the user has open in the host. |

### Config

| Method | Path                         | What it does                        |
| ------ | ---------------------------- | ----------------------------------- |
| `POST` | `{id}/config/set`            | Sets a config field.                |
| `POST` | `{id}/config/remove`         | Removes a config field.             |
| `POST` | `{id}/config/model`          | Sets the preferred model.           |
| `POST` | `{id}/config/compact`        | Turns compact mode on or off.       |
| `POST` | `{id}/config/provider-key`   | Sets a provider's API key.          |
| `POST` | `{id}/config/import-copilot` | Imports GitHub Copilot credentials. |
| `POST` | `{id}/config/refresh-oauth`  | Refreshes a provider's OAuth token. |

### Project

| Method | Path                       | What it does                                          |
| ------ | -------------------------- | ----------------------------------------------------- |
| `GET`  | `{id}/project/needs-init`  | Whether Crush should offer to initialize the project. |
| `POST` | `{id}/project/init`        | Marks the project as initialized.                     |
| `GET`  | `{id}/project/init-prompt` | The prompt Crush uses to initialize a project.        |

### LSP

| Method | Path                          | What it does                       |
| ------ | ----------------------------- | ---------------------------------- |
| `GET`  | `{id}/lsps`                   | Lists LSP servers and their state. |
| `GET`  | `{id}/lsps/{lsp}/diagnostics` | Diagnostics from one LSP server.   |
| `POST` | `{id}/lsps/start`             | Starts an LSP server.              |
| `POST` | `{id}/lsps/stop`              | Stops all LSP servers.             |

### MCP

| Method | Path                         | What it does                                                                    |
| ------ | ---------------------------- | ------------------------------------------------------------------------------- |
| `GET`  | `{id}/mcp/states`            | The state of each MCP server.                                                   |
| `POST` | `{id}/mcp/start`             | Starts an MCP server.                                                           |
| `POST` | `{id}/mcp/refresh-tools`     | Reloads an MCP server's tools.                                                  |
| `POST` | `{id}/mcp/refresh-prompts`   | Reloads an MCP server's prompts.                                                |
| `POST` | `{id}/mcp/refresh-resources` | Reloads an MCP server's resources.                                              |
| `GET`  | `{id}/mcp/prompts`           | Lists MCP prompts.                                                              |
| `POST` | `{id}/mcp/get-prompt`        | Fetches one MCP prompt.                                                         |
| `POST` | `{id}/mcp/read-resource`     | Reads an MCP resource.                                                          |
| `GET`  | `{id}/mcp/pending-auth`      | MCP servers waiting for an OAuth login.                                         |
| `GET`  | `{id}/mcp/auth-url`          | The OAuth URL for an MCP server while its login is in progress. Takes `name`.   |
| `POST` | `{id}/mcp/auth`              | Runs the OAuth login for an MCP server. Doesn't return until the login is over. |
| `GET`  | `{id}/mcp/disabled`          | MCP servers turned off for this project.                                        |
| `POST` | `{id}/mcp/disabled`          | Turns an MCP server on or off for this project.                                 |
| `GET`  | `{id}/mcp/enabled`           | MCP servers turned off in the config but turned on for this project.            |
| `POST` | `{id}/mcp/config-disabled`   | Turns an MCP server on or off in the global config.                             |
| `POST` | `{id}/mcp/docker/enable`     | Turns on the Docker MCP server.                                                 |
| `POST` | `{id}/mcp/docker/disable`    | Turns off the Docker MCP server.                                                |

### Skills

| Method | Path                          | What it does                                                    |
| ------ | ----------------------------- | --------------------------------------------------------------- |
| `GET`  | `{id}/skills`                 | Lists the skills the agent can see.                             |
| `POST` | `{id}/skills/read`            | Returns a skill's content.                                      |
| `GET`  | `{id}/skills/disabled`        | Skills turned off for this project.                             |
| `POST` | `{id}/skills/disabled`        | Turns a skill on or off for this project.                       |
| `GET`  | `{id}/skills/enabled`         | Skills turned off in the config but turned on for this project. |
| `POST` | `{id}/skills/config-disabled` | Turns a skill on or off in the global config.                   |
