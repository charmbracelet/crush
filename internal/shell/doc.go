// Package shell provides cross-platform shell execution capabilities.
//
// WINDOWS COMPATIBILITY:
// This implementation provides both POSIX shell emulation (mvdan.cc/sh/v3),
// which also works on Windows.
// Some caution has to be taken: commands should have forward slashes (/) as
// path separators to work, even on Windows.
//
// Example usage of the shell package:
//
// 1. For one-off commands:
//
//	sh := shell.NewShell(nil)
//	stdout, stderr, err := sh.Exec(context.Background(), "echo hello")
//
// 2. For maintaining state across commands:
//
//	sh := shell.NewShell(&shell.Options{
//	    WorkingDir: "/tmp",
//	})
//	sh.Exec(ctx, "export FOO=bar")
//	sh.Exec(ctx, "echo $FOO")  // Will print "bar"
package shell
