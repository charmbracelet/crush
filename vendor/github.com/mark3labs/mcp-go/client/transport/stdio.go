package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// Stdio implements the transport layer of the MCP protocol using stdio communication.
// It launches a subprocess and communicates with it via standard input/output streams
// using JSON-RPC messages. The client handles message routing between requests and
// responses, and supports asynchronous notifications.
type Stdio struct {
	command string
	args    []string
	env     []string

	cmd            *exec.Cmd
	stdin          io.WriteCloser
	stdout         *bufio.Reader
	stderr         io.ReadCloser
	responses      map[string]chan *JSONRPCResponse
	mu             sync.RWMutex
	done           chan struct{}
	onNotification func(mcp.JSONRPCNotification)
	notifyMu       sync.RWMutex
}

// NewIO returns a new stdio-based transport using existing input, output, and
// logging streams instead of spawning a subprocess.
// This is useful for testing and simulating client behavior.
func NewIO(input io.Reader, output io.WriteCloser, logging io.ReadCloser) *Stdio {
	return &Stdio{
		stdin:  output,
		stdout: bufio.NewReader(input),
		stderr: logging,

		responses: make(map[string]chan *JSONRPCResponse),
		done:      make(chan struct{}),
	}
}

// NewStdio creates a new stdio transport to communicate with a subprocess.
// It launches the specified command with given arguments and sets up stdin/stdout pipes for communication.
// Returns an error if the subprocess cannot be started or the pipes cannot be created.
func NewStdio(
	command string,
	env []string,
	args ...string,
) *Stdio {

	client := &Stdio{
		command: command,
		args:    args,
		env:     env,

		responses: make(map[string]chan *JSONRPCResponse),
		done:      make(chan struct{}),
	}

	return client
}

func (c *Stdio) Start(ctx context.Context) error {
	if err := c.spawnCommand(ctx); err != nil {
		return err
	}

	ready := make(chan struct{})
	go func() {
		close(ready)
		c.readResponses()
	}()
	<-ready

	return nil
}

// spawnCommand spawns a new process running c.command.
func (c *Stdio) spawnCommand(ctx context.Context) error {
	if c.command == "" {
		return nil
	}

	cmd := exec.CommandContext(ctx, c.command, c.args...)

	mergedEnv := os.Environ()
	mergedEnv = append(mergedEnv, c.env...)

	cmd.Env = mergedEnv

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stderr = stderr
	c.stdout = bufio.NewReader(stdout)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	return nil
}

// Close shuts down the stdio client, closing the stdin pipe and waiting for the subprocess to exit.
// Returns an error if there are issues closing stdin or waiting for the subprocess to terminate.
func (c *Stdio) Close() error {
	select {
	case <-c.done:
		return nil
	default:
	}
	// cancel all in-flight request
	close(c.done)

	if err := c.stdin.Close(); err != nil {
		return fmt.Errorf("failed to close stdin: %w", err)
	}
	if err := c.stderr.Close(); err != nil {
		return fmt.Errorf("failed to close stderr: %w", err)
	}

	if c.cmd != nil {
		return c.cmd.Wait()
	}

	return nil
}

// SetNotificationHandler sets the handler function to be called when a notification is received.
// Only one handler can be set at a time; setting a new one replaces the previous handler.
func (c *Stdio) SetNotificationHandler(
	handler func(notification mcp.JSONRPCNotification),
) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	c.onNotification = handler
}

// readResponses continuously reads and processes responses from the server's stdout.
// It handles both responses to requests and notifications, routing them appropriately.
// Runs until the done channel is closed or an error occurs reading from stdout.
func (c *Stdio) readResponses() {
	for {
		select {
		case <-c.done:
			return
		default:
			line, err := c.stdout.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading response: %v\n", err)
				}
				return
			}

			var baseMessage JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &baseMessage); err != nil {
				continue
			}

			// Handle notification
			if baseMessage.ID.IsNil() {
				var notification mcp.JSONRPCNotification
				if err := json.Unmarshal([]byte(line), &notification); err != nil {
					continue
				}
				c.notifyMu.RLock()
				if c.onNotification != nil {
					c.onNotification(notification)
				}
				c.notifyMu.RUnlock()
				continue
			}

			// Create string key for map lookup
			idKey := baseMessage.ID.String()

			c.mu.RLock()
			ch, exists := c.responses[idKey]
			c.mu.RUnlock()

			if exists {
				ch <- &baseMessage
				c.mu.Lock()
				delete(c.responses, idKey)
				c.mu.Unlock()
			}
		}
	}
}

// SendRequest sends a JSON-RPC request to the server and waits for a response.
// It creates a unique request ID, sends the request over stdin, and waits for
// the corresponding response or context cancellation.
// Returns the raw JSON response message or an error if the request fails.
func (c *Stdio) SendRequest(
	ctx context.Context,
	request JSONRPCRequest,
) (*JSONRPCResponse, error) {
	if c.stdin == nil {
		return nil, fmt.Errorf("stdio client not started")
	}

	// Marshal request
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	requestBytes = append(requestBytes, '\n')

	// Create string key for map lookup
	idKey := request.ID.String()

	// Register response channel
	responseChan := make(chan *JSONRPCResponse, 1)
	c.mu.Lock()
	c.responses[idKey] = responseChan
	c.mu.Unlock()
	deleteResponseChan := func() {
		c.mu.Lock()
		delete(c.responses, idKey)
		c.mu.Unlock()
	}

	// Send request
	if _, err := c.stdin.Write(requestBytes); err != nil {
		deleteResponseChan()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	select {
	case <-ctx.Done():
		deleteResponseChan()
		return nil, ctx.Err()
	case response := <-responseChan:
		return response, nil
	}
}

// SendNotification sends a json RPC Notification to the server.
func (c *Stdio) SendNotification(
	ctx context.Context,
	notification mcp.JSONRPCNotification,
) error {
	if c.stdin == nil {
		return fmt.Errorf("stdio client not started")
	}

	notificationBytes, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}
	notificationBytes = append(notificationBytes, '\n')

	if _, err := c.stdin.Write(notificationBytes); err != nil {
		return fmt.Errorf("failed to write notification: %w", err)
	}

	return nil
}

// Stderr returns a reader for the stderr output of the subprocess.
// This can be used to capture error messages or logs from the subprocess.
func (c *Stdio) Stderr() io.Reader {
	return c.stderr
}
