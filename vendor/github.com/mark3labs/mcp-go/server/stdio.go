package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/mark3labs/mcp-go/mcp"
)

// StdioContextFunc is a function that takes an existing context and returns
// a potentially modified context.
// This can be used to inject context values from environment variables,
// for example.
type StdioContextFunc func(ctx context.Context) context.Context

// StdioServer wraps a MCPServer and handles stdio communication.
// It provides a simple way to create command-line MCP servers that
// communicate via standard input/output streams using JSON-RPC messages.
type StdioServer struct {
	server      *MCPServer
	errLogger   *log.Logger
	contextFunc StdioContextFunc
}

// StdioOption defines a function type for configuring StdioServer
type StdioOption func(*StdioServer)

// WithErrorLogger sets the error logger for the server
func WithErrorLogger(logger *log.Logger) StdioOption {
	return func(s *StdioServer) {
		s.errLogger = logger
	}
}

// WithStdioContextFunc sets a function that will be called to customise the context
// to the server. Note that the stdio server uses the same context for all requests,
// so this function will only be called once per server instance.
func WithStdioContextFunc(fn StdioContextFunc) StdioOption {
	return func(s *StdioServer) {
		s.contextFunc = fn
	}
}

// stdioSession is a static client session, since stdio has only one client.
type stdioSession struct {
	notifications chan mcp.JSONRPCNotification
	initialized   atomic.Bool
	loggingLevel  atomic.Value
	clientInfo    atomic.Value // stores session-specific client info
}

func (s *stdioSession) SessionID() string {
	return "stdio"
}

func (s *stdioSession) NotificationChannel() chan<- mcp.JSONRPCNotification {
	return s.notifications
}

func (s *stdioSession) Initialize() {
	// set default logging level
	s.loggingLevel.Store(mcp.LoggingLevelError)
	s.initialized.Store(true)
}

func (s *stdioSession) Initialized() bool {
	return s.initialized.Load()
}

func (s *stdioSession) GetClientInfo() mcp.Implementation {
	if value := s.clientInfo.Load(); value != nil {
		if clientInfo, ok := value.(mcp.Implementation); ok {
			return clientInfo
		}
	}
	return mcp.Implementation{}
}

func (s *stdioSession) SetClientInfo(clientInfo mcp.Implementation) {
	s.clientInfo.Store(clientInfo)
}

func (s *stdioSession) SetLogLevel(level mcp.LoggingLevel) {
	s.loggingLevel.Store(level)
}

func (s *stdioSession) GetLogLevel() mcp.LoggingLevel {
	level := s.loggingLevel.Load()
	if level == nil {
		return mcp.LoggingLevelError
	}
	return level.(mcp.LoggingLevel)
}

var (
	_ ClientSession         = (*stdioSession)(nil)
	_ SessionWithLogging    = (*stdioSession)(nil)
	_ SessionWithClientInfo = (*stdioSession)(nil)
)

var stdioSessionInstance = stdioSession{
	notifications: make(chan mcp.JSONRPCNotification, 100),
}

// NewStdioServer creates a new stdio server wrapper around an MCPServer.
// It initializes the server with a default error logger that discards all output.
func NewStdioServer(server *MCPServer) *StdioServer {
	return &StdioServer{
		server: server,
		errLogger: log.New(
			os.Stderr,
			"",
			log.LstdFlags,
		), // Default to discarding logs
	}
}

// SetErrorLogger configures where error messages from the StdioServer are logged.
// The provided logger will receive all error messages generated during server operation.
func (s *StdioServer) SetErrorLogger(logger *log.Logger) {
	s.errLogger = logger
}

// SetContextFunc sets a function that will be called to customise the context
// to the server. Note that the stdio server uses the same context for all requests,
// so this function will only be called once per server instance.
func (s *StdioServer) SetContextFunc(fn StdioContextFunc) {
	s.contextFunc = fn
}

// handleNotifications continuously processes notifications from the session's notification channel
// and writes them to the provided output. It runs until the context is cancelled.
// Any errors encountered while writing notifications are logged but do not stop the handler.
func (s *StdioServer) handleNotifications(ctx context.Context, stdout io.Writer) {
	for {
		select {
		case notification := <-stdioSessionInstance.notifications:
			if err := s.writeResponse(notification, stdout); err != nil {
				s.errLogger.Printf("Error writing notification: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// processInputStream continuously reads and processes messages from the input stream.
// It handles EOF gracefully as a normal termination condition.
// The function returns when either:
// - The context is cancelled (returns context.Err())
// - EOF is encountered (returns nil)
// - An error occurs while reading or processing messages (returns the error)
func (s *StdioServer) processInputStream(ctx context.Context, reader *bufio.Reader, stdout io.Writer) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		line, err := s.readNextLine(ctx, reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			s.errLogger.Printf("Error reading input: %v", err)
			return err
		}

		if err := s.processMessage(ctx, line, stdout); err != nil {
			if err == io.EOF {
				return nil
			}
			s.errLogger.Printf("Error handling message: %v", err)
			return err
		}
	}
}

// readNextLine reads a single line from the input reader in a context-aware manner.
// It uses channels to make the read operation cancellable via context.
// Returns the read line and any error encountered. If the context is cancelled,
// returns an empty string and the context's error. EOF is returned when the input
// stream is closed.
func (s *StdioServer) readNextLine(ctx context.Context, reader *bufio.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}

	resultCh := make(chan result, 1)

	go func() {
		line, err := reader.ReadString('\n')
		resultCh <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", nil
	case res := <-resultCh:
		return res.line, res.err
	}
}

// Listen starts listening for JSON-RPC messages on the provided input and writes responses to the provided output.
// It runs until the context is cancelled or an error occurs.
// Returns an error if there are issues with reading input or writing output.
func (s *StdioServer) Listen(
	ctx context.Context,
	stdin io.Reader,
	stdout io.Writer,
) error {
	// Set a static client context since stdio only has one client
	if err := s.server.RegisterSession(ctx, &stdioSessionInstance); err != nil {
		return fmt.Errorf("register session: %w", err)
	}
	defer s.server.UnregisterSession(ctx, stdioSessionInstance.SessionID())
	ctx = s.server.WithContext(ctx, &stdioSessionInstance)

	// Add in any custom context.
	if s.contextFunc != nil {
		ctx = s.contextFunc(ctx)
	}

	reader := bufio.NewReader(stdin)

	// Start notification handler
	go s.handleNotifications(ctx, stdout)
	return s.processInputStream(ctx, reader, stdout)
}

// processMessage handles a single JSON-RPC message and writes the response.
// It parses the message, processes it through the wrapped MCPServer, and writes any response.
// Returns an error if there are issues with message processing or response writing.
func (s *StdioServer) processMessage(
	ctx context.Context,
	line string,
	writer io.Writer,
) error {
	// If line is empty, likely due to ctx cancellation
	if len(line) == 0 {
		return nil
	}

	// Parse the message as raw JSON
	var rawMessage json.RawMessage
	if err := json.Unmarshal([]byte(line), &rawMessage); err != nil {
		response := createErrorResponse(nil, mcp.PARSE_ERROR, "Parse error")
		return s.writeResponse(response, writer)
	}

	// Handle the message using the wrapped server
	response := s.server.HandleMessage(ctx, rawMessage)

	// Only write response if there is one (not for notifications)
	if response != nil {
		if err := s.writeResponse(response, writer); err != nil {
			return fmt.Errorf("failed to write response: %w", err)
		}
	}

	return nil
}

// writeResponse marshals and writes a JSON-RPC response message followed by a newline.
// Returns an error if marshaling or writing fails.
func (s *StdioServer) writeResponse(
	response mcp.JSONRPCMessage,
	writer io.Writer,
) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	// Write response followed by newline
	if _, err := fmt.Fprintf(writer, "%s\n", responseBytes); err != nil {
		return err
	}

	return nil
}

// ServeStdio is a convenience function that creates and starts a StdioServer with os.Stdin and os.Stdout.
// It sets up signal handling for graceful shutdown on SIGTERM and SIGINT.
// Returns an error if the server encounters any issues during operation.
func ServeStdio(server *MCPServer, opts ...StdioOption) error {
	s := NewStdioServer(server)

	for _, opt := range opts {
		opt(s)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		cancel()
	}()

	return s.Listen(ctx, os.Stdin, os.Stdout)
}
