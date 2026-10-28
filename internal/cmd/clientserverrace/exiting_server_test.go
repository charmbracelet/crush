package clientserverrace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestExitingServerIsReplaced is a regression test for a client that
// reaches a server after it has committed to shutting down, over tcp.
// Named pipes, the default on Windows, take the same path: neither leaves
// a socket file behind for the client to wait out.
//
// The exiting server refuses the workspace with a 503, so the client has
// to wait for it to release the address and bring up a replacement. It
// used to do neither, because the exiting server still reported itself
// healthy: the client took it for the replacement, retried it and gave up
// within milliseconds.
func TestExitingServerIsReplaced(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping exiting server replacement test in -short mode")
	}
	// Holding the server in its exiting state takes a graceful stop,
	// which takes SIGTERM.
	if runtime.GOOS == "windows" {
		t.Skip("skipping: stopping the server gracefully needs SIGTERM")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("skipping: 'go' not available on PATH")
	}

	bin := buildCrushBinary(t, repoRootFromTest(t))

	runDir := t.TempDir()
	env := isolatedEnv(t, runDir)

	// The old server hosts a project whose SessionEnd hook outlasts the
	// hook budget, so once stopped it spends a couple of seconds tearing
	// the workspace down while it keeps listening.
	oldProj := filepath.Join(runDir, "old")
	newProj := filepath.Join(runDir, "new")
	for _, dir := range []string{oldProj, newProj} {
		require.NoError(t, os.MkdirAll(dir, 0o700))
	}
	require.NoError(t, os.WriteFile(filepath.Join(oldProj, "crush.json"),
		[]byte(`{"hooks":{"SessionEnd":[{"command":"sleep 10"}]}}`), 0o600))

	addr := freeTCPAddr(t)
	host := "tcp://" + addr
	base := "http://" + addr
	// Cleanups run in reverse, so this stops the replacement after the
	// old server has been reaped.
	t.Cleanup(func() { stopTCPServer(t, base) })

	old := exec.CommandContext(t.Context(), bin, "server", "--host", host)
	old.Env = env
	require.NoError(t, old.Start())
	oldDone := make(chan struct{})
	go func() {
		_ = old.Wait()
		close(oldDone)
	}()
	t.Cleanup(func() {
		_ = old.Process.Kill()
		<-oldDone
	})

	require.Eventually(t, func() bool {
		status, _ := request(base, http.MethodGet, "/v1/health", nil)
		return status == http.StatusOK
	}, 30*time.Second, 50*time.Millisecond, "the old server never became healthy")

	status, body := request(base, http.MethodPost, "/v1/workspaces", map[string]string{
		"path":      oldProj,
		"client_id": uuid.NewString(),
	})
	require.Equal(t, http.StatusOK, status, "creating the old server's workspace: %s", body)

	// The server unregisters its workspaces as it latches the decision to
	// exit, before it tears them down.
	require.NoError(t, old.Process.Signal(syscall.SIGTERM))
	require.Eventually(t, func() bool {
		status, body := request(base, http.MethodGet, "/v1/workspaces", nil)
		var workspaces []json.RawMessage
		return status == http.StatusOK &&
			json.Unmarshal(body, &workspaces) == nil && len(workspaces) == 0
	}, 5*time.Second, 10*time.Millisecond, "the old server never started shutting down")

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	client := exec.CommandContext(ctx, bin, "--host", host, "--cwd", newProj, "run", "hi")
	client.Env = env
	out, _ := client.CombinedOutput()
	// Errors are word-wrapped to the terminal width.
	got := strings.ToLower(strings.Join(strings.Fields(string(out)), " "))

	require.NotContains(t, got, "server is shutting down",
		"the client gave up on the exiting server instead of replacing it:\n%s", out)
	// With no provider configured, this is as far as a run gets once a
	// live server has created its workspace.
	require.Contains(t, got, "no providers configured",
		"the client never got a workspace from a replacement server:\n%s", out)
}

// isolatedEnv returns a minimal environment that keeps crush away from
// the user's real state and from any provider credentials the test
// itself runs with.
func isolatedEnv(t *testing.T, runDir string) []string {
	t.Helper()
	env := []string{
		"CRUSH_CLIENT_SERVER=1",
		"CRUSH_DISABLE_PROVIDER_AUTO_UPDATE=1",
		"CRUSH_DISABLE_METRICS=1",
		// Exit once the last client leaves, so no server outlives the
		// test.
		"CRUSH_SERVER_IDLE_TIMEOUT=0",
	}
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		dir := filepath.Join(runDir, strings.ToLower(name))
		require.NoError(t, os.MkdirAll(dir, 0o700))
		env = append(env, name+"="+dir)
	}
	for _, name := range []string{"PATH", "TMPDIR", "USER"} {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return env
}

// freeTCPAddr returns a loopback address with a port nothing listens on.
func freeTCPAddr(t *testing.T) string {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

// request sends one API request to the server at base. It returns the
// status code, or 0 when nothing answered, and the response body.
func request(base, method, path string, body any) (int, []byte) {
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil
		}
		payload = bytes.NewReader(data)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, base+path, payload)
	if err != nil {
		return 0, nil
	}
	req.Header.Set("Content-Type", "application/json")
	tr := &http.Transport{DisableKeepAlives: true}
	defer tr.CloseIdleConnections()
	rsp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		return 0, nil
	}
	defer rsp.Body.Close()
	data, _ := io.ReadAll(rsp.Body)
	return rsp.StatusCode, data
}

// stopTCPServer asks any crush server still listening at base to shut
// down and waits for it to release the address.
func stopTCPServer(t *testing.T, base string) {
	t.Helper()
	_, _ = request(base, http.MethodPost, "/v1/control", map[string]string{"command": "shutdown"})
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if status, _ := request(base, http.MethodGet, "/v1/health", nil); status == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Logf("a crush server is still listening at %s", base)
}
