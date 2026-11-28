package server

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

// fakeBridge is a bridge for dial tests. Each control socket it accepts is
// delivered on controls; data sockets are delivered on data, keyed by id.
type fakeBridge struct {
	t        *testing.T
	srv      *httptest.Server
	controls chan *websocket.Conn
	data     chan dataSocket
	auth     chan string
	dials    atomic.Int32
}

type dataSocket struct {
	id   string
	conn *websocket.Conn
}

func newFakeBridge(t *testing.T) *fakeBridge {
	t.Helper()
	b := &fakeBridge{
		t:        t,
		controls: make(chan *websocket.Conn, 8),
		data:     make(chan dataSocket, 8),
		auth:     make(chan string, 16),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/tunnel/team/key/control", func(w http.ResponseWriter, r *http.Request) {
		b.dials.Add(1)
		b.auth <- r.Header.Get("Authorization")
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// The hijacked conn outlives the handler; the test owns it.
		b.controls <- c
	})
	mux.HandleFunc("/tunnel/team/key/data/{id}", func(w http.ResponseWriter, r *http.Request) {
		b.auth <- r.Header.Get("Authorization")
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		b.data <- dataSocket{id: r.PathValue("id"), conn: c}
	})
	b.srv = httptest.NewServer(mux)
	t.Cleanup(b.srv.Close)
	return b
}

func (b *fakeBridge) url() string {
	return "ws" + strings.TrimPrefix(b.srv.URL, "http") + "/tunnel/team/key"
}

func (b *fakeBridge) nextControl() *websocket.Conn {
	b.t.Helper()
	select {
	case c := <-b.controls:
		return c
	case <-time.After(5 * time.Second):
		b.t.Fatal("crush did not dial the control socket")
		return nil
	}
}

// readHello reads the first frame on c and checks it is a v1 hello.
func readHello(t *testing.T, c *websocket.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	typ, data, err := c.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, websocket.MessageText, typ)
	var hello map[string]any
	require.NoError(t, json.Unmarshal(data, &hello))
	require.Equal(t, "hello", hello["op"])
	require.EqualValues(t, 1, hello["v"])
	require.NotEmpty(t, hello["crush"])
}

func sendFrame(t *testing.T, c *websocket.Conn, frame string) {
	t.Helper()
	require.NoError(t, c.Write(t.Context(), websocket.MessageText, []byte(frame)))
}

var fastDial = dialConfig{backoffMin: 10 * time.Millisecond, backoffMax: 50 * time.Millisecond}

// startDialServer runs a dial server against b and returns ListenAndServe's
// result channel. A non-nil handler replaces the API.
func startDialServer(t *testing.T, b *fakeBridge) (*Server, <-chan error) {
	t.Helper()
	return startDialServerWith(t, b, fastDial, nil)
}

func startDialServerWith(t *testing.T, b *fakeBridge, cfg dialConfig, handler http.Handler) (*Server, <-chan error) {
	t.Helper()
	srv, err := NewDialServer(nil, b.url(), "secret")
	require.NoError(t, err)
	srv.dialCfg = cfg
	if handler != nil {
		srv.h.Handler = handler
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	t.Cleanup(func() { _ = srv.Close() })
	return srv, errc
}

// openData sends an open frame on c and returns the bridge's end of the
// data socket crush dials for it.
func openData(t *testing.T, b *fakeBridge, c *websocket.Conn, id int) net.Conn {
	t.Helper()
	sendFrame(t, c, fmt.Sprintf(`{"op":"open","id":%d}`, id))
	select {
	case ds := <-b.data:
		<-b.auth
		return websocket.NetConn(t.Context(), ds.conn, websocket.MessageBinary)
	case <-time.After(5 * time.Second):
		t.Fatal("crush did not dial the data socket")
		return nil
	}
}

// connectBridge waits for crush's control socket and its hello.
func connectBridge(t *testing.T, b *fakeBridge) *websocket.Conn {
	t.Helper()
	c := b.nextControl()
	t.Cleanup(func() { _ = c.CloseNow() })
	<-b.auth
	readHello(t, c)
	return c
}

// deadline bounds every later read and write on nc, so a hang fails the
// assertion that made the call instead of the whole test binary.
func deadline(t *testing.T, nc net.Conn, d time.Duration) {
	t.Helper()
	require.NoError(t, nc.SetDeadline(time.Now().Add(d)))
}

func waitErr(t *testing.T, errc <-chan error) error {
	t.Helper()
	select {
	case err := <-errc:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop")
		return nil
	}
}

func TestDial_HelloWithAuth(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	startDialServer(t, b)

	c := b.nextControl()
	defer c.CloseNow()
	require.Equal(t, "Bearer secret", <-b.auth)
	readHello(t, c)
}

func TestDial_OpenServesHTTP(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	startDialServer(t, b)

	c := b.nextControl()
	defer c.CloseNow()
	<-b.auth
	readHello(t, c)
	sendFrame(t, c, `{"op":"open","id":7}`)

	var ds dataSocket
	select {
	case ds = <-b.data:
	case <-time.After(5 * time.Second):
		t.Fatal("crush did not dial the data socket")
	}
	require.Equal(t, "7", ds.id)
	require.Equal(t, "Bearer secret", <-b.auth)

	nc := websocket.NetConn(t.Context(), ds.conn, websocket.MessageBinary)
	defer nc.Close()
	_, err := fmt.Fprint(nc, "GET /v1/health HTTP/1.1\r\nHost: crush\r\nOrigin: https://evil.example\r\n\r\n")
	require.NoError(t, err)
	res, err := http.ReadResponse(bufio.NewReader(nc), nil)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, 1, res.ProtoMinor)

	// The data socket survives the control socket dropping.
	c.CloseNow()
	_, err = fmt.Fprint(nc, "GET /v1/health HTTP/1.1\r\nHost: crush\r\n\r\n")
	require.NoError(t, err)
	res2, err := http.ReadResponse(bufio.NewReader(nc), nil)
	require.NoError(t, err)
	defer res2.Body.Close()
	require.Equal(t, http.StatusOK, res2.StatusCode)
}

func TestDial_ByeStopsCleanly(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	_, errc := startDialServer(t, b)

	c := b.nextControl()
	defer c.CloseNow()
	readHello(t, c)
	sendFrame(t, c, `{"op":"bye"}`)

	require.ErrorIs(t, waitErr(t, errc), ErrServerClosed)
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int32(1), b.dials.Load(), "crush redialed after bye")
}

func TestDial_SupersededStopsCleanly(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	_, errc := startDialServer(t, b)

	c := b.nextControl()
	readHello(t, c)
	_ = c.Close(dialCloseSuperseded, "superseded")

	require.ErrorIs(t, waitErr(t, errc), ErrServerClosed)
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int32(1), b.dials.Load(), "crush redialed after superseded")
}

func TestDial_RedialsAfterDrop(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	startDialServer(t, b)

	c := b.nextControl()
	readHello(t, c)
	c.CloseNow()

	c2 := b.nextControl()
	defer c2.CloseNow()
	readHello(t, c2)
	require.Equal(t, int32(2), b.dials.Load())
}

func TestDial_UnsupportedVersionIsFatal(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	_, errc := startDialServer(t, b)

	c := b.nextControl()
	readHello(t, c)
	_ = c.Close(dialCloseUnsupported, "unsupported version")

	err := waitErr(t, errc)
	require.ErrorIs(t, err, ErrDialUnsupported)
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int32(1), b.dials.Load(), "crush redialed after 4001")
}

func TestDialListener_CloseStopsAccept(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	l, err := newDialListener(b.url(), "secret", fastDial, nil, nil)
	require.NoError(t, err)
	l.start()
	c := b.nextControl()
	defer c.CloseNow()
	readHello(t, c)

	var wg sync.WaitGroup
	wg.Add(1)
	var acceptErr error
	go func() {
		defer wg.Done()
		_, acceptErr = l.Accept()
	}()
	require.NoError(t, l.Close())
	wg.Wait()
	require.True(t, errors.Is(acceptErr, net.ErrClosed))
	require.Equal(t, NetworkDial, l.Addr().Network())
}

func TestNewDialServer_Validation(t *testing.T) {
	t.Parallel()
	_, err := NewDialServer(nil, "wss://bridge.example/tunnel/t/k", "")
	require.Error(t, err)
	_, err = NewDialServer(nil, "https://bridge.example/tunnel/t/k", "tok")
	require.Error(t, err)
	_, err = NewDialServer(nil, "wss://user:pw@bridge.example/tunnel/t/k", "tok")
	require.Error(t, err)
}

// bigBody is a response large enough to span many websocket frames.
var bigBody = strings.Repeat("0123456789abcdef", 400<<10/16)

func bigHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /big", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(bigBody)))
		_, _ = io.WriteString(w, bigBody)
	})
	return mux
}

func TestDial_LargeResponseDeliveredBeforeClose(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	startDialServerWith(t, b, fastDial, bigHandler())
	c := connectBridge(t, b)
	nc := openData(t, b, c, 1)
	defer nc.Close()

	deadline(t, nc, 10*time.Second)
	_, err := io.WriteString(nc, "GET /big HTTP/1.1\r\nHost: crush\r\nConnection: close\r\n\r\n")
	require.NoError(t, err)
	br := bufio.NewReader(nc)
	res, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, bigBody, string(body))
	// Crush closes the websocket normally once the response is out.
	rest, err := io.ReadAll(br)
	require.NoError(t, err)
	require.Empty(t, rest)
}

func TestDial_ByeDrainsActiveDataSocket(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseHandler := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseHandler)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-release
		w.Header().Set("Content-Length", strconv.Itoa(len(bigBody)))
		_, _ = io.WriteString(w, bigBody)
	})
	srv, errc := startDialServerWith(t, b, fastDial, mux)
	c := connectBridge(t, b)
	nc := openData(t, b, c, 1)
	defer nc.Close()

	_, err := io.WriteString(nc, "GET /slow HTTP/1.1\r\nHost: crush\r\n\r\n")
	require.NoError(t, err)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach the handler")
	}
	sendFrame(t, c, `{"op":"bye"}`)
	require.Eventually(t, srv.selfShutdownStarted.Load, 5*time.Second, 5*time.Millisecond)
	releaseHandler()

	deadline(t, nc, 10*time.Second)
	br := bufio.NewReader(nc)
	res, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, len(bigBody), len(body))
	// The now idle connection is closed normally.
	_, err = io.ReadAll(br)
	require.NoError(t, err)
	require.ErrorIs(t, waitErr(t, errc), ErrServerClosed)
}

func TestDial_CloseCutsOffActiveDataSocket(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	entered := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hang", func(http.ResponseWriter, *http.Request) {
		close(entered)
		<-t.Context().Done()
	})
	srv, _ := startDialServerWith(t, b, fastDial, mux)
	c := connectBridge(t, b)
	nc := openData(t, b, c, 1)
	defer nc.Close()

	_, err := io.WriteString(nc, "GET /hang HTTP/1.1\r\nHost: crush\r\n\r\n")
	require.NoError(t, err)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach the handler")
	}
	closed := make(chan error, 1)
	go func() { closed <- srv.Close() }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return promptly")
	}
	start := time.Now()
	deadline(t, nc, 5*time.Second)
	_, err = nc.Read(make([]byte, 1))
	require.Error(t, err)
	require.Less(t, time.Since(start), 2*time.Second, "the data socket outlived Close")
}

func TestDial_BackoffGrowsAcrossQuickDrops(t *testing.T) {
	t.Parallel()
	base := 10 * time.Millisecond
	ceil := 80 * time.Millisecond
	for _, tc := range []struct {
		name        string
		stableAfter time.Duration
		want        []time.Duration
	}{
		// Drops right after the hello do not count as a stable socket.
		{"quick drops", time.Hour, []time.Duration{base, 2 * base, 4 * base, ceil, ceil, ceil}},
		// A socket up for stableAfter resets the backoff.
		{"stable drops", time.Nanosecond, []time.Duration{base, base, base, base}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newFakeBridge(t)
			waits := make(chan time.Duration, 16)
			cfg := dialConfig{
				backoffMin:  base,
				backoffMax:  ceil,
				stableAfter: tc.stableAfter,
				sleep: func(ctx context.Context, d time.Duration) bool {
					select {
					case waits <- d:
					default:
					}
					return ctx.Err() == nil
				},
			}
			startDialServerWith(t, b, cfg, nil)
			// Drop every control socket right after its hello.
			stop := make(chan struct{})
			var dropper sync.WaitGroup
			dropper.Go(func() {
				for {
					select {
					case <-stop:
						return
					case c := <-b.controls:
						<-b.auth
						_, _, _ = c.Read(t.Context())
						time.Sleep(time.Millisecond)
						_ = c.CloseNow()
					}
				}
			})
			t.Cleanup(func() {
				close(stop)
				dropper.Wait()
			})
			for i, want := range tc.want {
				select {
				case got := <-waits:
					require.GreaterOrEqual(t, got, want, "wait %d", i)
					require.LessOrEqual(t, got, min(want+want/5, ceil), "wait %d", i)
				case <-time.After(5 * time.Second):
					t.Fatalf("no redial %d", i)
				}
			}
		})
	}
}

func TestDial_StalledRequestBodyAbortsRelay(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /ignore", func(http.ResponseWriter, *http.Request) {
		<-t.Context().Done() // never reads the body
	})
	cfg := fastDial
	cfg.relayBuffer = relayChunk
	cfg.relayStall = 100 * time.Millisecond
	startDialServerWith(t, b, cfg, mux)
	c := connectBridge(t, b)
	nc := openData(t, b, c, 1)
	defer nc.Close()

	const size = 8 << 20
	go func() {
		_, _ = io.WriteString(nc, "POST /ignore HTTP/1.1\r\nHost: crush\r\nContent-Length: "+strconv.Itoa(size)+"\r\n\r\n")
		chunk := make([]byte, 64<<10)
		for range size / len(chunk) {
			if _, err := nc.Write(chunk); err != nil {
				return
			}
		}
	}()
	start := time.Now()
	deadline(t, nc, 10*time.Second)
	_, err := nc.Read(make([]byte, 1))
	require.Error(t, err)
	require.Less(t, time.Since(start), 5*time.Second, "the stalled relay was not aborted")
}

func TestDial_UnreadResponseAbortsRelay(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	const size = 256 << 20
	mux := http.NewServeMux()
	mux.HandleFunc("GET /flood", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(size))
		chunk := make([]byte, 64<<10)
		for range size / len(chunk) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})
	cfg := fastDial
	cfg.relayStall = 100 * time.Millisecond
	startDialServerWith(t, b, cfg, mux)
	c := connectBridge(t, b)
	nc := openData(t, b, c, 1)
	defer nc.Close()

	deadline(t, nc, 10*time.Second)
	_, err := io.WriteString(nc, "GET /flood HTTP/1.1\r\nHost: crush\r\n\r\n")
	require.NoError(t, err)
	// Stop reading for well past the write bound; crush must give up.
	time.Sleep(time.Second)
	start := time.Now()
	n, _ := io.Copy(io.Discard, nc)
	require.Less(t, n, int64(size), "the whole response arrived; the relay never aborted")
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestDial_SecondListenAndServeFails(t *testing.T) {
	t.Parallel()
	b := newFakeBridge(t)
	srv, _ := startDialServer(t, b)
	connectBridge(t, b)
	require.ErrorContains(t, srv.ListenAndServe(), "already started")
	require.Equal(t, int32(1), b.dials.Load())
}
