---
name: shell-builtins
description: Use when creating a new shell builtin command for Crush (internal/shell/), editing an existing one, or when the user needs to understand how commands are intercepted in Crush's embedded shell.
---

# Shell Builtins

Crush's shell (`internal/shell/`) uses `mvdan.cc/sh/v3` for POSIX shell
emulation. Commands can be intercepted before they reach the OS by adding
**builtins** — functions handled in-process.

## How Builtins Work

Builtins live in `builtinHandler()` in `internal/shell/run.go`. This is an
`interp.ExecHandlerFunc` middleware registered in `standardHandlers()`
**before** the block handler, so builtins run even for commands that would
otherwise be blocked. The same handler chain is shared by the stateful
`Shell` type and the stateless `Run` entrypoint used by the hook runner,
so builtins are available identically in the `bash` tool and in hooks.

The handler is a switch on `args[0]`. Each case either handles the command
inline or delegates to a helper function.

## Adding a New Builtin

1. **Add the case** to the switch in `builtinHandler()` in `run.go`.
2. **Get I/O from the handler context**, not from `os.Stdin`/`os.Stdout`.
   This ensures the builtin works with pipes and redirections:
   ```go
   case "mycommand":
       hc := interp.HandlerCtx(ctx)
       return handleMyCommand(ctx, args, hc.Stdin, hc.Stdout, hc.Stderr)
   ```
3. **Implement the handler** in its own file (e.g.,
   `internal/shell/mycommand.go`). The function signature must accept a
   `context.Context` as the first parameter, plus args, stdin, stdout, and
   stderr:
   ```go
   func handleMyCommand(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
       // args[0] is the command name ("mycommand"), args[1:] are arguments.
       // Write output to stdout, errors to stderr.
       // Return nil on success, or interp.ExitStatus(n) for non-zero exit codes.
   }
   ```
4. **Poll `ctx` in every unbounded loop.** Builtins that iterate over
   input, emit values in a generator-style loop, or do any other work
   that can exceed a few milliseconds MUST check `ctx.Err()` on each
   iteration and return it verbatim when non-nil. Hook timeouts rely on
   this: an unbounded builtin that never polls ctx cannot be interrupted
   by a hook's `timeout_sec`, and the hook runner will have to abandon
   the goroutine (see `internal/hooks/runner.go`). Returning `ctx.Err()`
   (not `interp.ExitStatus(n)`) lets callers distinguish "command exited
   non-zero" from "we ran out of time".
   ```go
   for _, item := range items {
       if err := ctx.Err(); err != nil {
           return err
       }
       // ... process item
   }
   ```
5. **Return values**: return `nil` for success, `interp.ExitStatus(n)` for
   non-zero exit codes, or `ctx.Err()` on cancellation. Write error
   messages to `stderr` before returning.
6. **No extra wiring needed** — `builtinHandler()` is already registered
   in `standardHandlers()`.

## Existing Builtins

| Command | File | Description |
|---------|------|-------------|
| `jq` | `jq.go` | JSON processor using `github.com/itchyny/gojq` |
