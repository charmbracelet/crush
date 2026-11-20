package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/crush/internal/version"
	"github.com/coder/websocket"
)

// NetworkDial is the network of a server that dials out to a bridge over
// websockets instead of listening, for sandboxes that accept no inbound
// connections. The server's address is the bridge's tunnel URL.
const NetworkDial = "dial"

// DialTokenEnv names the environment variable holding the bearer token a
// dialing server presents to the bridge.
const DialTokenEnv = "CRUSH_SERVER_DIAL_TOKEN"

// dialProtocolVersion is the tunnel protocol version sent in the hello.
const dialProtocolVersion = 1

// Close codes the bridge sends on the control socket.
const (
	// dialCloseSuperseded means another crush took over the tunnel.
	dialCloseSuperseded websocket.StatusCode = 4000
	// dialCloseUnsupported means the bridge does not speak our protocol
	// version.
	dialCloseUnsupported websocket.StatusCode = 4001
)

// ErrDialUnsupported is returned when the bridge rejects the tunnel
// protocol version. Redialing cannot help, so the server stops.
var ErrDialUnsupported = errors.New("bridge does not support this tunnel protocol version")

// ParseDialURL validates a bridge tunnel URL, which must use the ws or wss
// scheme and carry no credentials.
func ParseDialURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid dial url: %w", err)
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("invalid dial url %q: scheme must be ws or wss", raw)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid dial url %q: missing host", raw)
	}
	if u.User != nil {
		return nil, fmt.Errorf("invalid dial url: credentials belong in $%s, not the url", DialTokenEnv)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	u.Fragment = ""
	return u, nil
}

// dialConfig tunes a [dialListener]. Zero fields take defaults.
type dialConfig struct {
	// backoffMin and backoffMax bound the wait between control redials.
	backoffMin time.Duration
	backoffMax time.Duration
	// stableAfter is how long a control socket must stay up after its
	// hello for the backoff to reset.
	stableAfter time.Duration
	// upgradeTimeout bounds each websocket dial and upgrade.
	upgradeTimeout time.Duration
	// relayBuffer bounds the bytes read from a data socket that the HTTP
	// side has not consumed yet.
	relayBuffer int
	// relayStall is how long a full relay buffer may block the data
	// socket's reader, or one websocket write may take, before the relay
	// is aborted.
	relayStall time.Duration
	// closeTimeout bounds a websocket close handshake.
	closeTimeout time.Duration
	// sleep waits d, returning false if ctx ends first. Tests replace it.
	sleep func(ctx context.Context, d time.Duration) bool
}

func (c dialConfig) withDefaults() dialConfig {
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&c.backoffMin, time.Second)
	def(&c.backoffMax, 30*time.Second)
	def(&c.stableAfter, 10*time.Second)
	def(&c.upgradeTimeout, 15*time.Second)
	def(&c.relayStall, 30*time.Second)
	def(&c.closeTimeout, 5*time.Second)
	if c.relayBuffer <= 0 {
		c.relayBuffer = 1 << 20
	}
	if c.sleep == nil {
		c.sleep = sleepCtx
	}
	return c
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// relayChunk is the most a relay reads or writes at once.
const relayChunk = 32 << 10

// dialAddr is the [net.Addr] of a [dialListener].
type dialAddr string

func (dialAddr) Network() string  { return NetworkDial }
func (a dialAddr) String() string { return string(a) }

// dialListener is a [net.Listener] whose connections are websockets it
// dials out to a bridge. A control socket at <url>/control carries JSON
// frames; for each {"op":"open","id":N} the listener dials <url>/data/N
// and hands the result to Accept as a byte stream.
//
// The control socket is redialed with backoff when it drops. Connections
// already accepted are independent of it and keep running until the HTTP
// server closes them, [dialListener.drain] waits for them or
// [dialListener.abortRelays] cuts them off.
type dialListener struct {
	base   *url.URL
	header http.Header
	client *http.Client
	cfg    dialConfig
	logger *slog.Logger
	// onStop is called once the bridge asks crush to exit (a bye or a
	// superseded close), after redialing has stopped. It must close the
	// listener eventually; the server does so from a graceful shutdown,
	// so Accept keeps working until the server is shutting down and
	// [http.Server.Serve] reports ErrServerClosed rather than an accept
	// error.
	onStop func()

	ctx    context.Context
	cancel context.CancelFunc
	conns  chan net.Conn
	done   chan struct{}

	closeOnce sync.Once
	mu        sync.Mutex
	err       error // returned by Accept once done is closed

	relaysMu sync.Mutex
	relays   map[*relay]struct{}
	aborted  bool // abortRelays ran; new relays are aborted at once
}

func newDialListener(rawURL, token string, cfg dialConfig, logger *slog.Logger, onStop func()) (*dialListener, error) {
	base, err := ParseDialURL(rawURL)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("dialing %s needs a token in $%s", base.Redacted(), DialTokenEnv)
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	l := &dialListener{
		base:   base,
		header: http.Header{"Authorization": {"Bearer " + token}},
		client: newTunnelClient(),
		cfg:    cfg.withDefaults(),
		logger: logger.With("tunnel", base.Redacted()),
		onStop: onStop,
		ctx:    ctx,
		cancel: cancel,
		conns:  make(chan net.Conn),
		done:   make(chan struct{}),
		relays: make(map[*relay]struct{}),
	}
	if l.onStop == nil {
		l.onStop = func() { _ = l.Close() }
	}
	return l, nil
}

// start starts dialing the bridge.
func (l *dialListener) start() {
	go l.run()
}

// rawConnKey keys the [rawConnSlot] a tunnel dial records its connection in.
type rawConnKey struct{}

// rawConnSlot receives the network connection under a websocket, so it can
// be closed to cut the websocket off: [websocket.Conn.CloseNow] waits out
// a close handshake already in progress.
type rawConnSlot struct {
	mu       sync.Mutex
	conn     net.Conn
	canceled bool
}

// set records c, closing it instead if the dial has already given up.
func (s *rawConnSlot) set(c net.Conn) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.canceled {
		_ = c.Close()
		return net.ErrClosed
	}
	s.conn = c
	return nil
}

// take returns the recorded conn; with cancel, later conns are closed.
func (s *rawConnSlot) take(cancel bool) net.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.canceled = s.canceled || cancel
	return s.conn
}

// newTunnelClient returns the HTTP client for tunnel dials. It never
// follows redirects, and never reuses a connection, so each dial's
// connection is its own and can be recorded in its [rawConnSlot].
func newTunnelClient() *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				c, err := d.DialContext(ctx, network, addr)
				if slot, ok := ctx.Value(rawConnKey{}).(*rawConnSlot); ok && err == nil {
					if err := slot.set(c); err != nil {
						return nil, err
					}
				}
				return c, err
			},
			TLSHandshakeTimeout: 10 * time.Second,
			DisableKeepAlives:   true,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// dialWS dials the websocket at path under the tunnel URL, bounded by the
// upgrade timeout, and returns it with its network connection. On failure
// status is the HTTP status of a rejected upgrade, or zero.
func (l *dialListener) dialWS(path string) (_ *websocket.Conn, _ net.Conn, status int, _ error) {
	slot := new(rawConnSlot)
	ctx, cancel := context.WithTimeout(l.ctx, l.cfg.upgradeTimeout)
	defer cancel()
	ctx = context.WithValue(ctx, rawConnKey{}, slot)
	c, resp, err := websocket.Dial(ctx, l.url(path), &websocket.DialOptions{
		HTTPClient: l.client,
		HTTPHeader: l.header,
	})
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if raw := slot.take(true); raw != nil {
			_ = raw.Close()
		}
		if resp != nil {
			status = resp.StatusCode
		}
		return nil, nil, status, err
	}
	raw := slot.take(false)
	if raw == nil {
		_ = c.CloseNow()
		return nil, nil, 0, errors.New("tunnel dial recorded no connection")
	}
	return c, raw, 0, nil
}

// closeWS closes c with a normal close handshake, cutting raw off if the
// handshake takes longer than the close timeout.
func (l *dialListener) closeWS(c *websocket.Conn, raw net.Conn) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Close(websocket.StatusNormalClosure, "")
	}()
	t := time.NewTimer(l.cfg.closeTimeout)
	defer t.Stop()
	select {
	case <-done:
	case <-t.C:
		_ = raw.Close()
		<-done
	}
}

// Accept waits for the next data connection.
func (l *dialListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		return c, nil
	case <-l.done:
		l.mu.Lock()
		defer l.mu.Unlock()
		return nil, l.err
	}
}

// Close stops redialing. Accepted connections are left alone.
func (l *dialListener) Close() error {
	l.stop(net.ErrClosed)
	return nil
}

// Addr returns the tunnel URL.
func (l *dialListener) Addr() net.Addr {
	return dialAddr(l.base.Redacted())
}

// stop closes the listener, making Accept return err. Only the first call
// sets the error.
func (l *dialListener) stop(err error) {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.err = err
		l.mu.Unlock()
		l.cancel()
		close(l.done)
	})
}

func (l *dialListener) url(path string) string {
	u := *l.base
	u.Path += path
	return u.String()
}

// drain waits for the live relays to finish, aborting those left when ctx
// ends.
func (l *dialListener) drain(ctx context.Context) error {
	for {
		l.relaysMu.Lock()
		var r *relay
		for r = range l.relays {
			break
		}
		l.relaysMu.Unlock()
		if r == nil {
			return nil
		}
		select {
		case <-r.finished:
		case <-ctx.Done():
			l.abortRelays()
			return ctx.Err()
		}
	}
}

// abortRelays cuts off every live relay, and any started later.
func (l *dialListener) abortRelays() {
	l.relaysMu.Lock()
	l.aborted = true
	relays := make([]*relay, 0, len(l.relays))
	for r := range l.relays {
		relays = append(relays, r)
	}
	l.relaysMu.Unlock()
	for _, r := range relays {
		r.abort("server closed")
	}
}

// controlOutcome says what to do after a control socket ends.
type controlOutcome int

const (
	controlRedial controlOutcome = iota
	controlExit                  // bye or superseded: stop and exit cleanly
	controlFatal                 // unsupported version: stop with an error
	controlClosed                // the listener was closed
)

// run keeps a control socket up until the listener closes or the bridge
// tells crush to stop.
func (l *dialListener) run() {
	backoff := l.cfg.backoffMin
	for {
		outcome, upFor := l.control()
		switch outcome {
		case controlClosed:
			return
		case controlExit:
			l.onStop()
			return
		case controlFatal:
			l.stop(ErrDialUnsupported)
			l.abortRelays()
			return
		}
		// Only a socket that stayed up resets the backoff, so a bridge
		// that drops crush right after each hello is not hammered.
		if upFor >= l.cfg.stableAfter {
			backoff = l.cfg.backoffMin
		}
		// Up to 20% jitter so many sandboxes do not redial in lockstep.
		wait := backoff + time.Duration(rand.Int64N(int64(backoff)/5+1))
		wait = min(wait, l.cfg.backoffMax)
		l.logger.Info("Tunnel control socket lost, redialing", "in", wait)
		if !l.cfg.sleep(l.ctx, wait) {
			return
		}
		backoff = min(backoff*2, l.cfg.backoffMax)
	}
}

// controlFrame is a frame on the control socket.
type controlFrame struct {
	Op    string  `json:"op"`
	ID    *uint32 `json:"id,omitempty"`
	V     int     `json:"v,omitempty"`
	Crush string  `json:"crush,omitempty"`
}

// control dials the control socket, says hello and serves its frames until
// it ends. upFor is how long the socket stayed up after the hello, zero
// if it never went out.
func (l *dialListener) control() (outcome controlOutcome, upFor time.Duration) {
	c, _, status, err := l.dialWS("/control")
	if err != nil {
		if l.ctx.Err() != nil {
			return controlClosed, 0
		}
		l.logger.Warn("Failed to dial tunnel control socket", "status", status)
		return controlRedial, 0
	}
	defer c.CloseNow()

	hello, _ := json.Marshal(controlFrame{Op: "hello", V: dialProtocolVersion, Crush: version.Version})
	if err := c.Write(l.ctx, websocket.MessageText, hello); err != nil {
		return l.controlEnded(err), 0
	}
	helloAt := time.Now()
	l.logger.Info("Tunnel connected")

	for {
		typ, data, err := c.Read(l.ctx)
		if err != nil {
			return l.controlEnded(err), time.Since(helloAt)
		}
		if typ != websocket.MessageText {
			l.logger.Warn("Ignoring binary frame on tunnel control socket")
			continue
		}
		var f controlFrame
		if err := json.Unmarshal(data, &f); err != nil {
			l.logger.Warn("Ignoring malformed tunnel control frame")
			continue
		}
		switch f.Op {
		case "open":
			if f.ID == nil {
				l.logger.Warn("Ignoring tunnel open frame without an id")
				continue
			}
			go l.dialData(*f.ID)
		case "bye":
			l.logger.Info("Tunnel bridge said bye")
			return controlExit, time.Since(helloAt)
		default:
			l.logger.Warn("Ignoring unknown tunnel control op")
		}
	}
}

// controlEnded classifies the error that ended a control socket.
func (l *dialListener) controlEnded(err error) controlOutcome {
	if l.ctx.Err() != nil {
		return controlClosed
	}
	var ce websocket.CloseError
	if !errors.As(err, &ce) {
		l.logger.Warn("Tunnel control socket lost")
		return controlRedial
	}
	switch ce.Code {
	case dialCloseUnsupported:
		l.logger.Error("Tunnel bridge does not support this protocol version", "version", dialProtocolVersion)
		return controlFatal
	case dialCloseSuperseded:
		l.logger.Info("Tunnel superseded by another crush")
		return controlExit
	}
	l.logger.Warn("Tunnel control socket closed", "code", int(ce.Code))
	return controlRedial
}

// dialData dials the data socket for id and hands it to Accept. A failure
// only loses that connection.
func (l *dialListener) dialData(id uint32) {
	c, raw, status, err := l.dialWS("/data/" + strconv.FormatUint(uint64(id), 10))
	if err != nil {
		if l.ctx.Err() == nil {
			l.logger.Warn("Failed to dial tunnel data socket", "id", id, "status", status)
		}
		return
	}
	r := l.startRelay(id, c, raw)
	if r == nil {
		return
	}
	select {
	case l.conns <- r.conn:
	case <-l.done:
		r.abort("")
	}
}

// relay carries a data socket's bytes to and from one end of a
// [net.Pipe], whose other end the HTTP server serves.
//
// net/http interrupts reads by setting a deadline in the past, and on a
// websocket a deadline that hits an active read closes the whole socket,
// which would end every keep-alive connection after its first response.
// A pipe honours deadlines without closing anything, so the HTTP server
// never touches the websocket.
//
// Three goroutines move the bytes: readWS reads frames into a bounded
// channel, writePipe drains that into the pipe, and writeWS copies the
// HTTP side's output to the websocket. readWS stays ready to read, and so
// to answer the bridge's pings, unless the channel is full; it waits at
// most relayStall for room before giving up on the relay.
//
// A websocket close is a full close: there is no half-close, so whichever
// side closes first ends the connection both ways.
type relay struct {
	l      *dialListener
	id     uint32
	ws     *websocket.Conn
	raw    net.Conn // the network connection under ws
	conn   net.Conn // the HTTP server's end of the pipe
	remote net.Conn // the relay's end of the pipe
	chunks chan []byte

	closing   atomic.Bool // we started the close handshake
	abortOnce sync.Once
	aborted   chan struct{}
	finished  chan struct{} // all three goroutines have returned
}

// startRelay starts relaying ws, returning nil if the listener's relays
// have been aborted.
func (l *dialListener) startRelay(id uint32, ws *websocket.Conn, raw net.Conn) *relay {
	ws.SetReadLimit(-1)
	local, remote := net.Pipe()
	r := &relay{
		l:        l,
		id:       id,
		ws:       ws,
		raw:      raw,
		conn:     &dialConn{Conn: local, addr: l.Addr()},
		remote:   remote,
		chunks:   make(chan []byte, max(1, l.cfg.relayBuffer/relayChunk)),
		aborted:  make(chan struct{}),
		finished: make(chan struct{}),
	}
	l.relaysMu.Lock()
	if l.aborted {
		l.relaysMu.Unlock()
		r.abort("")
		return nil
	}
	l.relays[r] = struct{}{}
	l.relaysMu.Unlock()

	var wg sync.WaitGroup
	wg.Go(r.readWS)
	wg.Go(r.writePipe)
	wg.Go(r.writeWS)
	go func() {
		wg.Wait()
		// Everything has stopped; release what is left.
		_ = r.conn.Close()
		_ = r.raw.Close()
		_ = r.ws.CloseNow()
		l.relaysMu.Lock()
		delete(l.relays, r)
		l.relaysMu.Unlock()
		close(r.finished)
	}()
	return r
}

// abort cuts the relay off both ways, logging reason unless it is empty.
func (r *relay) abort(reason string) {
	r.abortOnce.Do(func() {
		if reason != "" {
			r.l.logger.Warn("Tunnel data socket aborted", "id", r.id, "reason", reason)
		}
		close(r.aborted)
		_ = r.remote.Close()
		_ = r.conn.Close()
		_ = r.raw.Close()
	})
}

// readWS reads the bridge's bytes into r.chunks.
func (r *relay) readWS() {
	defer close(r.chunks)
	for {
		typ, rd, err := r.ws.Reader(context.Background())
		if err != nil {
			// The bridge closing the websocket ends the connection at
			// once; there is no half-close to drain through.
			if !r.closing.Load() {
				if websocket.CloseStatus(err) == -1 {
					r.abort("read failed")
				} else {
					r.abort("")
				}
			}
			return
		}
		if typ != websocket.MessageBinary {
			r.abort("unexpected text frame")
			return
		}
		for {
			buf := make([]byte, relayChunk)
			n, err := rd.Read(buf)
			if n > 0 && !r.push(buf[:n]) {
				return
			}
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				if !r.closing.Load() {
					r.abort("read failed")
				}
				return
			}
		}
	}
}

// push queues b for the HTTP side, waiting at most relayStall for room.
func (r *relay) push(b []byte) bool {
	select {
	case r.chunks <- b:
		return true
	case <-r.aborted:
		return false
	default:
	}
	t := time.NewTimer(r.l.cfg.relayStall)
	defer t.Stop()
	select {
	case r.chunks <- b:
		return true
	case <-r.aborted:
		return false
	case <-t.C:
		r.abort("request body not consumed in time")
		return false
	}
}

// writePipe hands the bridge's bytes to the HTTP side.
func (r *relay) writePipe() {
	discard := false
	for b := range r.chunks {
		if discard {
			continue
		}
		if _, err := r.remote.Write(b); err != nil {
			// The pipe is closed: the HTTP side is done with the
			// connection, and writeWS closes the websocket. Keep
			// draining so readWS never blocks on a full channel.
			discard = true
		}
	}
}

// writeWS sends the HTTP side's bytes to the bridge, then closes the
// websocket once the HTTP side closes its end.
func (r *relay) writeWS() {
	buf := make([]byte, relayChunk)
	for {
		n, err := r.remote.Read(buf)
		if n > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), r.l.cfg.relayStall)
			werr := r.ws.Write(ctx, websocket.MessageBinary, buf[:n])
			cancel()
			if werr != nil {
				r.abort("response not consumed in time or write failed")
				return
			}
		}
		if errors.Is(err, io.EOF) {
			// Everything the HTTP side wrote has gone out.
			r.closing.Store(true)
			_ = r.remote.Close()
			r.l.closeWS(r.ws, r.raw)
			return
		}
		if err != nil {
			return // aborted, or the bridge closed the websocket
		}
	}
}

// dialConn is a tunnelled connection, addressed by its tunnel.
type dialConn struct {
	net.Conn
	addr net.Addr
}

func (c *dialConn) LocalAddr() net.Addr  { return c.addr }
func (c *dialConn) RemoteAddr() net.Addr { return c.addr }
