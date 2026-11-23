// Package lsphost speaks the LSP host protocol: how Crush reaches the
// language servers of a host application, such as an editor that runs
// Crush in one of its panes, instead of starting its own.
//
// The host serves its language servers on a unix socket and announces the
// socket to Crush (see proto.LSPHost). A connection starts with one JSON
// line each way, the [Hello] and its [Ack], and then carries the LSP base
// protocol unchanged.
//
// Crush's LSP library can only start a language server as a process, so a
// hosted server is the hidden `crush __lsp-dial` command ([Main]): it dials
// the socket, says hello, and then copies its stdin and stdout to and from
// the socket.
package lsphost

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/version"
)

const (
	// Protocol is the version of the hello and of the rules that follow it.
	Protocol = 1
	// MaxLine is the longest hello or ack, newline included.
	MaxLine = 4096
	// DialTimeout bounds connecting to the host's socket.
	DialTimeout = time.Second
	// HelloTimeout bounds the exchange of hello and ack.
	HelloTimeout = 2 * time.Second
	// WriteTimeout is how long the host may go without taking any of
	// what it is sent before [Pump] gives up on it. A host that is
	// suspended, or stuck, stops reading; without a limit the LSP client
	// would block on its next write for as long as that lasts.
	WriteTimeout = 5 * time.Second

	// Command is the hidden crush subcommand that runs [Main].
	Command = "__lsp-dial"
	// ClientName is how Crush names itself in the hello.
	ClientName = "crush"

	// ClosedMethod is the notification [Pump] writes to the LSP client
	// when the host closes the connection. A client only learns that its
	// server process went away the next time it writes to it; this tells
	// it at once.
	ClosedMethod = "$/crush/hostClosed"

	addressScheme = "unix://"
)

// ErrHostClosed is returned by [Pump] when the host closed the connection.
var ErrHostClosed = errors.New("the host closed the connection")

// Hello is the line a client sends first, before any LSP message.
type Hello struct {
	LSPHost       int      `json:"lsp_host"`
	Server        string   `json:"server"`
	Client        string   `json:"client,omitempty"`
	ClientVersion string   `json:"client_version,omitempty"`
	Workspace     string   `json:"workspace,omitempty"`
	Root          string   `json:"root,omitempty"`
	PID           int      `json:"pid"`
	Features      []string `json:"features,omitempty"`
}

// Ack is the host's answer to a hello. The LSP session follows when OK is
// set; otherwise the host closes the connection.
type Ack struct {
	OK       bool     `json:"ok"`
	V        int      `json:"v,omitempty"`
	Host     string   `json:"host,omitempty"`
	PID      int      `json:"pid,omitempty"`
	Features []string `json:"features,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// SocketPath returns the socket a host address names. An address is
// "unix://" followed by an absolute path.
func SocketPath(address string) (string, error) {
	path, ok := strings.CutPrefix(address, addressScheme)
	if !ok {
		return "", fmt.Errorf("address %q is not a unix:// address", address)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("address %q has no absolute path", address)
	}
	return path, nil
}

// Address returns the host address of a socket.
func Address(socket string) string {
	return addressScheme + socket
}

// Dial connects to the host listening on socket and says hello. It returns
// the connection once the host accepted it, with nothing read past the ack,
// so the LSP session can start on it right away.
func Dial(ctx context.Context, socket string, hello Hello) (net.Conn, Ack, error) {
	hello.LSPHost = Protocol
	line, err := json.Marshal(hello)
	if err != nil {
		return nil, Ack{}, fmt.Errorf("encoding hello: %w", err)
	}
	line = append(line, '\n')
	if len(line) > MaxLine {
		return nil, Ack{}, errors.New("hello is too long")
	}

	dialer := net.Dialer{Timeout: DialTimeout}
	conn, err := dialer.DialContext(ctx, "unix", socket)
	if err != nil {
		return nil, Ack{}, fmt.Errorf("connecting to the host: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(HelloTimeout))
	if _, err := conn.Write(line); err != nil {
		_ = conn.Close()
		return nil, Ack{}, fmt.Errorf("sending hello: %w", err)
	}
	reply, err := readLine(conn, MaxLine)
	if err != nil {
		_ = conn.Close()
		return nil, Ack{}, fmt.Errorf("reading the host's answer: %w", err)
	}
	var ack Ack
	if err := json.Unmarshal(reply, &ack); err != nil {
		_ = conn.Close()
		return nil, Ack{}, fmt.Errorf("invalid answer from the host: %w", err)
	}
	switch {
	case !ack.OK:
		_ = conn.Close()
		return nil, ack, fmt.Errorf("the host refused: %s", cmp.Or(ack.Error, "no reason given"))
	case ack.V != 0 && ack.V != Protocol:
		_ = conn.Close()
		return nil, ack, fmt.Errorf("the host speaks protocol %d, not %d", ack.V, Protocol)
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, ack, nil
}

// readLine reads up to a newline, which it drops. It reads one byte at a
// time so that nothing after the line is consumed: the LSP session follows
// on the same connection.
func readLine(r io.Reader, limit int) ([]byte, error) {
	var line []byte
	b := make([]byte, 1)
	for len(line) < limit {
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, err //nolint:wrapcheck
		}
		if b[0] == '\n' {
			return line, nil
		}
		line = append(line, b[0])
	}
	return nil, errors.New("line too long")
}

// closedNotice is the framed [ClosedMethod] notification. Params are
// present because Crush's LSP library reads them without a nil check.
var closedNotice = func() []byte {
	body := `{"jsonrpc":"2.0","method":"` + ClosedMethod + `","params":{}}`
	return []byte("Content-Length: " + strconv.Itoa(len(body)) + "\r\n\r\n" + body)
}()

// hostWriter writes to the host and gives up when the host takes nothing
// for timeout. A host that reads slowly is not given up on, however much
// there is to write: the limit starts over whenever it takes something.
// Only writes are limited; a connection may stay idle for as long as it
// likes.
type hostWriter struct {
	conn    net.Conn
	timeout time.Duration
}

func (w hostWriter) Write(p []byte) (int, error) {
	written := 0
	for {
		_ = w.conn.SetWriteDeadline(time.Now().Add(w.timeout))
		n, err := w.conn.Write(p[written:])
		written += n
		if err == nil || n == 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
			return written, err //nolint:wrapcheck
		}
	}
}

// Pump copies in to conn and conn to out until one side ends, and closes
// conn. It returns nil when in ends, which is how the LSP client stops its
// server, and [ErrHostClosed] when the host closes conn or stops reading
// for [WriteTimeout], after telling the client so with a [ClosedMethod]
// notification.
func Pump(ctx context.Context, conn net.Conn, in io.Reader, out io.Writer) error {
	return pump(ctx, conn, in, out, WriteTimeout)
}

func pump(ctx context.Context, conn net.Conn, in io.Reader, out io.Writer, writeTimeout time.Duration) error {
	defer conn.Close()

	fromHost := make(chan struct{})
	go func() {
		defer close(fromHost)
		_, _ = io.Copy(out, conn)
	}()
	toHost := make(chan error, 1)
	go func() {
		_, err := io.Copy(hostWriter{conn: conn, timeout: writeTimeout}, in)
		toHost <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-toHost:
		if err == nil {
			return nil
		}
		// Writing to the host failed: it is gone, or it stopped reading.
		// Stop the other copy before writing the notice, so the two
		// cannot interleave.
		_ = conn.Close()
		<-fromHost
	case <-fromHost:
	}
	// If the host was cut off in the middle of a message the notice is
	// unreadable, and the client finds out on its next write instead.
	_, _ = out.Write(closedNotice)
	return ErrHostClosed
}

// Args returns the arguments of the crush command that connects an LSP
// client to the server named server on the host's socket: [Command] and
// its flags. The hello is built from them by [Main].
func Args(socket, server, root, workspace string, pid int) []string {
	return []string{
		Command,
		"--address", Address(socket),
		"--server", server,
		"--root", root,
		"--workspace", workspace,
		"--pid", strconv.Itoa(pid),
	}
}

// Main is the `crush __lsp-dial` command, given the arguments after the
// command's name. It connects to the host, says hello, and pumps in and
// out to and from the host until either ends. It returns the exit code: 0
// when in ended, and non-zero when the host could not be reached, refused,
// or closed the connection, so that the LSP client sees its server fail.
//
// Nothing is written to errOut unless connecting failed: Crush logs what a
// language server writes to its stderr as an error.
func Main(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	fs := flag.NewFlagSet(Command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	var (
		address   = fs.String("address", "", "host address (unix:///absolute/path)")
		server    = fs.String("server", "", "the host's name for the language server")
		root      = fs.String("root", "", "project root")
		workspace = fs.String("workspace", "", "workspace ID")
		pid       = fs.Int("pid", os.Getppid(), "PID of the Crush process the server is for")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	socket, err := SocketPath(*address)
	if err != nil || *server == "" {
		fmt.Fprintln(errOut, "crush: lsp host: --address and --server are required")
		return 2
	}

	conn, _, err := Dial(ctx, socket, Hello{
		Server:        *server,
		Client:        ClientName,
		ClientVersion: version.Version,
		Workspace:     *workspace,
		Root:          *root,
		PID:           *pid,
	})
	if err != nil {
		fmt.Fprintf(errOut, "crush: lsp host: %s: %v\n", *server, err)
		return 1
	}
	if err := Pump(ctx, conn, in, out); err != nil {
		return 1
	}
	return 0
}
