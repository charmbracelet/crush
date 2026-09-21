# Orca status integration

[Orca](https://github.com/stablyai/orca) shows a live status for every agent
pane in its left panel: working, idle, or waiting on you. Agents like Claude
Code tell Orca about that status through hooks Orca installs. Crush does it
through its terminal title, and can report through hooks as well.

Orca tracks status per pane, so several Crush panes in one worktree each show
their own state.

## Status via the terminal title

Crush's terminal title carries the state in its leading glyph, and always names
Crush after it so the pane stays attributable:

| State               | Title           | Glyph                       |
| ------------------- | --------------- | --------------------------- |
| Working             | `⠋ crush <dir>` | an animated braille spinner |
| Idle                | `✳ crush <dir>` | `✳`                         |
| Waiting on the user | `✋ crush <dir>` | `✋`                         |

Orca reads those glyphs before it checks whether a title belongs to an agent it
knows about, which is why this works today even though Crush is not (yet) in
Orca's agent-name list. There is nothing to configure: it is the title Crush
already sets.

The sequence is a plain OSC 2 title, so other terminal-side tools can read it
too; nothing in it is Orca-specific beyond the glyph convention.

## Status via hooks

Some detail only a hook can provide: which prompt a turn started from, which
tool just ran, exactly when a turn ended. Crush fires the same events Claude
Code does, and `integrations/orca/crush-hook.sh` forwards them to Orca's local
agent-hook endpoint the same way Orca's own per-agent scripts do.

Copy the script somewhere stable and register it for each event you want
reported. From your Crush checkout:

```sh
mkdir -p ~/.config/crush
cp integrations/orca/crush-hook.sh ~/.config/crush/orca-hook.sh
chmod +x ~/.config/crush/orca-hook.sh

hook add SessionStart     --command "$HOME/.config/crush/orca-hook.sh" --timeout 5
hook add UserPromptSubmit --command "$HOME/.config/crush/orca-hook.sh" --timeout 5
hook add PreToolUse       --command "$HOME/.config/crush/orca-hook.sh" --timeout 5
hook add PostToolUse      --command "$HOME/.config/crush/orca-hook.sh" --timeout 5
hook add Stop             --command "$HOME/.config/crush/orca-hook.sh" --timeout 5
```

The equivalent `crush.json`:

```jsonc
{
  "hooks": {
    "SessionStart": [{ "command": "$HOME/.config/crush/orca-hook.sh", "timeout": 5 }],
    "UserPromptSubmit": [{ "command": "$HOME/.config/crush/orca-hook.sh", "timeout": 5 }],
    "PreToolUse": [{ "command": "$HOME/.config/crush/orca-hook.sh", "timeout": 5 }],
    "PostToolUse": [{ "command": "$HOME/.config/crush/orca-hook.sh", "timeout": 5 }],
    "Stop": [{ "command": "$HOME/.config/crush/orca-hook.sh", "timeout": 5 }],
  },
}
```

Orca passes each agent pane its endpoint and pane key through the environment
(`ORCA_AGENT_HOOK_PORT`, `ORCA_AGENT_HOOK_TOKEN`, `ORCA_PANE_KEY`, and the
rest). The script does nothing when those are absent, so the same `crushrc` is
safe outside Orca, and it always writes `{}` to stdout, so it can never block a
turn.

The script posts to `/hook/crush`. Orca's hook server serves a fixed set of
routes and does not serve that one yet, so the request is rejected until Orca
adds Crush support. If you run a build that accepts a different route, point
the script at it with `ORCA_CRUSH_HOOK_ROUTE`.

## Which mechanism does what

| Need                                  | Use                                     |
| ------------------------------------- | --------------------------------------- |
| Working / idle / waiting in the panel | Terminal title, already on              |
| Turn started, tool ran, turn finished | Hook events, needs Orca's `/hook/crush` |
| Notifications outside Orca            | Hook events plus your own script        |
