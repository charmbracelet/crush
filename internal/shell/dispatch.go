package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/crush/internal/filepathext"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// probeWindow is how many bytes we read from the head of a file to decide
// how to dispatch it. 128 is plenty for a shebang line and for magic-byte
// inspection, while small enough to make the probe cheap for users whose
// hooks invoke many scripts.
const probeWindow = 128

// scriptDispatchHandler returns middleware that intercepts exec of a
// path-prefixed argv[0] (e.g. ./foo.sh, /opt/bin/tool, C:\foo\bar.exe) and
// dispatches based on the file's contents:
//
//  1. Shebang line (#!...) → exec the named interpreter via os/exec. The
//     interpreter is resolved literally first, then via PATH on the
//     basename as a permissive fallback (so #!/bin/bash works on Windows
//     boxes where Git for Windows puts bash.exe on PATH).
//  2. Known binary magic (MZ, ELF, Mach-O) or a NUL byte in the probe
//     window → pass through to the next handler (mvdan's default exec).
//  3. Otherwise → treat the file as shell source and run it in-process via
//     a nested interp.Runner that reuses the same handler stack.
//
// Non-path-prefixed argv[0] and empty args are passed straight through; this
// handler is a no-op for ordinary commands like `echo` or `jq`.
//
// blockFuncs is the block list used when building the nested runner for the
// shell-source case, so deny rules apply recursively to commands invoked
// from in-process scripts.
func scriptDispatchHandler(blockFuncs []BlockFunc) func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if len(args) == 0 || !isPathPrefixed(args[0]) {
				return next(ctx, args)
			}

			// Resolve relative paths against the interpreter's cwd, not
			// the process cwd — hook commands are authored with the hook
			// Runner's cwd in mind and sub-shells can cd before an exec.
			scriptPath := filepathext.SmartJoin(interp.HandlerCtx(ctx).Dir, args[0])
			probe, err := probeFile(scriptPath)
			if err != nil {
				return err
			}

			switch {
			case hasShebang(probe):
				return dispatchShebang(ctx, scriptPath, probe, args)
			case isBinary(probe):
				return next(ctx, args)
			default:
				return runShellSource(ctx, scriptPath, args, blockFuncs)
			}
		}
	}
}

// isPathPrefixed reports whether argv[0] is a file reference (as opposed
// to a bare command to be resolved via PATH). A path reference starts with
// `./`, `../`, `/`, or — on Windows — a drive-letter prefix.
//
// Note: mvdan already performs tilde expansion during word expansion, so
// `~/script.sh` arrives here as an absolute path. We still call the helper
// on the raw string to stay robust if a future change ever bypasses that
// expansion; cover that path with a regression test.
func isPathPrefixed(arg string) bool {
	switch {
	case strings.HasPrefix(arg, "./"),
		strings.HasPrefix(arg, "../"),
		strings.HasPrefix(arg, "/"):
		return true
	}
	if runtime.GOOS == "windows" {
		// Drive-letter paths: C:\foo or C:/foo (length check avoids
		// accidentally matching a single letter followed by a colon).
		if len(arg) >= 3 && isDriveLetter(arg[0]) && arg[1] == ':' &&
			(arg[2] == '\\' || arg[2] == '/') {
			return true
		}
		// Also treat backslash-prefixed UNC-like paths as path-prefixed.
		if strings.HasPrefix(arg, "\\") {
			return true
		}
	}
	return false
}

func isDriveLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// probeFile reads the first probeWindow bytes of the target path. It
// deliberately does not slurp the whole file: callers that need the full
// contents (only the shell-source branch) re-open via os.ReadFile. This
// keeps memory bounded when argv[0] turns out to be a large binary.
//
// Returns errors surfaced by os.Open/os.Stat directly so callers see the
// real reason: ENOENT, EACCES, EISDIR, ELOOP, etc.
func probeFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("%s: is a directory", path)
	}
	probe := make([]byte, probeWindow)
	n, err := io.ReadFull(f, probe)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	return probe[:n], nil
}

// hasShebang reports whether probe starts with the `#!` marker. A
// one-byte file that happens to be `#` is not a shebang.
func hasShebang(probe []byte) bool {
	return len(probe) >= 2 && probe[0] == '#' && probe[1] == '!'
}

// isBinary heuristically classifies probe as an executable or otherwise
// non-text file. A NUL byte in the first probeWindow bytes is the classic
// Unix-y text-vs-binary signal; we additionally recognize known magic
// numbers so we can fast-path well-formed binaries that happen to have no
// NUL in the first 128 bytes (rare but possible for small binaries).
func isBinary(probe []byte) bool {
	if bytes.IndexByte(probe, 0) >= 0 {
		return true
	}
	magics := [][]byte{
		{'M', 'Z'},               // Windows PE / DOS MZ.
		{0x7F, 'E', 'L', 'F'},    // ELF.
		{0xFE, 0xED, 0xFA, 0xCE}, // Mach-O 32-bit BE.
		{0xFE, 0xED, 0xFA, 0xCF}, // Mach-O 64-bit BE.
		{0xCF, 0xFA, 0xED, 0xFE}, // Mach-O 64-bit LE.
		{0xCE, 0xFA, 0xED, 0xFE}, // Mach-O 32-bit LE.
		{0xCA, 0xFE, 0xBA, 0xBE}, // Mach-O fat binary.
	}
	for _, m := range magics {
		if bytes.HasPrefix(probe, m) {
			return true
		}
	}
	return false
}

// dispatchShebang parses probe's shebang line and execs the resolved
// interpreter via os/exec, inheriting the parent runner's cwd, env, and
// stdio. Returns interp.ExitStatus on non-zero interpreter exit so the
// parent interpreter sees it as a normal non-zero status.
func dispatchShebang(ctx context.Context, scriptPath string, probe []byte, args []string) error {
	sb, err := parseShebang(probe)
	if err != nil {
		hc := interp.HandlerCtx(ctx)
		fmt.Fprintf(hc.Stderr, "crush: %s: %s\n", scriptPath, err)
		return interp.ExitStatus(126)
	}

	interpreter, err := resolveInterpreter(sb.interpreter)
	if err != nil {
		hc := interp.HandlerCtx(ctx)
		fmt.Fprintf(hc.Stderr, "crush: %s: %s\n", scriptPath, err)
		return interp.ExitStatus(127)
	}

	cmdArgs := append([]string{}, sb.args...)
	cmdArgs = append(cmdArgs, scriptPath)
	cmdArgs = append(cmdArgs, args[1:]...)

	cmd := exec.CommandContext(ctx, interpreter, cmdArgs...)
	hc := interp.HandlerCtx(ctx)
	cmd.Dir = hc.Dir
	cmd.Env = execEnvList(hc.Env)
	cmd.Stdin = hc.Stdin
	cmd.Stdout = hc.Stdout
	cmd.Stderr = hc.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code := exitErr.ExitCode()
			if code < 0 {
				code = 1
			}
			return interp.ExitStatus(uint8(code))
		}
		return err
	}
	return nil
}

// resolveInterpreter tries the literal shebang path first, then falls back
// to PATH-lookup on its basename — but only when the literal path is
// genuinely missing. A file that exists but fails stat for another reason
// (EACCES, ELOOP, etc.) surfaces the real error: silently resolving a
// different binary off PATH in that case would hide a real problem and
// produce surprising behavior for the user.
//
// The permissive fallback is what makes #!/bin/bash portable to Windows
// boxes where Git for Windows puts bash.exe on PATH but there is no
// /bin/bash on disk.
func resolveInterpreter(path string) (string, error) {
	_, statErr := os.Stat(path)
	if statErr == nil {
		return path, nil
	}
	if !errors.Is(statErr, fs.ErrNotExist) {
		return "", statErr
	}

	base := filepath.Base(path)
	if base == "" || base == path && !strings.ContainsAny(path, `/\`) {
		// Already a bare name — just do a PATH lookup.
		resolved, err := exec.LookPath(path)
		if err != nil {
			return "", fmt.Errorf("interpreter %q not found in PATH", path)
		}
		return resolved, nil
	}
	resolved, err := exec.LookPath(base)
	if err != nil {
		return "", fmt.Errorf("interpreter %q not found and %q not in PATH", path, base)
	}
	slog.Debug("Shebang interpreter not found; falling back to PATH",
		"requested", path, "resolved", resolved)
	return resolved, nil
}

// shebang captures the parsed `#!` line. interpreter is the program to
// invoke; args is the list of extra arguments to pass before the script
// path. The kernel's single-arg semantics (for literal paths and for env
// without `-S`) is encoded by returning a single-element args slice
// containing the un-tokenized remainder.
type shebang struct {
	interpreter string
	args        []string
}

// parseShebang extracts the interpreter invocation from probe. It tolerates
// CRLF line endings and a single leading space between `#!` and the path.
// env special-cases: `/usr/bin/env NAME [args...]` unwraps to NAME with
// kernel single-arg semantics; `-S` enables tokenized argument splitting.
func parseShebang(probe []byte) (*shebang, error) {
	if !hasShebang(probe) {
		return nil, errors.New("not a shebang")
	}
	line := probe[2:]
	// Take up to the first newline.
	if idx := bytes.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	// Strip trailing CR (CRLF-authored scripts).
	line = bytes.TrimRight(line, "\r")
	// Strip leading whitespace ("#! /usr/bin/env bash" is legal).
	line = bytes.TrimLeft(line, " \t")
	if len(line) == 0 {
		return nil, errors.New("empty shebang")
	}

	var pathStr, rest string
	if idx := bytes.IndexAny(line, " \t"); idx >= 0 {
		pathStr = string(line[:idx])
		rest = strings.TrimLeft(string(line[idx+1:]), " \t")
	} else {
		pathStr = string(line)
	}

	if isEnvShebang(pathStr) {
		return parseEnvShebang(rest)
	}

	// Literal-path shebang: kernel semantics pass the remainder as a
	// single argv[1], not tokenized.
	sb := &shebang{interpreter: pathStr}
	if rest != "" {
		sb.args = []string{rest}
	}
	return sb, nil
}

// isEnvShebang reports whether the shebang path targets `env`. We accept
// both common absolute paths and a bare `env` so that unusual setups
// (NixOS, BSDs) still work.
func isEnvShebang(p string) bool {
	if p == "/usr/bin/env" || p == "/bin/env" {
		return true
	}
	return filepath.Base(p) == "env"
}

// parseEnvShebang handles `/usr/bin/env` rewriting. Without `-S`, the
// remainder after the program name is a single argv[1] (kernel
// single-arg semantics via env, even though real env would fail to find a
// program named "bash -x"). With `-S`, the remainder is tokenized on
// whitespace. Any other `env` flag is rejected — forwarding unknown flags
// to a /usr/bin/env on disk is a subtle portability footgun we don't want.
func parseEnvShebang(rest string) (*shebang, error) {
	if rest == "" {
		return nil, errors.New("env: missing program name")
	}

	useSplit := false
	if strings.HasPrefix(rest, "-") {
		var flag, after string
		if idx := strings.IndexAny(rest, " \t"); idx >= 0 {
			flag = rest[:idx]
			after = strings.TrimLeft(rest[idx+1:], " \t")
		} else {
			flag = rest
			after = ""
		}
		if flag != "-S" {
			return nil, fmt.Errorf("unsupported env flag: %s", flag)
		}
		useSplit = true
		rest = after
		if rest == "" {
			return nil, errors.New("env -S requires a program")
		}
	}

	if rest == "" {
		return nil, errors.New("env: missing program name")
	}

	var prog, remainder string
	if idx := strings.IndexAny(rest, " \t"); idx >= 0 {
		prog = rest[:idx]
		remainder = strings.TrimLeft(rest[idx+1:], " \t")
	} else {
		prog = rest
	}

	sb := &shebang{interpreter: prog}
	if remainder != "" {
		if useSplit {
			sb.args = strings.Fields(remainder)
		} else {
			sb.args = []string{remainder}
		}
	}
	return sb, nil
}

// runShellSource parses path's contents as POSIX shell and runs it
// in-process via a nested interp.Runner. It reuses the parent runner's cwd,
// env, and stdio, and rebuilds the Crush handler stack so builtins and the
// dispatch handler itself remain available to anything the script invokes.
// Positional parameters ($1, $2, …) come from args[1:].
//
// This is the only branch that reads the full file; probeFile keeps its
// read to probeWindow bytes so the binary/shebang paths never touch more
// than 128 bytes of I/O.
func runShellSource(ctx context.Context, path string, args []string, blockFuncs []BlockFunc) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	file, err := syntax.NewParser().Parse(bytes.NewReader(data), path)
	if err != nil {
		return fmt.Errorf("could not parse %s: %w", path, err)
	}

	hc := interp.HandlerCtx(ctx)

	opts := []interp.RunnerOption{
		interp.StdIO(hc.Stdin, hc.Stdout, hc.Stderr),
		interp.Interactive(false),
		interp.Env(hc.Env),
		interp.Dir(hc.Dir),
		interp.ExecHandlers(standardHandlers(blockFuncs)...),
	}
	if len(args) > 1 {
		// Params with a leading "--" avoids any of args[1:] being
		// misinterpreted as set-options (e.g. a user passing "-e" as
		// a positional arg to their script).
		params := append([]string{"--"}, args[1:]...)
		opts = append(opts, interp.Params(params...))
	}

	runner, err := interp.New(opts...)
	if err != nil {
		return fmt.Errorf("could not build runner for %s: %w", path, err)
	}
	return runner.Run(ctx, file)
}

// execEnvList converts an expand.Environ to the []string form that
// os/exec.Cmd.Env expects. Only exported string variables are included,
// matching what a real shell would pass to a child process.
func execEnvList(env expand.Environ) []string {
	var out []string
	env.Each(func(name string, vr expand.Variable) bool {
		if vr.Exported && vr.Kind == expand.String {
			out = append(out, name+"="+vr.Str)
		}
		return true
	})
	return out
}
