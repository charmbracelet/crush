---
name: shell-builtins
description: Use when creating a new shell builtin command for Crush (internal/shell/), editing an existing one, or when the user needs to understand how commands are intercepted in Crush's embedded shell.
---

# Shell Builtins

Crush's shell (`internal/shell/`) uses `mvdan.cc/sh/v3` for POSIX shell
emulation. Commands can be intercepted before they reach the OS by adding
**builtins** — functions handled in-process.

## How Builtins Work

Builtins live in `Shell.builtinHandler()` in `internal/shell/shell.go`.
This is an `interp.ExecHandlerFunc` middleware registered in
`execHandlers()` **before** the block handler, so builtins run even for
commands that would otherwise be blocked.

The handler is a switch on `args[0]`. Each case either handles the command
inline or delegates to a helper function.

## Adding a New Builtin

1. **Add the case** to the switch in `builtinHandler()` in `shell.go`.
2. **Get I/O from the handler context**, not from `os.Stdin`/`os.Stdout`.
   This ensures the builtin works with pipes and redirections:
   ```go
   case "mycommand":
       hc := interp.HandlerCtx(ctx)
       return handleMyCommand(args, hc.Stdin, hc.Stdout, hc.Stderr)
   ```
3. **Implement the handler** in its own file (e.g.,
   `internal/shell/mycommand.go`). The function signature should accept
   args, stdin, stdout, and stderr:
   ```go
   func handleMyCommand(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
       // args[0] is the command name ("mycommand"), args[1:] are arguments.
       // Write output to stdout, errors to stderr.
       // Return nil on success, or interp.ExitStatus(n) for non-zero exit codes.
   }
   ```
4. **Return values**: return `nil` for success, `interp.ExitStatus(n)` for
   non-zero exit codes. Write error messages to `stderr` before returning.
5. **No extra wiring needed** — `builtinHandler()` is already registered
   in `execHandlers()`.

## Existing Builtins

| Command | File | Description |
|---------|------|-------------|
| `jq` | `jq.go` | JSON processor using `github.com/itchyny/gojq` |
