//go:build solaris || illumos

package notification

// NativeSupported reports whether native OS notifications are available on
// this platform. It is false on illumos/solaris: beeep pulls in
// github.com/godbus/dbus/v5, whose connection code is excluded on the solaris
// build tag (which illumos also satisfies) and has no replacement, so it fails
// to compile. OSC and bell backends still work, so callers hide the native
// option and fall back to a terminal-based backend.
const NativeSupported = false

// defaultNotifyFunc is a no-op fallback so NewNativeBackend still compiles if
// something forces the native backend on an unsupported platform. Selection
// logic avoids the native backend here, so this is only a safety net.
var defaultNotifyFunc = func(title, message string, icon any) error { return nil }
