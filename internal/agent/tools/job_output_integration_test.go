package tools

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestJobOutputWithWaitTime_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("background shell wait functionality", func(t *testing.T) {
		// This test manually verifies the wait logic we implemented in job_output.go
		workingDir := t.TempDir()
		bgManager := shell.GetBackgroundShellManager()

		// Start a quick task
		bgShell, err := bgManager.Start(t.Context(), workingDir, nil, "sleep 0.5 && echo 'done'", "")
		require.NoError(t, err)
		defer bgManager.Kill(bgShell.ID)

		// Simulate the wait logic from our job_output tool
		maxWaitTime := 2 // 2 seconds
		done := false

		// Get initial state
		_, _, initialDone, err := bgShell.GetOutput()
		require.NoError(t, err)
		require.False(t, initialDone, "Task should not be done immediately")

		// Wait for completion with timeout (simulating our job_output logic)
		if !initialDone && maxWaitTime > 0 {
			waitDone := make(chan bool, 1)

			go func() {
				bgShell.Wait()
				waitDone <- true
			}()

			select {
			case <-waitDone:
				// Task completed
				_, _, done, err = bgShell.GetOutput()
				require.NoError(t, err)
				require.True(t, done, "Task should be completed")
			case <-time.After(time.Duration(maxWaitTime) * time.Second):
				// Timeout
				t.Fatal("Should not have timed out for a 0.5 second task with 2 second timeout")
			}
		}

		require.True(t, done, "Task should have completed successfully")
	})
}
