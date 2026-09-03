//go:build !windows

package shell

// normalizeHomeForPOSIXShell is a no-op on POSIX platforms: HOME never
// contains backslashes there, so there is nothing for the POSIX shell
// tokenizer to misinterpret. See home_windows.go for the Windows case.
func normalizeHomeForPOSIXShell(env []string) []string {
	return env
}
