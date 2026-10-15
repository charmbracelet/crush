//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd

package pinentry

// TerminalHandoverSupported reports whether the terminal handover
// helpers (terminal mode configuration and Ctrl-L injection) exist on
// this platform. It is a compile-time constant so callers can gate the
// calls without a runtime check.
const TerminalHandoverSupported = false

// procExeSupported reports whether /proc/<pid>/exe can be read to resolve
// a process executable when the process table does not provide a path.
const procExeSupported = false

// ReapplyTerminalModes is a no-op on platforms without a POSIX terminal:
// there are no terminal modes to configure and no pinentry dialog that
// could read from them.
func ReapplyTerminalModes() {}

// InjectCtrlLRedraw is a no-op on platforms without a POSIX terminal:
// terminal-based pinentry flavors do not exist to receive the byte.
func InjectCtrlLRedraw() {}
