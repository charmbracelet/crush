package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	stdpath "path"
	"path/filepath"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/server"
	"github.com/google/uuid"
)

// DummyHost is used to satisfy the http.Client's requirement for a URL.
const DummyHost = "api.crush.localhost"

// Client represents an RPC client connected to a Crush server.
type Client struct {
	h        *http.Client
	path     string
	network  string
	addr     string
	clientID string
}

// DefaultClient creates a new [Client] connected to the default server address.
func DefaultClient(path string) (*Client, error) {
	host, err := server.ParseHostURL(server.DefaultHost())
	if err != nil {
		return nil, err
	}
	return NewClient(path, host.Scheme, host.Host)
}

// NewClient creates a new [Client] connected to the server at the given
// network and address.
func NewClient(path, network, address string) (*Client, error) {
	c := new(Client)
	c.path = filepath.Clean(path)
	c.network = network
	c.addr = address
	c.clientID = uuid.New().String()
	p := &http.Protocols{}
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Protocols = p
	tr.DialContext = c.dialer
	if c.network == "npipe" || c.network == "unix" {
		tr.DisableCompression = true
	}
	c.h = &http.Client{
		Transport: tr,
		Timeout:   0,
	}
	return c, nil
}

// Path returns the client's workspace filesystem path.
func (c *Client) Path() string {
	return c.path
}

// ClientID returns the per-process client ID minted in [NewClient].
// The server uses it as a presence/coordination handle.
func (c *Client) ClientID() string {
	return c.clientID
}

// GetGlobalConfig retrieves the server's configuration.
func (c *Client) GetGlobalConfig(ctx context.Context) (*config.Config, error) {
	var cfg config.Config
	rsp, err := c.get(ctx, "/config", nil, nil)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if err := json.NewDecoder(rsp.Body).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Health checks the server's health status.
func (c *Client) Health(ctx context.Context) error {
	rsp, err := c.get(ctx, "/health", nil, nil)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("server health check failed: %s", rsp.Status)
	}
	return nil
}

// VersionInfo retrieves the server's version information.
func (c *Client) VersionInfo(ctx context.Context) (*proto.VersionInfo, error) {
	var vi proto.VersionInfo
	rsp, err := c.get(ctx, "version", nil, nil)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()
	if err := json.NewDecoder(rsp.Body).Decode(&vi); err != nil {
		return nil, err
	}
	return &vi, nil
}

// ShutdownServerIfIdle asks the server to shut down, which it grants only
// if it is hosting nothing. There is deliberately no unconditional
// variant: a client only ever wants a server replaced, never other
// sessions killed.
//
// A server that declines because it is in use returns an error wrapping
// [ErrServerBusy]. A server too old to know the command returns
// [ErrUnsupported]; it must be left running, since the shutdown request
// it does understand is unconditional and would take its sessions down.
func (c *Client) ShutdownServerIfIdle(ctx context.Context) error {
	rsp, err := c.post(ctx, "/control", nil, jsonBody(proto.ServerControl{
		Command: proto.ServerControlShutdownIfIdle,
	}), nil)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode == http.StatusOK {
		return nil
	}
	failure := fmt.Errorf("server shutdown failed: %s", rsp.Status)
	switch rsp.StatusCode {
	case http.StatusConflict:
		return fmt.Errorf("%w: %w", ErrServerBusy, failure)
	case http.StatusBadRequest:
		// The only way a well-formed control request is rejected as bad
		// is an unknown command, i.e. a server predating this one.
		return fmt.Errorf("%w: %w", ErrUnsupported, failure)
	}
	return failure
}

// ShutdownServer sends the original, unconditional "shutdown" command.
// It exists for backward compatibility with servers that predate
// [ServerControlShutdownIfIdle]: those servers reject the idle-checked
// variant with [ErrUnsupported], so a client that has already verified
// the server is idle (e.g. via [Client.ListWorkspaces]) can fall back to
// this command to replace an old server.
//
// New servers apply the same idleness check to this command as they do
// to [ServerControlShutdownIfIdle], so it is never more dangerous.
func (c *Client) ShutdownServer(ctx context.Context) error {
	rsp, err := c.post(ctx, "/control", nil, jsonBody(proto.ServerControl{
		Command: proto.ServerControlShutdown,
	}), nil)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if rsp.StatusCode == http.StatusOK {
		return nil
	}
	failure := fmt.Errorf("server shutdown failed: %s", rsp.Status)
	switch rsp.StatusCode {
	case http.StatusConflict:
		return fmt.Errorf("%w: %w", ErrServerBusy, failure)
	}
	return failure
}

// RetireClient tells the server this client has exited, releasing every
// claim it holds on every workspace. It is the client's authoritative
// goodbye: after it returns, the server refuses further workspace
// creates from this client ID, so a create whose response was lost cannot
// leave a workspace nobody can name.
//
// Servers predating the endpoint answer 404, reported as
// [ErrUnsupported] so callers can fall back to releasing by workspace ID.
func (c *Client) RetireClient(ctx context.Context) error {
	rsp, err := c.delete(ctx, "/clients/"+c.clientID, nil, nil)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	if err := checkStatus(rsp); err != nil {
		if errors.Is(err, ErrNotFound) {
			return fmt.Errorf("%w: %w", ErrUnsupported, err)
		}
		return fmt.Errorf("failed to retire client: %w", err)
	}
	return nil
}

// Dial opens a connection to the server using the same scheme-aware
// logic the client uses for its HTTP transport. Exposed so callers can
// reuse the dialer when they need to construct sibling HTTP transports
// (e.g. a readiness probe in the CLI).
func (c *Client) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	return c.dialer(ctx, network, address)
}

func (c *Client) dialer(ctx context.Context, network, address string) (net.Conn, error) {
	d := net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	switch c.network {
	case "npipe":
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return dialPipeContext(ctx, c.addr)
	case "unix":
		return d.DialContext(ctx, "unix", c.addr)
	default:
		return d.DialContext(ctx, network, address)
	}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, headers http.Header) (*http.Response, error) {
	return c.sendReq(ctx, http.MethodGet, path, query, nil, headers)
}

func (c *Client) post(ctx context.Context, path string, query url.Values, body io.Reader, headers http.Header) (*http.Response, error) {
	return c.sendReq(ctx, http.MethodPost, path, query, body, headers)
}

func (c *Client) delete(ctx context.Context, path string, query url.Values, headers http.Header) (*http.Response, error) {
	return c.sendReq(ctx, http.MethodDelete, path, query, nil, headers)
}

func (c *Client) put(ctx context.Context, path string, query url.Values, body io.Reader, headers http.Header) (*http.Response, error) {
	return c.sendReq(ctx, http.MethodPut, path, query, body, headers)
}

func (c *Client) sendReq(ctx context.Context, method, path string, query url.Values, body io.Reader, headers http.Header) (*http.Response, error) {
	url := (&url.URL{
		Path:     stdpath.Join("/v1", path),
		RawQuery: query.Encode(),
	}).String()
	req, err := c.buildReq(ctx, method, url, body, headers)
	if err != nil {
		return nil, err
	}

	rsp, err := c.h.Do(req)
	if err != nil {
		return nil, err
	}

	return rsp, nil
}

func (c *Client) buildReq(ctx context.Context, method, url string, body io.Reader, headers http.Header) (*http.Request, error) {
	r, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		r.Header[http.CanonicalHeaderKey(k)] = v
	}

	r.URL.Scheme = "http"
	r.URL.Host = c.addr
	if c.network == "npipe" || c.network == "unix" {
		r.Host = DummyHost
	}

	if body != nil && r.Header.Get("Content-Type") == "" {
		r.Header.Set("Content-Type", "text/plain")
	}

	return r, nil
}
