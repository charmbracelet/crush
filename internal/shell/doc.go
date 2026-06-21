package shell

// Example usage of the shell package:
//
// 1. For one-off commands:
//
//	shell := shell.NewShell(nil)
//	stdout, stderr, err := shell.Exec(context.Background(), "echo hello")
//
// 2. For maintaining state across commands:
//
//	shell := shell.NewShell(&shell.Options{
//	    WorkingDir: "/tmp",
//	    Logger: myLogger,
//	})
//	shell.Exec(ctx, "export FOO=bar")
//	shell.Exec(ctx, "echo $FOO")  // Will print "bar"
//
// 3. Managing environment and working directory:
//
//	shell := shell.NewShell(nil)
//	shell.SetEnv("MY_VAR", "value")
//	shell.SetWorkingDir("/tmp")
//	cwd := shell.GetWorkingDir()
//	env := shell.GetEnv()
