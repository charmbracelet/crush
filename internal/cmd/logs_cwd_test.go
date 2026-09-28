package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogsFindsAncestorProjectLog(t *testing.T) {
	if os.Getenv("LOGS_CWD_TEST") == "1" {
		rootCmd.SetArgs([]string{"logs"})
		if err := rootCmd.Execute(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	project := t.TempDir()
	sub := filepath.Join(project, "sub")
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".crush", "logs"), 0o755))
	require.NoError(t, os.MkdirAll(sub, 0o755))
	init := exec.Command("git", "init")
	init.Dir = project
	require.NoError(t, init.Run())
	require.NoError(t, os.WriteFile(filepath.Join(project, "crush.json"), []byte("{}\n"), 0o644))
	line := "{\"time\":\"2026-01-02T03:04:05Z\",\"level\":\"INFO\",\"msg\":\"LOGTOKEN-ancestor\"}\n"
	require.NoError(t, os.WriteFile(filepath.Join(project, ".crush", "logs", "crush.log"), []byte(line), 0o644))

	cmd := exec.Command(os.Args[0], "-test.run=^TestLogsFindsAncestorProjectLog$")
	cmd.Dir = sub
	cmd.Env = append(os.Environ(), "LOGS_CWD_TEST=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	require.Contains(t, string(out), "LOGTOKEN-ancestor")
}
