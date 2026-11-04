//go:build darwin || linux || freebsd || netbsd || openbsd

package pinentry

// TerminalHandoverSupported reports whether the terminal handover
// helpers (terminal mode configuration and Ctrl-L injection) exist on
// this platform. It is a compile-time constant so callers can gate the
// calls without a runtime check.
const TerminalHandoverSupported = true

// procExeSupported reports whether /proc/<pid>/exe can be read to resolve
// a process executable when the process table does not provide a path.
const procExeSupported = true
