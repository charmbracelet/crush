# Telegram Bridge Setup

Drive a Crush agent from a single authorized Telegram chat: send prompts,
watch results, approve/deny permission requests, manage sessions, and cancel
runs.

## Prerequisites

1. A configured Crush install (`crush` has been run at least once so a
   provider/model is set).
2. A Telegram bot token from [@BotFather](https://t.me/BotFather)
   (`/newbot`).
3. Your numeric Telegram chat ID (message [@userinfobot](https://t.me/userinfobot),
   or start the bridge with a wrong `--chat-id` and read the logged
   `Rejected message from unauthorized chat` line).

## Environment

```sh
export CRUSH_TELEGRAM_BOT_TOKEN=123456:ABC...
export CRUSH_TELEGRAM_CHAT_ID=7654321
```

Flags work too: `crush telegram --token ... --chat-id ...`.

## Run

```sh
cd ~/some/project
crush telegram
```

The command auto-spawns a `crush server` on the default unix socket when
one is not already running (same path as `crush run` / the TUI in
client-server mode).

### Co-watch with the TUI

```sh
# terminal 1
crush telegram

# terminal 2 — attach the TUI to the same server
CRUSH_CLIENT_SERVER=1 crush
```

Permissions approved in either place resolve for both.

## Commands (in Telegram)

| Command | Action |
|---|---|
| `/help` | list commands |
| `/new [title]` | create and switch to a new session |
| `/sessions` | list recent sessions |
| `/use <n>` | switch active session (index from `/sessions`) |
| `/status` | model, busy/ready, cost, queued prompts |
| `/cancel` | cancel the active session's run |

Any other text is a prompt to the **active** session. Reply to a bridge
message to target that message's session instead (parallel multi-session
use from one chat).

## Security

- Exactly **one** chat ID is authorized. Other chats are logged (chat id
  only, never message content) and ignored.
- The bot token is never written to logs, error messages, or replies.
- There is no skip-permissions / YOLO control from Telegram. Approvals
  always show the command or file path being requested.
- Prefer a private chat with the bot over a group.

## Logs

```
$XDG_CACHE_HOME/crush/telegram/crush.log
# or ~/.cache/crush/telegram/crush.log
```

Pass `--debug` for verbose logging to stderr as well.
