# Channels (experimental)

Most MCP servers just expose tools that Crush calls. A channel goes the other
way: the server _pushes_ events into your session, so Crush can react to things
happening outside the terminal, like a webhook firing, a CI run failing, or
someone sending you a chat message. The protocol itself is described in the
[channels reference](https://code.claude.com/docs/en/channels-reference).

## How it works

To become a channel, a server declares the `claude/channel` capability in its
`initialize` result (`capabilities.experimental["claude/channel"] = {}`) and
then emits `notifications/claude/channel` events. Each event has a `content`
body and an optional `meta` map. Crush drops every event into the active session
as a `<channel>` element:

```text
<channel source="webhook" severity="high" run_id="1234">
build failed on main: https://ci.example.com/run/1234
</channel>
```

## Enabling a channel

Adding a channel server to `mcp` doesn't turn it on by itself. Pushing is
opt-in, so a server sitting in your config stays quiet until you say otherwise.
You can opt in for a single launch with `--channels`:

```bash
# Enable one or more configured MCP servers as channels for this session.
crush --channels server:webhook
crush --channels server:webhook --channels server:messaging
```

If you don't want to pass the flag every time, set `channel_enabled` on the
server's `mcp` entry instead:

```json
{
  "mcp": {
    "webhook": {
      "type": "http",
      "url": "https://example.com/mcp",
      "channel_enabled": true
    },
    "messaging": {
      "type": "stdio",
      "command": "/path/to/your-messaging-mcp",
      "args": ["--channel"],
      "channel_enabled": true
    }
  }
}
```

In `crushrc` it's `--channel true`:

```bash
mcp add webhook --type http --url "https://example.com/mcp" --channel true
```

Once a server is enabled, you'll see a Channels section on the landing page and
in the sidebar showing its state. The Channels command lists every enabled
server, plus any channel-capable servers you haven't turned on yet.

## Security

The `source` attribute is always the server's name from your config, so you can
trust it. The payload is a different story. It comes from the server, so Crush
treats it as untrusted: it checks the structure, caps the size of the body and
attributes, only allows `meta` keys made of `[A-Za-z0-9_]`, and escapes
everything so a payload can't break out of the `<channel>` element or fake its
own attributes. Anything malformed gets dropped. A server that hasn't been
enabled with `--channels` or `channel_enabled`, or that never declared the
capability, can't inject anything at all.

## Delivery

Channels work both in the regular in-process `crush` and against a shared
`crush serve` backend.

In-process, an event goes into whatever session you have open. If nothing is
open, Crush starts a new session, so events are never lost.

With `crush serve`, the server delivers each event exactly once. If a client is
attached, it goes into the session that client is looking at (or the most
recently updated one, if clients are looking at different sessions). If no one
is attached, it goes into the workspace's most recent session, and a new one is
only created when there isn't any. This means a headless server keeps handling
channel events even with nobody connected. Clients that attach later see the
injected turn come in through the usual event stream.

Live channels are tagged `channel` in the MCP list, which is a quick way to
check that your opt-in actually worked.

## Two-way channels

Channels can talk back, too. A channel is still a normal MCP server, so any tool
it exposes (say, a `reply` tool) is available to the agent like any other MCP
tool. There's nothing channel-specific to set up. Declare `tools` in the
server's capabilities, register the tool, and use the server's `instructions`
string (which ends up in the system prompt) to tell Crush when to call it and
which `<channel>` attribute to send back, such as a `chat_id`.

For example, a messaging MCP server (`your-messaging-mcp` here) could let Crush
send and receive chat messages:

1. Add it to your config, using the `messaging` example from
   [Enabling a channel](#enabling-a-channel).
2. Start Crush with `crush --channels server:messaging`.

Incoming messages show up as `<channel>` tags, and Crush replies with the
server's send tool, for example `send_message_to_user`.

## Channel reply routing

Out of the box, a turn triggered by a channel only prints to the terminal. If
someone messages you through a chat channel, they won't see an answer unless the
model decides to call a send tool on its own. That's not great.

A `channel_reply` block on the server's MCP config fixes this. When a turn that
came from that channel ends and the model _didn't_ reply through the channel,
Crush sends the final response back through the tool you configured: to the
sender for direct messages, or to the group for group messages.

```json
{
  "mcp": {
    "messaging": {
      "type": "stdio",
      "command": "/path/to/your-messaging-mcp",
      "args": ["--channel"],
      "channel_reply": {
        "user": { "tool": "send_message_to_user", "target_param": "user_id" },
        "group": {
          "tool": "send_message_to_group",
          "target_param": "group_id"
        },
        "suppress_tools": ["send"]
      }
    }
  }
}
```

In `crushrc`, pass the same object to `--channel-reply`:

```bash
mcp add messaging --command /path/to/your-messaging-mcp \
  --args --channel \
  --channel true \
  --channel-reply '{
    "user": {"tool": "send_message_to_user", "target_param": "user_id"},
    "group": {"tool": "send_message_to_group", "target_param": "group_id"},
    "suppress_tools": ["send"]
  }'
```

A few details on how routing works:

- If the event's meta has a `group`, the reply goes through the `group` route.
  If it has a `sender`, it goes through the `user` route. Use `target_meta` to
  pick a different meta attribute for the target, and `message_param` (`message`
  by default) to name the tool argument that gets the reply text.
- If the model already called one of the route tools, or anything in
  `suppress_tools`, during the turn, Crush skips the automatic reply so you
  don't get the same answer twice.
- Normal local turns, and channels without a `channel_reply` block, aren't
  affected at all.

This isn't tied to any particular chat service. It works the same way for
Discord, Slack, or anything else: point the routes at that server's send tools
and the right meta attributes.
