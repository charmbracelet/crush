# Client/Server Mode

Normally, each `crush` you start is self-contained. The TUI, the agent, your
LSPs and MCP servers, and the session database all live in that one process, and
they all go away when you quit.

Client/server mode splits Crush in two. A background `crush server` process runs
the agent and everything around it, and the TUI becomes a client that talks to
the server over HTTP. You get two things out of that:

Client/server mode is off by default.

## Turning it on

Set `CRUSH_CLIENT_SERVER` and run Crush like you normally would:

```bash
export CRUSH_CLIENT_SERVER=1
crush
```

`1` and `true` both work. Add the `export` to your shell profile to keep it on.

The first time, there's no server yet, so Crush starts one in the background,
waits for it to come up, and connects to it. After that, every `crush` you run
connects to the same server. You don't have to look after it: it shuts itself
down a minute after you close your last Crush window.

## How it works

Client/server mode consists of 3 different parts:

1. The server: it listens on a UNIX socket (or a named pipe on Windows)
2. A workspace: one project that is opened at once. There is never more, than
   one workspace opened for one directory (even with several clients on the same
   directory!).
3. The client: it is anything that talks to the server (e.g. Crush window).

When you start the TUI, it asks the server for a workspace for the current
directory. If another window already has that directory open, the TUI joins that
workspace. If not, the server opens a new one.

### Sharing a workspace

Everyone in a workspace sees the same sessions and messages. Each window tells
the server which session it's showing, and permission requests and questions
from the agent only go to the windows showing the session that asked. If two
windows show that session, both get the prompt. The first answer counts, and the
other window's dialog goes away.

Settings are shared, too. A workspace keeps the settings of the window that
opened it, so a second window in the same project ignores its own `-y`,
`--debug`, and `--data-dir` flags. And if you switch permission modes from
inside the TUI, the change applies to every window in the workspace.

### When a workspace closes

A workspace stays open while at least one client is connected to it. When the
last one leaves, the server closes it: it cancels whatever the agent is still
doing, stops the LSPs and MCP servers, and closes the database. That's what
happens when you quit Crush in local mode, too.

A client that quits normally leaves right away. If a client disappears without
saying goodbye, because it crashed or its terminal was killed, the server holds
its place for 10 seconds in case it comes back.

The TUI reconnects by itself when it loses its connection. If it reconnects and
finds its workspace gone, for instance because the server restarted, it opens
the workspace again and carries on. Sessions are stored on disk, so none of them
are lost. If the server itself dies, though, the TUI doesn't start a new one. It
keeps trying until a server is back, for example the next time you start Crush
in another terminal.

## Starting and stopping

### Starting

When a client can't reach a server, it starts one:

1. If there's a socket file but nothing is listening on it (a leftover from a
   crash, for example), the client deletes it.
2. It runs `crush server` in the background, detached from your terminal, so
   closing the terminal doesn't take the server with it.
3. It waits for the server to respond. By default it gives up after 10 seconds.
   If your machine needs longer, set `CRUSH_SERVER_READY_TIMEOUT` to something
   like `30s`.

If two terminals start Crush at the same moment, you still get one server. The
first terminal takes a lock while it starts the server, and the second one waits
for it and then uses that server.

### Stopping

The server shuts itself down 60 seconds after its last workspace closes. If you
open Crush again before then, it reuses the running server. You can change the
delay with `CRUSH_SERVER_IDLE_TIMEOUT`, in seconds. Set it to `0` to have the
server exit as soon as its last workspace closes.

To stop the server yourself, send it `SIGTERM`:

```bash
pkill -f 'crush server'
```

If it's running in your terminal, `ctrl+c` does the same. Either way, it closes
every workspace first, giving them up to 15 seconds, and then exits. On Windows,
end the process in Task Manager.

### Reloading config

`SIGHUP` doesn't stop the server. It reloads the server's config, and the config
of every open workspace, from disk:

```bash
pkill -HUP -f 'crush server'
```

If a config fails to load, the workspace keeps its old settings and the error
goes to the log.

### Upgrading

Clients check the server's version when they connect. If you've upgraded Crush
and the server is still running the old version, the client asks it to shut down
so it can start a new one. The server only agrees when no workspaces are open,
so if you still have Crush open somewhere, the client keeps using the old server
for now. Once you close everything, the old server exits within a minute, and
the next `crush` you run starts the new version.

The check compares build IDs as well as version numbers, so a rebuilt binary
counts as a new version even if the version number didn't change. That mostly
matters when you're working on Crush itself.

## The server's environment

The server gets its environment from the terminal that started it, and it reads
that environment once. Everything that runs inside the server uses it: your
config (including any `$VARIABLES` in your [`crushrc`](../config/README.md)),
LSPs, MCP servers, and the commands the agent runs.

That has a catch. Say you start Crush in one terminal, then open a second
terminal, activate a Python virtualenv, and start Crush there too. The agent in
the second window still runs commands with the first terminal's `PATH`. In the
same way, an API key you export after the server started won't reach the server.

So when you change an environment variable Crush depends on, restart the server.
Close your Crush windows and wait for it to exit, or stop it yourself.

The one exception is shell commands you run from the TUI prompt with `!`. They
use the environment of the window that opened the workspace.

## Running the server yourself

You can start the server by hand. That's useful when you want to watch its logs
or run it somewhere other than the default socket:

```bash
crush server
```

Without flags, it listens where clients look by default. To use a different
address, pass `--host`, and give clients the same `--host` so they can find it:

```bash
# In one terminal
crush server --host unix:///tmp/crush-dev.sock

# In another
CRUSH_CLIENT_SERVER=1 crush --host unix:///tmp/crush-dev.sock
```

When the server runs in a terminal, it logs there as well as to its log file.
Add `--debug` for more detail. A server that a client starts always logs at the
normal level, so starting it yourself is how you get debug logs out of the
server.

A server you start by hand follows the same rules as any other: it exits 60
seconds after its last workspace closes. It doesn't time out before the first
client shows up, though. To keep it around longer between sessions, raise
`CRUSH_SERVER_IDLE_TIMEOUT`.

### Addresses

`--host` takes a URL, and the scheme picks the transport:

| Scheme     | Example                       | Notes                                         |
| ---------- | ----------------------------- | --------------------------------------------- |
| `unix://`  | `unix:///tmp/crush.sock`      | The default on macOS and Linux                |
| `npipe://` | `npipe:////./pipe/crush.sock` | The default on Windows                        |
| `tcp://`   | `tcp://127.0.0.1:8080`        | No authentication; read [Security](#security) |
| `http://`  | `http://127.0.0.1:8080`       | Same as `tcp://`                              |

Clients only start servers on their own for Unix sockets and named pipes. With
TCP, start the server yourself before you connect. Clients also skip the version
check from [Upgrading](#upgrading) on TCP.

The default socket is called `crush-<uid>.sock`, where `<uid>` is your user ID.
It goes in `$XDG_RUNTIME_DIR` if that's set, which on Linux is usually
`/run/user/<uid>`. Otherwise it goes in `$TMPDIR`, a private per-user directory
on macOS, and if that isn't set either, in `/tmp`.

If the full path comes out longer than 104 bytes, which is too long for a Unix
socket on macOS, Crush uses `/tmp/crush-<uid>.sock` instead. On Windows, the
default is the named pipe `\\.\pipe\crush-<user SID>.sock`.

To get the default socket path in a shell:

```bash
dir="${XDG_RUNTIME_DIR:-${TMPDIR:-/tmp}}"
sock="${dir%/}/crush-$(id -u).sock"
[ ${#sock} -gt 104 ] && sock="/tmp/crush-$(id -u).sock"
echo "$sock"
```

## Files

Each server address gets its own folder in Crush's cache directory. The cache
directory is `~/.cache/crush`, or `%LOCALAPPDATA%\crush\cache` on Windows.
`$CRUSH_CACHE_DIR` and `$XDG_CACHE_HOME` change it. The folder is named after
the address, like `server-unix____tmp_crush-dev.sock`, and holds:

| File                       | What it is                                                                  |
| -------------------------- | --------------------------------------------------------------------------- |
| `crush.log`                | The log. The server and its clients all write to it.                        |
| `stdout.log`, `stderr.log` | Output of a server that a client started. Check these if it fails to start. |
| `start.lock`               | Keeps two clients from starting two servers at once.                        |

The log is JSON, one entry per line. To follow it:

```bash
tail -f ~/.cache/crush/server-*/crush.log
```

While a project is open, the server also holds a lock file, `crush.lock`, in the
project's data directory (`.crush` by default). It stops a second server from
opening the same project. Crush in local mode doesn't use this lock.

## Commands

Most commands work the same with or without client/server mode. These are the
ones that don't:

| Command        | With `CRUSH_CLIENT_SERVER=1`                                                                                          |
| -------------- | --------------------------------------------------------------------------------------------------------------------- |
| `crush`        | Connects to the server, and starts one if needed.                                                                     |
| `crush login`  | Same as `crush`.                                                                                                      |
| `crush run`    | Uses the server only if one is already running. Otherwise it runs locally, so a one-off prompt leaves nothing behind. |
| `crush logout` | Always goes through a server, even without `CRUSH_CLIENT_SERVER`, and starts one if needed.                           |
| `crush logs`   | Shows the project log, which client/server mode doesn't write to. Read the server's `crush.log` instead.              |

`crush run`, `crush login`, and `crush logout` take `--host` the same way
`crush` does.

`crush session` and `crush stats` read the project database straight from disk,
so they work whether a server is running or not. `crush projects` only lists
projects that were opened in local mode at least once.

## Environment variable reference

| Variable                     | Default          | Read by | What it does                                                                               |
| ---------------------------- | ---------------- | ------- | ------------------------------------------------------------------------------------------ |
| `CRUSH_CLIENT_SERVER`        | off              | client  | Turns client/server mode on.                                                               |
| `CRUSH_SERVER_READY_TIMEOUT` | `10s`            | client  | How long a client waits for a server it just started. Takes a duration like `30s` or `2m`. |
| `CRUSH_SERVER_IDLE_TIMEOUT`  | `60`             | server  | Seconds the server stays up after its last workspace closes. `0` means exit right away.    |
| `CRUSH_SERVER_DETACH_GRACE`  | `10`             | server  | Seconds the server waits for a client that dropped off to come back. `0` means don't wait. |
| `CRUSH_SKIP_DATADIR_LOCK`    | off              | server  | Turns off the project lock file. Only for filesystems that don't support file locks.       |
| `CRUSH_CACHE_DIR`            | `~/.cache/crush` | both    | Where the server's logs and lock file go.                                                  |

The server reads its variables once, when it starts. When a client starts the
server for you, the server gets them from the client's terminal, so set them
there before the server is running.

## Security

The server doesn't check who's connecting. Anyone who can reach it can do
anything you can do in Crush: read your config, talk to the agent, and run
commands on your machine as you.

With a Unix socket, file permissions decide who can connect. The default socket
goes in a per-user directory when there is one: `$XDG_RUNTIME_DIR` on Linux,
`$TMPDIR` on macOS. If it ends up in `/tmp` on a machine you share with other
people, check the socket's permissions.

TCP has no such protection. Any process that can reach the port can connect,
including other people's processes on the same machine. If you use TCP anyway,
bind to `127.0.0.1`, never to `0.0.0.0` or a public address, and don't do it on
shared machines.

## Troubleshooting

**"failed to initialize crush server"**

The client started a server but couldn't reach it in time. Look at `stderr.log`
and `crush.log` in the server's folder (see [Files](#files)). If the server was
just slow to start, raise `CRUSH_SERVER_READY_TIMEOUT`.

**"data directory already in use by another crush process"**

Another server already has this project open. The message includes its process
ID. This usually means two servers are running on different addresses. Stop the
one you don't need.

**The old version is still running after an upgrade**

The old server is waiting for its workspaces to close (see
[Upgrading](#upgrading)). Close your Crush windows, or stop the server yourself.

**The agent doesn't see an environment variable you just set**

Restart the server (see [The server's environment](#the-servers-environment)).

**What is the server doing?**

Ask it. With the `$sock` variable from [Addresses](#addresses):

```bash
# Paths of open workspaces
curl -s --unix-socket "$sock" http://crush/v1/workspaces | jq -r '.[].path'

# Which version is running
curl -s --unix-socket "$sock" http://crush/v1/version
```

For more, see [API.md](API.md).
