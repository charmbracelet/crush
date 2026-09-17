package pinentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/google/uuid"
)

// socketRequest is what the git wrapper subprocess sends to the Crush
// process hosting the socket.
type socketRequest struct {
	Request PromptRequest `json:"request"`
}

// socketResponse is what the host sends back.
type socketResponse struct {
	Secret    string `json:"secret,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
	Error     string `json:"error,omitempty"`
}

// StartIntegration enables the integrated pinentry for this process:
// it writes the git gpg wrapper script, opens the prompt socket, configures
// the process-wide integration, and starts serving wrapper requests.
//
// Returns a cleanup function that stops the socket, removes the wrapper,
// and disables integration. It is safe to cancel ctx to stop serving.
func StartIntegration(ctx context.Context, executable string, cacheTimeout time.Duration) (cleanup func(), err error) {
	// The socket dir must have no spaces (git's gpg program is exec'd
	// as a path); os.TempDir on macOS can contain none, but create a
	// dedicated random dir to be certain.
	wrapperPath := ""
	var wrapperCleanup func()
	if runtime.GOOS != "windows" {
		wrapperPath, wrapperCleanup, err = WriteGPGWrapper(executable)
		if err != nil {
			return nil, err
		}
	}

	dir, err := os.MkdirTemp("", "crush-pinentry-sock-*")
	if err != nil {
		if wrapperCleanup != nil {
			wrapperCleanup()
		}
		return nil, err
	}
	socketPath := filepath.Join(dir, "pinentry.sock")

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "unix", socketPath)
	if err != nil {
		_ = os.RemoveAll(dir)
		if wrapperCleanup != nil {
			wrapperCleanup()
		}
		return nil, err
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = ln.Close()
		_ = os.RemoveAll(dir)
		if wrapperCleanup != nil {
			wrapperCleanup()
		}
		return nil, err
	}

	ConfigureIntegration(IntegrationConfig{
		Prompter: func(ctx context.Context, req PromptRequest) (string, error) {
			return defaultPrompts.Prompt(ctx, req)
		},
		CacheTimeout: cacheTimeout,
		WrapperPath:  wrapperPath,
		SocketPath:   socketPath,
	})

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		serveConnections(ctx, ln)
	}()

	var once bool
	cleanup = func() {
		if once {
			return
		}
		once = true
		_ = ln.Close()
		<-serveDone
		_ = os.RemoveAll(dir)
		if wrapperCleanup != nil {
			wrapperCleanup()
		}
		if wrapperPath != "" {
			DisableIntegration()
		}
	}
	return cleanup, nil
}

// serveConnections accepts and handles wrapper connections until the
// listener closes.
func serveConnections(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleConnection(ctx, conn)
	}
}

// handleConnection services a single git wrapper request: block on the
// prompt service until the user answers (or the context dies), then
// return the secret over the socket.
func handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	var req socketRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	// Bound the wait so a stuck gpg cannot hold the socket forever.
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	secret, err := defaultPrompts.Prompt(reqCtx, req.Request)

	resp := socketResponse{Secret: secret, Cancelled: errors.Is(err, ErrCancelled)}
	if err != nil && !errors.Is(err, ErrCancelled) {
		resp.Error = err.Error()
	}
	_ = json.NewEncoder(conn).Encode(resp)
}

// SocketPrompter returns a Prompter that asks the Crush process hosting
// the given unix socket for the credential. Used by the git gpg wrapper
// subcommand, which runs as the child of git and cannot talk to the
// in-process prompt service directly.
func SocketPrompter(socketPath string) Prompter {
	return func(ctx context.Context, req PromptRequest) (string, error) {
		return AskCredentialOverSocket(ctx, socketPath, req)
	}
}

// AskCredentialOverSocket sends a single prompt request to the host
// socket and returns the secret.
func AskCredentialOverSocket(ctx context.Context, socketPath string, req PromptRequest) (string, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return "", fmt.Errorf("pinentry: connect to host socket: %w", err)
	}
	defer conn.Close()

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	enc := json.NewEncoder(conn)
	if err := enc.Encode(socketRequest{Request: req}); err != nil {
		return "", fmt.Errorf("pinentry: write request: %w", err)
	}

	type result struct {
		resp socketResponse
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		var resp socketResponse
		err := json.NewDecoder(conn).Decode(&resp)
		ch <- result{resp: resp, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return "", fmt.Errorf("pinentry: read response: %w", r.err)
		}
		switch {
		case r.resp.Cancelled:
			return "", ErrCancelled
		case r.resp.Error != "":
			return "", errors.New(r.resp.Error)
		default:
			return r.resp.Secret, nil
		}
	}
}
