//go:build windows

package cmd

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// jobTestRoleEnv makes a run of the test binary play one side of a server
// spawn instead of running tests: the "client" calls startDetachedServer,
// which starts the binary again as the "server". The roles are dispatched
// from init, before the testing package runs, because the server is
// started with crush's arguments rather than test flags, and so that this
// file does not take the package's only TestMain.
const jobTestRoleEnv = "CRUSH_TEST_JOB_ROLE"

// jobTestHost is the host the client spawns a server for. Only its
// per-host log directory is used: the stand-in server does not listen.
var jobTestHost = &url.URL{Scheme: "npipe", Host: "//./pipe/crush-job-test"}

func init() {
	switch os.Getenv(jobTestRoleEnv) {
	case "client":
		// Wait for the test to put this process in its job.
		_, _ = io.Copy(io.Discard, os.Stdin)
		_ = os.Setenv(jobTestRoleEnv, "server")
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		if _, err := startDetachedServer(cmd, jobTestHost); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	case "server":
		// Report the PID through stdout.log and wait to be killed.
		fmt.Println(os.Getpid())
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

// TestStartDetachedServerOutlivesJob checks that a server spawned from a
// job that kills its processes when it closes but lets them break away,
// like the job Win32-OpenSSH runs each session in, survives the job.
func TestStartDetachedServerOutlivesJob(t *testing.T) {
	t.Parallel()

	job, closeJob := newKillOnCloseJob(t, windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK)
	server := spawnServerInJob(t, job)

	require.NoError(t, closeJob())
	event, err := windows.WaitForSingleObject(server, 2000)
	require.NoError(t, err)
	require.Equal(t, uint32(windows.WAIT_TIMEOUT), event,
		"the server was killed with the job of the client that spawned it")
}

// TestStartDetachedServerInJobWithoutBreakaway checks that the server is
// still started, inside the job, when the job does not allow breakaway.
func TestStartDetachedServerInJobWithoutBreakaway(t *testing.T) {
	t.Parallel()

	job, closeJob := newKillOnCloseJob(t, 0)
	server := spawnServerInJob(t, job)

	require.NoError(t, closeJob())
	event, err := windows.WaitForSingleObject(server, 10000)
	require.NoError(t, err)
	require.Equal(t, uint32(windows.WAIT_OBJECT_0), event,
		"the server was not started inside the job")
}

// newKillOnCloseJob creates a job object that kills its processes when its
// last handle is closed, with limits added to its limit flags. It returns
// the job and a function that closes it.
func newKillOnCloseJob(t *testing.T, limits uint32) (windows.Handle, func() error) {
	t.Helper()
	job, err := windows.CreateJobObject(nil, nil)
	require.NoError(t, err)
	closeJob := sync.OnceValue(func() error { return windows.CloseHandle(job) })
	t.Cleanup(func() { _ = closeJob() })

	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | limits
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	require.NoError(t, err)
	return job, closeJob
}

// spawnServerInJob runs a client in job that spawns a server with
// startDetachedServer, and returns a handle to the server process.
func spawnServerInJob(t *testing.T, job windows.Handle) windows.Handle {
	t.Helper()
	exe, err := os.Executable()
	require.NoError(t, err)
	cacheDir := t.TempDir()

	client := exec.CommandContext(t.Context(), exe)
	client.Env = append(os.Environ(), jobTestRoleEnv+"=client", "CRUSH_CACHE_DIR="+cacheDir)
	stdin, err := client.StdinPipe()
	require.NoError(t, err)
	var output strings.Builder
	client.Stdout = &output
	client.Stderr = &output
	require.NoError(t, client.Start())

	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false, uint32(client.Process.Pid))
	require.NoError(t, err)
	err = windows.AssignProcessToJobObject(job, proc)
	_ = windows.CloseHandle(proc)
	if err != nil {
		_ = client.Process.Kill()
		_ = client.Wait()
		// A job that sets UI limits, for one, cannot be nested in.
		t.Skipf("Cannot put a process in a job object here: %v", err)
	}
	require.NoError(t, stdin.Close())
	require.NoError(t, client.Wait(), "client output: %s", output.String())

	stdoutLog := filepath.Join(cacheDir, "server-"+safeHostName(jobTestHost), "stdout.log")
	var pid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(stdoutLog)
		if err != nil {
			return false
		}
		line, ok := strings.CutSuffix(string(data), "\n")
		if !ok {
			return false
		}
		pid, err = strconv.Atoi(line)
		return err == nil
	}, 10*time.Second, 10*time.Millisecond, "the server did not start")

	server, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = windows.TerminateProcess(server, 1)
		_, _ = windows.WaitForSingleObject(server, 10000)
		_ = windows.CloseHandle(server)
	})
	return server
}
