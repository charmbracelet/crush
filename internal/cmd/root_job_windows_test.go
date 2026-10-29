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

// jobTestRoleEnv makes the test binary play the "client", which spawns the
// binary again as the "server" with startDetachedServer. It is dispatched
// from init because the server gets crush's arguments, not test flags.
const jobTestRoleEnv = "CRUSH_TEST_JOB_ROLE"

var jobTestHost = &url.URL{Scheme: "npipe", Host: "//./pipe/crush-job-test"}

func init() {
	switch os.Getenv(jobTestRoleEnv) {
	case "client":
		_, _ = io.Copy(io.Discard, os.Stdin) // Wait to be put in the job.
		_ = os.Setenv(jobTestRoleEnv, "server")
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		if _, err := startDetachedServer(cmd, jobTestHost); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	case "server":
		fmt.Println(os.Getpid()) // Lands in stdout.log.
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}

// TestStartDetachedServerOutlivesJob guards the server being killed with
// the kill-on-close job its client ran in (as Win32-OpenSSH sessions do)
// although the job allows breakaway.
func TestStartDetachedServerOutlivesJob(t *testing.T) {
	t.Parallel()

	job, err := windows.CreateJobObject(nil, nil)
	require.NoError(t, err)
	closeJob := sync.OnceValue(func() error { return windows.CloseHandle(job) })
	t.Cleanup(func() { _ = closeJob() })
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	require.NoError(t, err)

	exe, err := os.Executable()
	require.NoError(t, err)
	cacheDir := t.TempDir()
	client := exec.CommandContext(t.Context(), exe)
	client.Env = append(os.Environ(), jobTestRoleEnv+"=client", "CRUSH_CACHE_DIR="+cacheDir)
	stdin, err := client.StdinPipe()
	require.NoError(t, err)
	var output strings.Builder
	client.Stdout, client.Stderr = &output, &output
	require.NoError(t, client.Start())
	proc, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE,
		false, uint32(client.Process.Pid))
	require.NoError(t, err)
	err = windows.AssignProcessToJobObject(job, proc)
	_ = windows.CloseHandle(proc)
	if err != nil {
		_ = client.Process.Kill()
		_ = client.Wait()
		t.Skipf("Cannot put a process in a job object here: %v", err)
	}
	require.NoError(t, stdin.Close())
	require.NoError(t, client.Wait(), "client output: %s", output.String())

	stdoutLog := filepath.Join(cacheDir, "server-"+safeHostName(jobTestHost), "stdout.log")
	var pid int
	require.Eventually(t, func() bool {
		data, err := os.ReadFile(stdoutLog)
		line, ok := strings.CutSuffix(string(data), "\n")
		if err != nil || !ok {
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

	require.NoError(t, closeJob())
	event, err := windows.WaitForSingleObject(server, 2000)
	require.NoError(t, err)
	require.Equal(t, uint32(windows.WAIT_TIMEOUT), event, "the server was killed with the client's job")
}
