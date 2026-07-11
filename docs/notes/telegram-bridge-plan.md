# Telegram Remote-Control Bridge — Implementation Plan

**Status**: ready to implement
**Target**: a new `crush telegram` subcommand that lets a single authorized
Telegram chat drive a Crush agent remotely: send prompts, watch progress,
approve/deny permission requests, manage sessions, and cancel runs.

This plan is self-contained. Every fact about the codebase you need is in
§2 (Facts) and §10 (Pitfalls) with `file:line` references — trust them, they
were verified against the current tree. Do NOT re-architect; implement as
specified. When something is genuinely ambiguous, prefer the smallest change
consistent with the patterns in `internal/cmd/run.go`.

---

## 1. Architecture overview

Crush has a client/server split (default on; set `CRUSH_CLIENT_SERVER=0` for legacy local mode):
`crush server` exposes workspaces/sessions/agent/permissions over HTTP on a
unix socket, and the TUI / `crush run` are just clients. The bridge is
**another client process** — zero changes to the server, TUI, or agent.

```
Telegram cloud ──long poll──▶ crush telegram (bridge) ──unix socket──▶ crush server ──▶ agent
     ▲                            │        ▲
     └── sendMessage/keyboards ───┘        └── SSE events (messages, permissions, run-complete)
```

Two goroutine "pumps" + shared state:

- **update pump**: long-polls Telegram `getUpdates`, authenticates the chat,
  handles commands (`/new`, `/sessions`, `/use`, `/cancel`, `/status`,
  `/help`), and turns plain text into `client.SendMessage` calls with a fresh
  `runID`.
- **event pump**: consumes `client.SubscribeEvents` (SSE), reacts to
  `proto.PermissionRequest` (inline keyboard), `proto.RunComplete` (final
  answer), `proto.AgentEvent` errors; reconnects when the channel closes.
- **state** (mutex-protected struct): active session ID, set of in-flight
  runIDs we own, pending permission requests (permID → request + Telegram
  message ID), last `getUpdates` offset, cached session list for `/use`.

### New files (only these; do not modify anything else except go.mod — which
stays untouched, there are **no new dependencies**):

| File | Purpose |
|---|---|
| `internal/cmd/telegram.go` | cobra command; flag/env resolution; reuses `connectToServer` (same package); hands off to `telegram.Run` |
| `internal/telegram/api.go` | minimal hand-rolled Telegram Bot API client (stdlib `net/http` only) |
| `internal/telegram/api_test.go` | tests against `httptest.Server` fake Telegram |
| `internal/telegram/bridge.go` | the bridge: state, both pumps, crush-client interaction |
| `internal/telegram/bridge_test.go` | tests with fake Telegram + fake crush server (both `httptest`) |
| `internal/telegram/render.go` | text chunking, HTML escaping, permission summaries, diff truncation |
| `internal/telegram/render_test.go` | table tests for rendering |
| `docs/notes/telegram-bridge.md` | short user-facing setup doc (BotFather, chat ID, env vars) |

Why hand-rolled Telegram client: we need exactly 7 endpoints of a simple,
very stable JSON API (fully specified in §5); this avoids a new dependency
and keeps the build pure-Go (`CGO_ENABLED=0` per AGENTS.md).

---

## 2. Facts about the codebase (verified; use as reference, do not re-derive)

Module `github.com/charmbracelet/crush`, Go 1.26.5. Logger: stdlib `slog`
(setup helper `internal/log/log.go:23` — `crushlog.Setup(logFile, debug, ws...)`,
a `sync.Once`). Testing: testify `require`, `t.Parallel()`, `t.TempDir()`.

### 2.1 Connecting to the server (all in package `internal/cmd`, reusable directly)

- `connectToServer(cmd)` — `internal/cmd/root.go:363-413`. Does everything:
  parses `--host` (default `server.DefaultHost()`), auto-spawns a detached
  `crush server` if the socket is missing/stale (`ensureServer`,
  `root.go:419-527`), builds `client.NewClient(cwd, scheme, host)`, then
  `c.CreateWorkspace(ctx, proto.Workspace{Path: cwd, DataDir, Debug, YOLO,
  Version: version.Version, Env: os.Environ()})`. Returns
  `(c *client.Client, ws *proto.Workspace, cleanup func(), err)`. `cleanup`
  calls `DeleteWorkspace` — defer it.
- Workspace = one project dir on the shared per-user server; create-or-attach
  is idempotent by resolved path (`internal/backend/backend.go:234-353`).
  **Every client method's `id` parameter is the workspace ID (`ws.ID`).**
- `ResolveCwd(cmd)` — `root.go:828-842` (used by `connectToServer` internally).
- No auth on the socket; access control is unix file permissions.
- Config check: `ws.Config.IsConfigured()` — if false, exit with an error
  telling the user to run `crush` once to configure a provider/model
  (pattern: `internal/cmd/run.go:117`).
- Readiness: poll `c.GetAgentInfo(ctx, ws.ID)` every 200 ms up to 30 s until
  `info.IsReady`, then call `c.UpdateAgent(ctx, ws.ID)` once (loads MCP
  tools). Pattern: `waitForAgent`, `internal/cmd/run.go:450-468`. Also call
  `c.InitiateAgentProcessing(ctx, ws.ID)` once at startup
  (`internal/client/proto.go:505`).

### 2.2 Client API surface used by the bridge (`internal/client/proto.go`)

```go
SubscribeEvents(ctx, wsID) (<-chan any, error)                         // :114
SendMessage(ctx, wsID, sessionID, runID, prompt string,
            attachments ...message.Attachment) error                  // :421
GetAgentInfo(ctx, wsID) (*proto.AgentInfo, error)                      // :385
UpdateAgent(ctx, wsID) error                                           // :402
InitiateAgentProcessing(ctx, wsID) error                               // :505
CreateSession(ctx, wsID, title string) (*proto.Session, error)         // :569
ListSessions(ctx, wsID) ([]proto.Session, error)                       // :586
GetAgentSessionInfo(ctx, wsID, sessionID) (*proto.AgentSession, error) // :475
CancelAgentSession(ctx, wsID, sessionID) error                         // :742
GrantPermission(ctx, wsID, req proto.PermissionGrant) (bool, error)    // :607
GetAgentSessionQueuedPrompts(ctx, wsID, sessionID) (int, error)        // :355
```

### 2.3 Event stream

`SubscribeEvents` returns a buffered (cap 100) `<-chan any`. Each value is a
**`pubsub.Event[T]` value** (not pointer): `struct { Type EventType; Payload T }`
with `Type` ∈ `"created" | "updated" | "deleted"`
(`internal/pubsub/events.go:9-52`). Concrete `T`s the bridge cares about:

| Channel value | Meaning for bridge |
|---|---|
| `pubsub.Event[proto.PermissionRequest]` (`created`) | show approval keyboard |
| `pubsub.Event[proto.PermissionNotification]` | request resolved (maybe by TUI) — update keyboard message |
| `pubsub.Event[proto.RunComplete]` | **authoritative end-of-turn**; deliver final text |
| `pubsub.Event[proto.AgentEvent]` | if `.Payload.Error != nil` → report failure |
| `pubsub.Event[proto.Message]` | streaming updates; v1 ignores except optional status (M7) |
| `pubsub.Event[proto.Session]` | title/usage updates (ignore in v1) |

Others (`LSPEvent`, `MCPEvent`, `File`, `ConfigChanged`, `SkillsEvent`,
`UpdateAvailable`) — ignore with a `default:` branch.

**The stream is workspace-scoped, NOT session-scoped** — filter by
`SessionID` yourself. **No replay on reconnect** and the broker is lossy
under back-pressure — after every (re)subscribe, resync state via REST
(`GetAgentInfo`, `GetAgentSessionInfo`). When the channel closes (server
restart, EOF), re-call `SubscribeEvents` in a loop with backoff
(1s, 2s, 4s… cap 30s); notify the chat once ("⚠️ lost server connection,
reconnecting…") and once on success.

### 2.4 Prompt → response lifecycle

1. Mint `runID := uuid.New().String()` (`github.com/google/uuid`, already a dep).
2. `SendMessage(ctx, ws.ID, sessID, runID, text)` → server returns 202; run
   is decoupled from the HTTP call.
3. Wait for `pubsub.Event[proto.RunComplete]` where `Payload.RunID == runID`:

```go
type RunComplete struct {                     // internal/proto/proto.go:75-82
    SessionID string `json:"session_id"`
    RunID     string `json:"run_id,omitempty"`
    MessageID string `json:"message_id"`
    Text      string `json:"text,omitempty"`   // authoritative final assistant text
    Error     string `json:"error,omitempty"`
    Cancelled bool   `json:"cancelled,omitempty"`
}
```

- `Error != "" && !Cancelled` → send "❌ run failed: <Error>".
- `Cancelled` → send "🛑 run cancelled."
- else → send `Text` (chunked, §6.3). If `Text == ""` send "✅ done (no text output)."
- **Never** infer completion from `proto.Message` `Finish` parts — a Finish
  part with reason `tool_use` fires on *every tool step*
  (see comment at `internal/cmd/run.go:376-380`).
- Sending while the session is busy **queues** the prompt (not an error) —
  see §2.6 `IsBusy`; tell the user "⏳ queued behind the current run."

### 2.5 Permissions

Arrives as `pubsub.Event[proto.PermissionRequest]` with `Type == "created"`:

```go
type PermissionRequest struct {               // internal/proto/permission.go:26-35
    ID          string `json:"id"`
    SessionID   string `json:"session_id"`
    ToolCallID  string `json:"tool_call_id"`
    ToolName    string `json:"tool_name"`
    Description string `json:"description"`
    Action      string `json:"action"`
    Params      any    `json:"params"`   // concrete type by ToolName, see table
    Path        string `json:"path"`
}
```

`Params` is already decoded to a concrete type by `UnmarshalJSON`
(`permission.go:40-58`); type-assert on it. Types are aliases in
`internal/proto/tools.go` of `internal/agent/tools` structs:

| ToolName | Params type | Key fields to render |
|---|---|---|
| `"bash"` | `proto.BashPermissionsParams` | `.Command`, `.WorkingDir`, `.RunInBackground` |
| `"edit"` | `proto.EditPermissionsParams` | `.FilePath`, `.OldContent`, `.NewContent` |
| `"write"` | `proto.WritePermissionsParams` | `.FilePath`, `.OldContent`, `.NewContent` |
| `"multiedit"` | `proto.MultiEditPermissionsParams` | `.FilePath`, `.OldContent`, `.NewContent` |
| `"download"` | `proto.DownloadPermissionsParams` | `.URL`, `.FilePath` |
| `"fetch"` | `proto.FetchPermissionsParams` | `.URL`, `.Format` |
| `"agentic_fetch"` | `proto.AgenticFetchPermissionsParams` | `.URL`, `.Prompt` |
| `"view"` | `proto.ViewPermissionsParams` | `.FilePath` |
| `"ls"` | `proto.LSPermissionsParams` | `.Path` |
| anything else | `map[string]any` | render `Description` + compact JSON of params |

To resolve, POST back the **full request you received**:

```go
resolved, err := c.GrantPermission(ctx, ws.ID, proto.PermissionGrant{
    Permission: req,                            // the whole PermissionRequest
    Action:     proto.PermissionAllow,          // | PermissionAllowForSession | PermissionDeny
})
```

`resolved == false` is **not an error** — someone else (e.g. the TUI)
resolved it first; edit the Telegram message to "↩️ already handled elsewhere".
Constants: `proto.PermissionAllow "allow"`, `proto.PermissionAllowForSession
"allow_session"`, `proto.PermissionDeny "deny"` (`internal/proto/proto.go:175-179`).

Deny does not abort with a special finish reason: the tool returns
"User denied permission" (`IsError=true`) and the turn ends normally — the
bridge just sees a normal `RunComplete`.

**There is no permission-cancelled event.** If a run ends (RunComplete /
AgentEvent error for that session) while keyboards are pending, edit those
messages to "⌛ expired" and drop them from the pending map. Also handle
`proto.PermissionNotification` (`permission.go:19-23`, fields `ToolCallID`,
`Granted`, `Denied`): if a pending request with that `ToolCallID` was
resolved elsewhere, update its message (✅ allowed / 🚫 denied) and remove it.

### 2.6 Sessions & busy state

`proto.Session` (`internal/proto/session.go:16-30`): `ID`, `Title`,
`MessageCount`, `PromptTokens`, `CompletionTokens`, `Cost` (USD float64),
`UpdatedAt` (unix). **`IsBusy`/`AttachedClients` are only populated on REST
reads, never on SSE events** — for busy state use
`GetAgentSessionInfo(...).IsBusy` or `GetAgentInfo(...).IsBusy`.

### 2.7 Project conventions (AGENTS.md — enforced by lint)

- Format with `gofumpt -w .` (or `task fmt`); imports grouped stdlib/external/internal.
- Tests: testify **`require`**, `t.Parallel()`, `t.TempDir()`, `t.Setenv`.
- JSON tags snake_case. File perms octal `0o644` style.
- **Log messages must start with a capital letter** (`task lint:log` enforces).
- Comments end with periods; wrap at 78 columns.
- Errors: wrap with `fmt.Errorf("...: %w", err)`. `context.Context` first param.
- Semantic commits, one line (`feat: ...`).
- Build `go build .`; test `task test` or `go test -race ./...`; lint `task lint:fix`.

---

## 3. CLI: `internal/cmd/telegram.go`

Follow the registration style of `internal/cmd/server.go:20-99` (own file,
own `init()` with `rootCmd.AddCommand(telegramCmd)`).

```go
var (
    telegramToken  string
    telegramChatID int64
)

func init() {
    telegramCmd.Flags().StringVar(&telegramToken, "token", "", "Telegram bot token (or $CRUSH_TELEGRAM_BOT_TOKEN)")
    telegramCmd.Flags().Int64Var(&telegramChatID, "chat-id", 0, "Authorized Telegram chat ID (or $CRUSH_TELEGRAM_CHAT_ID)")
    rootCmd.AddCommand(telegramCmd)
}
```

`RunE` flow:

1. Resolve token: flag, else `os.Getenv("CRUSH_TELEGRAM_BOT_TOKEN")`; empty → error
   `telegram bot token required: pass --token or set CRUSH_TELEGRAM_BOT_TOKEN`.
2. Resolve chat ID: flag, else `strconv.ParseInt(os.Getenv("CRUSH_TELEGRAM_CHAT_ID"), 10, 64)`;
   zero → error explaining how to obtain it (message `@userinfobot`, or start
   the bridge with a wrong ID and read the logged rejected chat ID).
3. Logging: `crushlog.Setup(filepath.Join(config.GlobalCacheDir(), "telegram", "crush.log"), debug, os.Stderr)`
   (mirror `server.go`'s pattern; `debug` from persistent `--debug` flag).
4. `c, ws, cleanup, err := connectToServer(cmd)`; `defer cleanup()`.
5. `if !ws.Config.IsConfigured() { return fmt.Errorf("crush is not configured; run 'crush' once to set up a provider") }`
6. Signal handling: `ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)`.
7. `return telegram.Run(ctx, telegram.Options{Client: c, Workspace: *ws, Token: telegramToken, ChatID: telegramChatID})`

The command must NOT expose or touch YOLO / skip-permissions in any way.

---

## 4. Bridge core: `internal/telegram/bridge.go`

```go
type Options struct {
    Client    *client.Client
    Workspace proto.Workspace
    Token     string
    ChatID    int64
    // BaseURL overrides the Telegram API base for tests. Default
    // "https://api.telegram.org".
    BaseURL string
}

func Run(ctx context.Context, opts Options) error
```

`Run`:

1. Build `*api` (Telegram client, §5) and `getMe` to validate the token
   (fail fast with a clear error on 401).
2. Agent readiness: `InitiateAgentProcessing`, then poll `GetAgentInfo`
   every 200 ms (≤30 s) until `IsReady`, then `UpdateAgent` (§2.1). If not
   ready in time → error.
3. Pick initial session: `ListSessions`; choose the one with the greatest
   `UpdatedAt`, else `CreateSession(ctx, wsID, "telegram")`.
4. Send startup message to the chat:
   `🟢 crush bridge online\nproject: <ws.Path>\nsession: <title>` .
5. Start event pump goroutine and typing-indicator ticker (§4.3); run the
   update pump in the current goroutine; on ctx cancel, send
   `🔴 bridge shutting down` (best-effort, 2 s timeout) and return.

Internal state (one struct, `sync.Mutex`):

```go
type bridge struct {
    mu         sync.Mutex
    opts       Options
    tg         *api
    sessionID  string                      // the "active" session (plain text routes here)
    titles     map[string]string           // session ID -> title (refreshed from ListSessions/Session events)
    runs       map[string]string           // runID -> session ID, awaiting RunComplete
    pendPerms  map[string]pendingPerm      // perm request ID -> request + tg message id
    sessions   []proto.Session             // cache for /sessions -> /use indices
    msgSession map[int64]string            // outbound tg message ID -> session ID (reply routing)
    msgOrder   []int64                     // FIFO of msgSession keys; evict oldest past 500
}
type pendingPerm struct {
    req   proto.PermissionRequest
    msgID int64
}
```

### 4.1 Update pump (Telegram → crush)

Loop: `updates, err := tg.getUpdates(ctx, offset, 50 /*sec*/)`; on error
log + sleep 3 s + continue; for each update set `offset = u.UpdateID + 1`.

**Auth**: for messages: `u.Message.Chat.ID != opts.ChatID` → log
`slog.Info("Rejected message from unauthorized chat", "chat_id", ...)` and
skip (this log line is how users discover their chat ID). For callback
queries: check `u.CallbackQuery.Message.Chat.ID`; always
`answerCallbackQuery` even when rejecting (empty text) so the button spinner
stops.

**Commands** (`strings.HasPrefix(text, "/")`, split first token, strip
optional `@botname` suffix):

| Command | Behavior |
|---|---|
| `/start`, `/help` | static help text listing commands |
| `/new [title]` | `CreateSession` (default title "telegram"); set active; reply `🆕 session: <title>` |
| `/sessions` | `ListSessions`, sort by `UpdatedAt` desc, cache; reply numbered list: `1. <title> — <MessageCount> msgs, $<Cost .2f>` (top 10), active one marked `▶` |
| `/use <n>` | index into cached list (1-based); errors politely if stale/out of range; set active session |
| `/status` | `GetAgentInfo` + `GetAgentSessionInfo(active)` + `GetAgentSessionQueuedPrompts`; reply model ID, ready/busy, session title, queued count, session `Cost`/tokens |
| `/cancel` | `CancelAgentSession(active)`; reply `🛑 cancel requested` (the RunComplete with `Cancelled` confirms) |
| unknown `/x` | "unknown command, try /help" |

**Plain text** (non-command): resolve the target session (§4.5): if the
update is a Telegram *reply* to a bridge message recorded in `msgSession`,
target that session; otherwise target the active session. Mint runID; `mu`:
`runs[runID] = targetSession`; `SendMessage` with the target session. On
error, remove from `runs` and report `❌ <err>`. Then check
`GetAgentSessionInfo(target).IsBusy` — if the run didn't start immediately
because a turn was already in flight, reply `⏳ queued behind the current
run` (queued-prompt semantics, §2.4). Send `sendChatAction "typing"`
immediately for responsiveness.

Photos/captions: out of scope for v1 — reply "📎 attachments not supported yet"
when `Message.Photo != nil` or `Message.Document != nil` (wired for M7).

### 4.2 Event pump (crush → Telegram)

```go
for {
    evc, err := c.SubscribeEvents(ctx, wsID)
    if err != nil { backoff; continue }
    resyncAfterReconnect()          // REST re-check busy state; expire stale perms
    for ev := range evc {
        switch e := ev.(type) {
        case pubsub.Event[proto.PermissionRequest]:
            if e.Type != pubsub.CreatedEvent || !b.knownSession(e.Payload.SessionID) { continue }
            b.showPermissionKeyboard(ctx, e.Payload)   // tagged with session title if not active (§4.5)
        case pubsub.Event[proto.PermissionNotification]:
            b.resolvePermByToolCall(ctx, e.Payload)
        case pubsub.Event[proto.RunComplete]:
            b.handleRunComplete(ctx, e.Payload)   // matches by RunID ∈ runs; fallback: SessionID == active
        case pubsub.Event[proto.AgentEvent]:
            if e.Payload.Error != nil { b.reportAgentError(ctx, e.Payload) }
        case pubsub.Event[proto.Session]:
            b.trackSession(e)                     // keep titles map current (§4.5)
        default:
            // Ignore message/lsp/mcp/etc. in v1.
        }
    }
    if ctx.Err() != nil { return }
    notify chat once; backoff (1s→30s doubling); continue
}
```

`handleRunComplete`: under `mu`, if `RunID` in `runs` (or `RunID == ""` and
`SessionID == sessionID`) → delete from `runs`, expire pending perms for that
session (edit their messages to `⌛ expired`), then send the outcome per §2.4,
session-tagged per §4.5.

### 4.5 Multi-session routing rules

Sessions are server-side objects; the bridge multiplexes them into one chat:

- **Known sessions**: `titles` is seeded from `ListSessions` at startup and
  maintained from `pubsub.Event[proto.Session]` (created/updated set
  `titles[id] = Title`; deleted removes). Only **top-level** sessions count:
  skip any with `ParentSessionID != ""` and IDs prefixed `"title-"` (title
  generation) or containing `"$$"` (agent tool sub-sessions) — their events
  (including permission requests, which carry only `SessionID`) would
  otherwise leak sub-agent noise into the chat. `knownSession(id)` = present
  in `titles`.
- **Active pointer**: `sessionID` is where plain (non-reply) text goes;
  moved by `/new` and `/use`.
- **Outbound tagging**: every bridge message that belongs to a session
  (final answers, permission keyboards, error reports) is prefixed with
  `📁 <session title>\n` **when that session is not the active one**. The
  active session's traffic stays unprefixed to keep the common case clean.
- **Reply routing**: after every session-owned `sendMessage`, record
  `msgSession[sentMsg.MessageID] = sessID` (append to `msgOrder`; if
  `len > 500` evict the oldest). Inbound messages with
  `Message.ReplyToMessage.MessageID` present (add `ReplyToMessage *Message
  \`json:"reply_to_message"\`` to the api `Message` type) that hit
  `msgSession` route to that session instead of the active one. This lets
  the user drive two runs in parallel from one chat: plain text → active
  session, swipe-reply → the other.
- **Cross-session events are surfaced, not filtered**: permission requests
  and run completions from ANY known session produce (tagged) messages.
  A run in a background session must never stall invisibly on an approval.

### 4.3 Typing indicator

One goroutine with a 4 s ticker: if `len(runs) > 0`, `sendChatAction(chatID,
"typing")` (chat actions auto-expire after ~5 s). Stop naturally when `runs`
is empty. Cheap and avoids editMessageText rate limits entirely in v1.

### 4.4 Permission keyboard

Message text (HTML mode) built by `render.PermissionSummary(req)` (§6.2).
Inline keyboard, one row:

```
[✅ Allow] [✅ For session] [🚫 Deny]
callback_data: "perm:<req.ID>:allow" | "perm:<req.ID>:allow_session" | "perm:<req.ID>:deny"
```

(`req.ID` is a UUID; `"perm:" + 36 + ":allow_session"` = 55 bytes ≤ the
64-byte callback_data limit.)

Callback handling: parse the three segments; look up `pendPerms[id]` (gone →
`answerCallbackQuery("already handled")`); call `GrantPermission` with the
stored full request and the mapped action; on `resolved==false` edit message
to `↩️ already handled elsewhere`; on success edit the original message —
append `\n\n✅ allowed` / `✅ allowed for session` / `🚫 denied` and remove the
keyboard (`reply_markup` omitted in editMessageText); `answerCallbackQuery`
with a short toast. Delete from `pendPerms`.

---

## 5. Telegram API client: `internal/telegram/api.go`

Hand-rolled, stdlib only. The Bot API is JSON-over-HTTPS:
`POST {base}/bot{token}/{method}` with a JSON body; every response is:

```go
type apiResponse struct {
    OK          bool            `json:"ok"`
    Result      json.RawMessage `json:"result"`
    Description string          `json:"description"`
    ErrorCode   int             `json:"error_code"`
    Parameters  *struct {
        RetryAfter int `json:"retry_after"`
    } `json:"parameters"`
}
```

```go
type api struct {
    base   string        // "https://api.telegram.org" or test server URL
    token  string
    client *http.Client  // Timeout: 65 * time.Second (must exceed long-poll 50s)
}

func (a *api) call(ctx context.Context, method string, payload, result any) error
```

`call`: marshal payload → POST `a.base+"/bot"+a.token+"/"+method`
(`Content-Type: application/json`) → decode envelope → if `!OK`: on
`ErrorCode==429` and `Parameters.RetryAfter>0`, sleep that many seconds and
retry (max 3 attempts total), else return
`fmt.Errorf("telegram %s: %s (code %d)", method, Description, ErrorCode)`.
**Never include the token in error strings or logs.**

Types (only the fields we use; all snake_case tags):

```go
type User struct {
    ID       int64  `json:"id"`
    Username string `json:"username"`
}
type Chat struct {
    ID   int64  `json:"id"`
    Type string `json:"type"`
}
type Message struct {
    MessageID int64       `json:"message_id"`
    From      *User       `json:"from"`
    Chat      Chat        `json:"chat"`
    Text      string      `json:"text"`
    Photo     []PhotoSize `json:"photo"`
    Document  *Document   `json:"document"`
    Caption   string      `json:"caption"`
}
type PhotoSize struct {
    FileID   string `json:"file_id"`
    FileSize int64  `json:"file_size"`
}
type Document struct {
    FileID   string `json:"file_id"`
    MimeType string `json:"mime_type"`
    FileName string `json:"file_name"`
}
type CallbackQuery struct {
    ID      string   `json:"id"`
    From    User     `json:"from"`
    Message *Message `json:"message"`
    Data    string   `json:"data"`
}
type Update struct {
    UpdateID      int64          `json:"update_id"`
    Message       *Message       `json:"message"`
    CallbackQuery *CallbackQuery `json:"callback_query"`
}
type InlineKeyboardButton struct {
    Text         string `json:"text"`
    CallbackData string `json:"callback_data"`
}
type InlineKeyboardMarkup struct {
    InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}
```

Methods (thin wrappers over `call`):

```go
getMe(ctx) (User, error)                    // method "getMe", empty payload {}
getUpdates(ctx, offset int64, timeoutSec int) ([]Update, error)
    // {"offset": offset, "timeout": timeoutSec,
    //  "allowed_updates": ["message", "callback_query"]}
    // NOTE: pass a per-call context; rely on a.client.Timeout (65s) > 50s poll.
sendMessage(ctx, chatID int64, text string, opts *sendOpts) (Message, error)
    // {"chat_id":…, "text":…} plus optional "parse_mode":"HTML",
    // "reply_markup": InlineKeyboardMarkup,
    // "link_preview_options": {"is_disabled": true}
editMessageText(ctx, chatID, messageID int64, text string, html bool) error
    // {"chat_id":…, "message_id":…, "text":…, ["parse_mode":"HTML"]}
    // Treat error "message is not modified" as success (string match on
    // Description) — happens on double-callback races.
answerCallbackQuery(ctx, id string, text string) error
    // {"callback_query_id": id, "text": text}
sendChatAction(ctx, chatID int64, action string) error
    // {"chat_id":…, "action":"typing"}
```

`sendOpts struct { HTML bool; Keyboard *InlineKeyboardMarkup }`. Text
messages are capped at 4096 UTF-16 code units — callers must pre-chunk (§6.3);
`sendMessage` itself does no chunking.

---

## 6. Rendering: `internal/telegram/render.go`

### 6.1 Escaping

All dynamic text placed inside HTML-mode messages goes through
`html.EscapeString` (stdlib `html`). Assistant final text is sent as
**plain text (no parse_mode)** in v1 — no escaping needed, no entity-parse
failures possible. Only permission summaries and status/help messages use
HTML mode.

### 6.2 `PermissionSummary(req proto.PermissionRequest) string` (HTML)

Header: `🔐 <b>Permission request</b>\n<tool emoji> <b><ToolName></b> — <escaped Description>`.
Then per-Params type (type switch per table §2.5):

- bash: `<pre>escaped .Command</pre>` (truncate §6.4), plus `dir: <WorkingDir>` if set.
- edit/write/multiedit: `<code><FilePath></code>` + unified-ish preview:
  `− old` / `+ new` first N lines each inside `<pre>` (use
  `TruncateMiddle`, §6.4; do NOT diff-compute, just show both truncated).
- download/fetch/agentic_fetch: escaped URL in `<code>`.
- view/ls: path in `<code>`.
- fallback `map[string]any`: `json.Marshal` compact, escaped, in `<pre>`, truncated.

### 6.3 `Chunk(text string, limit int) []string`

Telegram counts UTF-16 code units; to stay safe use `limit = 3900` measured
in UTF-16 units (`len(utf16.Encode([]rune(s)))`). Split preferentially on
`"\n"` boundaries (scan backwards from the limit), falling back to a hard
rune-boundary split. Never split inside a UTF-16 surrogate pair (splitting
on rune boundaries guarantees this). Empty input → `[]string{}`.
Send chunks sequentially; on multi-chunk sends add `(i/n)` prefix like
`[2/3]\n` to chunks after the first.

### 6.4 `TruncateMiddle(s string, max int) string`

If `len([]rune(s)) <= max` return as-is; else keep first `max*2/3` and last
`max/3` runes joined by `\n…(truncated)…\n`. Used for commands (max 1000)
and file contents (max 600 per side).

---

## 7. Tests

All tests `t.Parallel()`, testify `require`, no network.

### 7.1 `api_test.go`

Fake Telegram via `httptest.NewServer`: a handler that records
`{method, decoded JSON body}` and returns scripted responses. Cases:

- `call` happy path: envelope decode into typed result.
- `!ok` error includes description + code, **does not contain the token**
  (assert `!strings.Contains(err.Error(), token)`).
- 429 with `retry_after: 0`/`1` → retries then succeeds (keep retry_after
  small; inject sleep func or accept 1 s test latency — prefer a
  `sleep func(time.Duration)` field on `api` defaulting to `time.Sleep`,
  overridden in tests).
- `getUpdates` builds correct payload (offset/timeout/allowed_updates).
- `editMessageText` "message is not modified" → nil error.

### 7.2 `render_test.go`

- `Chunk`: empty; short; exact-limit; long text with newlines (splits on
  newline); pathological no-newline text (hard split); emoji/surrogate text
  (chunks re-encode to ≤ limit UTF-16 units and re-join to original when
  prefixes are stripped).
- `TruncateMiddle` boundary cases.
- `PermissionSummary`: one case per Params type incl. fallback map; asserts
  escaping (`<` in a bash command becomes `&lt;`) and truncation.

### 7.3 `bridge_test.go`

Fake **crush server** via `httptest.NewServer` implementing just the routes
the bridge touches (all under `/v1`):

- `GET /v1/workspaces/{id}/agent` → `proto.AgentInfo{IsReady: true}`
- `POST /v1/workspaces/{id}/agent/init`, `/agent/update` → 200
- `GET /v1/workspaces/{id}/agent/sessions/{sid}` → `proto.AgentSession`
- `POST /v1/workspaces/{id}/sessions` / `GET .../sessions` → canned sessions
- `POST /v1/workspaces/{id}/agent` → capture `proto.AgentMessage`, 202
- `POST /v1/workspaces/{id}/permissions/grant` → capture, `{"resolved":true}`
- `GET /v1/workspaces/{id}/events` → SSE: set `Content-Type: text/event-stream`,
  flush headers, then write test-injected events as
  `data: {"type":"<payload_type>","payload":{"type":"created","payload":{...}}}\n\n`
  — i.e. a `pubsub.Payload` envelope whose `Payload` is the marshaled
  `pubsub.Event[T]`. **Verify against `internal/client/proto.go:114-266`
  decode switch and mirror exactly what it expects.**

Connect with `client.NewClient(t.TempDir(), "tcp", strings.TrimPrefix(srv.URL, "http://"))`
— the client dials plain TCP when network is not unix/npipe
(`internal/client/client.go:141-156`).

Fake Telegram from 7.1 extended with a scripted `getUpdates` queue.

Scenarios:

1. **Round trip**: inject a text update → assert `POST /agent` captured with
   non-empty run_id and correct session; inject matching `RunComplete` SSE →
   assert final text sent to chat.
2. **Unauthorized chat** → no crush calls, nothing sent (except nothing);
   log-only.
3. **Permission flow**: inject `PermissionRequest` SSE → assert keyboard
   message sent with 3 buttons and correct callback data; inject callback
   `perm:<id>:allow` → assert grant POST captured with action `allow` and
   the full original request; assert message edited.
4. **Resolved elsewhere**: grant returns `{"resolved":false}` → message
   edited to the "already handled" text.
5. **Run error**: `RunComplete{Error:"boom"}` → "❌" message.
6. **/cancel** → cancel endpoint hit; `RunComplete{Cancelled:true}` → 🛑 message.
7. **Chunking**: `RunComplete.Text` of ~9000 chars → ≥3 sendMessage calls.
8. **Reply routing**: two sessions; complete a run in the non-active session
   (message gets `📁` tag); inject an update that replies to that message →
   assert the next `POST /agent` targets the non-active session, while a
   plain (non-reply) update targets the active one.

Give the bridge a small test hook: `Options.BaseURL` for Telegram and accept
the crush client from outside (already the case — `Options.Client`).

---

## 8. Milestones (implement in order; finish each before the next)

Each milestone ends with: `gofumpt -w .` (or `task fmt`), `go build .`,
`go test -race ./internal/telegram/... ./internal/cmd/...`, `task lint:fix`.
Commit each milestone separately with the given message.

- **M1** — `internal/telegram/api.go` + `api_test.go`.
  Commit: `feat(telegram): add minimal bot api client`
- **M2** — `render.go` + `render_test.go`.
  Commit: `feat(telegram): add message rendering helpers`
- **M3** — `bridge.go` skeleton (Options/state/Run wiring, session pick,
  startup message) + `internal/cmd/telegram.go`. Verify `go run . telegram --help`.
  Commit: `feat(telegram): add crush telegram command and bridge skeleton`
- **M4** — update pump + event pump + run lifecycle + typing ticker +
  reconnect loop; bridge_test scenarios 1, 2, 5, 7.
  Commit: `feat(telegram): prompt round trip with run completion`
- **M5** — permission keyboards + callbacks + expiry; scenarios 3, 4.
  Commit: `feat(telegram): permission approval via inline keyboards`
- **M6** — commands `/new /sessions /use /status /cancel /help`; scenario 6.
  Commit: `feat(telegram): session management commands`
- **M7 (optional, only if everything above is green)** — photo attachments:
  `getFile` (`{"file_id":…}` → `{file_path}`), download
  `{base}/file/bot{token}/{file_path}` (≤20 MB), build
  `message.Attachment{FileName, MimeType: "image/jpeg", Content: bytes}`
  (`internal/message/attachment.go:8-13`; images pass through to the model
  as binary parts — `internal/proto/message.go:529-539`).
  Commit: `feat(telegram): photo attachments`
- **M8** — `docs/notes/telegram-bridge.md`: BotFather setup, getting the chat
  ID, env vars, TUI co-watching (client/server is default),
  security notes (§9). Commit: `docs: telegram bridge setup guide`
- **M9 (optional, future)** — forum-topics mode: in a supergroup with topics
  enabled, map one topic per session (`createForumTopic`; carry
  `message_thread_id` on send and read it from inbound messages) so each
  session is its own thread. Only sketch: config flag `--forum`, topic ID ↔
  session ID map persisted to
  `filepath.Join(config.GlobalCacheDir(), "telegram", "topics.json")`.
  Do NOT start this without explicit instruction.

Manual smoke test (document in M8, run it if a token is available):

```sh
# 1. @BotFather -> /newbot -> token. 2. message @userinfobot -> your ID.
export CRUSH_TELEGRAM_BOT_TOKEN=123456:ABC...
export CRUSH_TELEGRAM_CHAT_ID=7654321
cd ~/some/project
crush telegram          # auto-spawns the server if needed
# in Telegram: /status, then a prompt, watch permission keyboard appear.
# TUI in the same project attaches to the same server by default:
# crush
```

---

## 9. Security requirements (non-negotiable)

1. Exactly one authorized chat ID; every message AND callback checked; others
   logged (`slog.Info`, include chat id, never content) and ignored.
2. The bot token must never appear in logs, error messages, or replies.
3. Do NOT implement or expose: skip-permissions/YOLO toggles,
   `SetPermissionsSkipRequests`, arbitrary shell execution commands
   (`RunShellCommand` stays unused), config mutation endpoints.
4. Permission messages must show the full command / file path (truncated only
   for length) — the user must see what they're approving.
5. Group chats: only obey the configured chat ID; recommend a private chat in
   docs.

---

## 10. Pitfalls checklist (read before coding, re-read before each milestone)

- [ ] End-of-turn = `RunComplete` matched by `RunID`. Message `Finish` parts
      fire mid-turn (`tool_use`) — never use them for completion.
- [ ] Event stream is workspace-wide: filter every event by session.
- [ ] `pubsub.Event[T]` arrives as a **value type** in the `any` channel —
      type-switch on `pubsub.Event[proto.X]`, not pointers.
- [ ] SSE has no replay + broker is lossy: on reconnect, resync via REST and
      expire pending permission keyboards.
- [ ] `SubscribeEvents` channel close ≠ error — it's the disconnect signal;
      loop and resubscribe.
- [ ] `GrantPermission` returning `false` = already resolved, not an error.
- [ ] `PermissionNotification` with `Granted=false, Denied=false` is a
      "request created" hint (published on a different broker, may arrive
      after the request event) — never treat it as a resolution.
- [ ] No permission-cancel event exists — expire keyboards on RunComplete.
- [ ] `proto.Session.IsBusy` is zero on SSE events; only trust REST reads.
- [ ] Prompts sent while busy are queued server-side — surface that to the user.
- [ ] Telegram: 4096-char (UTF-16) message cap → chunk at 3900;
      callback_data ≤ 64 bytes; long-poll timeout 50 s needs http.Client
      timeout > 50 s; handle 429 retry_after; "message is not modified" on
      edit = success.
- [ ] Plain text for model output; HTML only for bridge-authored messages
      with `html.EscapeString` on all dynamic parts.
- [ ] AGENTS.md: capitalized log messages, comments end in periods (78 col),
      gofumpt, testify require, snake_case JSON tags, semantic one-line commits.
- [ ] New deps: none. Stdlib + existing `google/uuid` only.
