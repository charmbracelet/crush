//go:build !windows

package lsphost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/version"
	"github.com/stretchr/testify/require"
)

// socketPath returns a path for a unix socket that fits the platform's
// limit, which the test's temporary directory may not.
func socketPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "h.sock")
	if len(path) < 100 {
		return path
	}
	dir, err := os.MkdirTemp("/tmp", "crush-lsphost-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "h.sock")
}

// fakeHost listens on a unix socket and hands the one connection it
// accepts, with the hello read from it, to serve.
func fakeHost(t *testing.T, serve func(conn net.Conn, hello Hello)) string {
	t.Helper()
	path := socketPath(t)
	ln, err := net.Listen("unix", path) //nolint:noctx
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		line, err := readLine(conn, MaxLine)
		if err != nil {
			return
		}
		var hello Hello
		if err := json.Unmarshal(line, &hello); err != nil {
			return
		}
		serve(conn, hello)
	}()
	return path
}

// syncBuffer is a bytes.Buffer that can be written and read concurrently.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// dialCommand runs Main as `crush __lsp-dial` would, with a pipe for its
// stdin, and reports its exit code on the returned channel.
type dialCommand struct {
	stdin  *io.PipeWriter
	stdout *syncBuffer
	stderr *syncBuffer
	exit   <-chan int
}

func runDialCommand(t *testing.T, args ...string) *dialCommand {
	t.Helper()
	in, stdin := io.Pipe()
	t.Cleanup(func() { _ = stdin.Close() })
	exit := make(chan int, 1)
	cmd := &dialCommand{stdin: stdin, stdout: &syncBuffer{}, stderr: &syncBuffer{}, exit: exit}
	go func() {
		// The command's name is not among the arguments Main is given.
		exit <- Main(t.Context(), args[1:], in, cmd.stdout, cmd.stderr)
	}()
	return cmd
}

func (c *dialCommand) wait(t *testing.T) int {
	t.Helper()
	select {
	case code := <-c.exit:
		return code
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the command did not exit")
		return -1
	}
}

func TestArgs(t *testing.T) {
	t.Parallel()
	require.Equal(t,
		[]string{
			"__lsp-dial",
			"--address", "unix:///run/joy-501/lsp-1.sock",
			"--server", "gopls",
			"--root", "/work/proj",
			"--workspace", "w1",
			"--pid", "4242",
		},
		Args("/run/joy-501/lsp-1.sock", "gopls", "/work/proj", "w1", 4242))
}

func TestSocketPath(t *testing.T) {
	t.Parallel()

	path, err := SocketPath("unix:///run/joy-501/lsp-1.sock")
	require.NoError(t, err)
	require.Equal(t, "/run/joy-501/lsp-1.sock", path)
	require.Equal(t, "unix:///run/joy-501/lsp-1.sock", Address(path))

	for _, address := range []string{"", "/run/joy.sock", "tcp://127.0.0.1:1", "unix://", "unix://relative/joy.sock"} {
		_, err := SocketPath(address)
		require.Error(t, err, address)
	}
}

// TestDialCommandPumpsBothWays covers the success path: the hello carries
// what the command was given, bytes the host sends right behind its ack
// are not swallowed with it, both directions flow, nothing is written to
// stderr, and the command exits 0 when its stdin closes.
func TestDialCommandPumpsBothWays(t *testing.T) {
	t.Parallel()

	hellos := make(chan Hello, 1)
	fromClient := make(chan string, 1)
	more := make(chan struct{})
	socket := fakeHost(t, func(conn net.Conn, hello Hello) {
		hellos <- hello
		// The ack and the first bytes of the session in one write.
		_, _ = conn.Write([]byte(`{"ok":true,"v":1,"host":"joy","pid":7}` + "\nfirst;"))
		buf := make([]byte, len("to-host"))
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		fromClient <- string(buf)
		<-more
		_, _ = conn.Write([]byte("second;"))
		// Reading until the client leaves keeps the connection open.
		_, _ = io.Copy(io.Discard, conn)
	})

	cmd := runDialCommand(t, Args(socket, "gopls", "/work/proj", "w1", 4242)...)

	hello := <-hellos
	require.Equal(t, Hello{
		LSPHost:       Protocol,
		Server:        "gopls",
		Client:        "crush",
		ClientVersion: version.Version,
		Workspace:     "w1",
		Root:          "/work/proj",
		PID:           4242,
	}, hello)

	require.Eventually(t, func() bool { return cmd.stdout.String() == "first;" }, 5*time.Second, 5*time.Millisecond)

	_, err := cmd.stdin.Write([]byte("to-host"))
	require.NoError(t, err)
	require.Equal(t, "to-host", <-fromClient)

	close(more)
	require.Eventually(t, func() bool { return cmd.stdout.String() == "first;second;" }, 5*time.Second, 5*time.Millisecond)

	require.NoError(t, cmd.stdin.Close())
	require.Equal(t, 0, cmd.wait(t))
	require.Empty(t, cmd.stderr.String(), "nothing is written to stderr on the success path")
	require.Equal(t, "first;second;", cmd.stdout.String(), "no notice when the client stopped the command")
}

func TestDialCommandFailsWhenRefused(t *testing.T) {
	t.Parallel()

	socket := fakeHost(t, func(conn net.Conn, _ Hello) {
		_, _ = conn.Write([]byte(`{"ok":false,"error":"no server gopls"}` + "\n"))
	})

	start := time.Now()
	cmd := runDialCommand(t, Args(socket, "gopls", "/work/proj", "w1", 1)...)
	require.NotEqual(t, 0, cmd.wait(t))
	require.Less(t, time.Since(start), HelloTimeout, "a refusal fails at once")
	require.Contains(t, cmd.stderr.String(), "no server gopls")
	require.Empty(t, cmd.stdout.String(), "nothing reaches the LSP client")
}

func TestDialCommandFailsOnAnotherProtocol(t *testing.T) {
	t.Parallel()

	socket := fakeHost(t, func(conn net.Conn, _ Hello) {
		_, _ = conn.Write([]byte(`{"ok":true,"v":2}` + "\n"))
		_, _ = io.Copy(io.Discard, conn)
	})

	cmd := runDialCommand(t, Args(socket, "gopls", "/work/proj", "w1", 1)...)
	require.NotEqual(t, 0, cmd.wait(t))
	require.Contains(t, cmd.stderr.String(), "protocol 2")
}

func TestDialCommandFailsWithoutHost(t *testing.T) {
	t.Parallel()

	// A socket nobody listens on, and one that does not exist.
	stale := socketPath(t)
	ln, err := net.Listen("unix", stale) //nolint:noctx
	require.NoError(t, err)
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())

	for _, socket := range []string{stale, filepath.Join(filepath.Dir(stale), "missing.sock")} {
		start := time.Now()
		cmd := runDialCommand(t, Args(socket, "gopls", "/work/proj", "w1", 1)...)
		require.NotEqual(t, 0, cmd.wait(t))
		require.Less(t, time.Since(start), DialTimeout)
		require.Contains(t, cmd.stderr.String(), "connecting to the host")
		require.Empty(t, cmd.stdout.String())
	}
}

func TestDialCommandFailsWhenTheHostDoesNotAnswer(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	socket := fakeHost(t, func(net.Conn, Hello) { <-release })

	start := time.Now()
	cmd := runDialCommand(t, Args(socket, "gopls", "/work/proj", "w1", 1)...)
	require.NotEqual(t, 0, cmd.wait(t))
	require.Less(t, time.Since(start), HelloTimeout+2*time.Second)
	require.Contains(t, cmd.stderr.String(), "reading the host's answer")
}

// TestDialCommandExitsWhenTheHostCloses covers the host going away in the
// middle of a session: the command exits non-zero without waiting for its
// stdin, stays silent on stderr, and leaves the LSP client a notification
// saying so.
func TestDialCommandExitsWhenTheHostCloses(t *testing.T) {
	t.Parallel()

	closeNow := make(chan struct{})
	socket := fakeHost(t, func(conn net.Conn, _ Hello) {
		_, _ = conn.Write([]byte(`{"ok":true,"v":1}` + "\n"))
		<-closeNow
	})

	cmd := runDialCommand(t, Args(socket, "gopls", "/work/proj", "w1", 1)...)
	select {
	case code := <-cmd.exit:
		require.FailNow(t, "exited while the host was there", "code %d", code)
	case <-time.After(100 * time.Millisecond):
	}

	close(closeNow)
	require.NotEqual(t, 0, cmd.wait(t))
	require.Empty(t, cmd.stderr.String())

	// The notice is one well-formed LSP message.
	r := bufio.NewReader(bytes.NewReader([]byte(cmd.stdout.String())))
	header, err := r.ReadString('\n')
	require.NoError(t, err)
	var length int
	require.Greater(t, len(header), len("Content-Length: \r\n"))
	length, err = strconv.Atoi(header[len("Content-Length: ") : len(header)-2])
	require.NoError(t, err)
	blank, err := r.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "\r\n", blank)
	body, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Len(t, body, length)
	var msg struct {
		JSONRPC string           `json:"jsonrpc"`
		ID      *json.RawMessage `json:"id"`
		Method  string           `json:"method"`
		Params  json.RawMessage  `json:"params"`
	}
	require.NoError(t, json.Unmarshal(body, &msg))
	require.Equal(t, "2.0", msg.JSONRPC)
	require.Nil(t, msg.ID, "a notification")
	require.Equal(t, ClosedMethod, msg.Method)
	require.JSONEq(t, `{}`, string(msg.Params))
}

func TestDialCommandUsage(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{},
		{"--server", "gopls"},
		{"--address", "unix:///run/joy.sock"},
		{"--address", "/run/joy.sock", "--server", "gopls"},
		{"--no-such-flag"},
	} {
		var stderr bytes.Buffer
		code := Main(context.Background(), args, bytes.NewReader(nil), io.Discard, &stderr)
		require.Equal(t, 2, code, args)
		require.NotEmpty(t, stderr.String(), args)
	}
}

// dialFakeHost connects to a fake host, as Main does before it pumps.
func dialFakeHost(t *testing.T, socket string) net.Conn {
	t.Helper()
	conn, _, err := Dial(t.Context(), socket, Hello{Server: "gopls"})
	require.NoError(t, err)
	return conn
}

// TestPumpGivesUpOnAHostThatStopsReading covers a host that is suspended
// or stuck: it keeps the connection and reads nothing. The pump must not
// block on it for good. It ends as when the host closes the connection,
// notice included.
func TestPumpGivesUpOnAHostThatStopsReading(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	socket := fakeHost(t, func(conn net.Conn, _ Hello) {
		_, _ = conn.Write([]byte(`{"ok":true,"v":1}` + "\n"))
		<-release
	})
	conn := dialFakeHost(t, socket)

	const timeout = 300 * time.Millisecond
	var out syncBuffer
	begin := time.Now()
	// Far more than the socket's buffers take.
	err := pump(t.Context(), conn, bytes.NewReader(make([]byte, 16<<20)), &out, timeout)
	require.ErrorIs(t, err, ErrHostClosed)
	require.GreaterOrEqual(t, time.Since(begin), timeout)
	require.Less(t, time.Since(begin), 5*time.Second)
	require.Equal(t, string(closedNotice), out.String())
}

// TestPumpWaitsForASlowHost covers a host that reads slowly: every write
// takes longer than the timeout, and none is given up, because the host
// keeps taking something.
func TestPumpWaitsForASlowHost(t *testing.T) {
	t.Parallel()

	const (
		timeout = 200 * time.Millisecond
		size    = 384 << 10
	)
	received := make(chan int, 1)
	socket := fakeHost(t, func(conn net.Conn, _ Hello) {
		_, _ = conn.Write([]byte(`{"ok":true,"v":1}` + "\n"))
		// A few KiB at a time, several times per timeout: less than one
		// write of the pump in that time.
		total := 0
		buf := make([]byte, 4<<10)
		for {
			n, err := conn.Read(buf)
			total += n
			if err != nil {
				break
			}
			time.Sleep(timeout / 8)
		}
		received <- total
	})
	conn := dialFakeHost(t, socket)

	var out syncBuffer
	require.NoError(t, pump(t.Context(), conn, bytes.NewReader(make([]byte, size)), &out, timeout))
	select {
	case total := <-received:
		require.Equal(t, size, total, "the host got everything")
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the host did not see the end")
	}
	require.Empty(t, out.String(), "no notice: the host was there all along")
}

// TestPumpKeepsAnIdleConnection verifies that the limit on writes is not
// one on the connection: nothing is read from the host for longer than
// the timeout, and nothing is written to it for longer either, and both
// directions still work afterwards.
func TestPumpKeepsAnIdleConnection(t *testing.T) {
	t.Parallel()

	const timeout = 150 * time.Millisecond
	got := make(chan string, 2)
	socket := fakeHost(t, func(conn net.Conn, _ Hello) {
		_, _ = conn.Write([]byte(`{"ok":true,"v":1}` + "\n"))
		buf := make([]byte, len("first"))
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		got <- string(buf)
		time.Sleep(4 * timeout)
		_, _ = conn.Write([]byte("late;"))
		if _, err := io.ReadFull(conn, buf); err != nil {
			return
		}
		got <- string(buf)
		_, _ = io.Copy(io.Discard, conn)
	})
	conn := dialFakeHost(t, socket)

	in, stdin := io.Pipe()
	t.Cleanup(func() { _ = stdin.Close() })
	var out syncBuffer
	done := make(chan error, 1)
	go func() { done <- pump(t.Context(), conn, in, &out, timeout) }()

	_, err := stdin.Write([]byte("first"))
	require.NoError(t, err)
	require.Equal(t, "first", <-got)

	// The host says nothing for several timeouts, then something.
	require.Eventually(t, func() bool { return out.String() == "late;" }, 10*time.Second, 5*time.Millisecond)

	// The client wrote nothing for several timeouts, then something.
	_, err = stdin.Write([]byte("again"))
	require.NoError(t, err)
	require.Equal(t, "again", <-got)

	require.NoError(t, stdin.Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		require.FailNow(t, "the pump did not end")
	}
	require.Equal(t, "late;", out.String())
}
