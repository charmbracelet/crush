package openai

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	callbackAddress = "127.0.0.1:1455"
	callbackPath    = "/auth/callback"
)

type LocalServer struct {
	server    *http.Server
	listener  net.Listener
	codeCh    chan string
	closeOnce sync.Once
}

func StartLocalServer(state string) (*LocalServer, error) {
	listener, err := net.Listen("tcp", callbackAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to bind %s: %w", callbackAddress, err)
	}

	codeCh := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "Missing authorization code", http.StatusBadRequest)
			return
		}
		select {
		case codeCh <- code:
		default:
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><p>Authorization complete. You can close this window.</p></body></html>"))
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	local := &LocalServer{
		server:   server,
		listener: listener,
		codeCh:   codeCh,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	return local, nil
}

func (s *LocalServer) WaitForCode(ctx context.Context) (string, error) {
	select {
	case code := <-s.codeCh:
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *LocalServer) Close() error {
	var err error
	s.closeOnce.Do(func() {
		if s.server == nil {
			err = nil
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if shutdownErr := s.server.Shutdown(shutdownCtx); shutdownErr != nil {
			err = shutdownErr
			return
		}
		if s.listener == nil {
			return
		}
		if closeErr := s.listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			err = closeErr
		}
	})
	return err
}
