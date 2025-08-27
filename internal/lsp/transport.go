package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
)

// WriteMessage writes an LSP message to the given writer
func WriteMessage(w io.Writer, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	cfg := config.Get()

	if cfg.Options.DebugLSP {
		slog.Debug("Sending message to server", "method", msg.Method, "id", msg.ID)
	}

	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data))
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// ReadMessage reads a single LSP message from the given reader
func ReadMessage(r *bufio.Reader) (*Message, error) {
	cfg := config.Get()
	// Read headers
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
		line = strings.TrimSpace(line)

		if cfg.Options.DebugLSP {
			slog.Debug("Received header", "line", line)
		}

		if line == "" {
			break // End of headers
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			_, err := fmt.Sscanf(line, "Content-Length: %d", &contentLength)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}
		}
	}

	if cfg.Options.DebugLSP {
		slog.Debug("Content-Length", "length", contentLength)
	}

	// Read content
	content := make([]byte, contentLength)
	_, err := io.ReadFull(r, content)
	if err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	if cfg.Options.DebugLSP {
		slog.Debug("Received content", "content", string(content))
	}

	// Parse message
	var msg Message
	if err := json.Unmarshal(content, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &msg, nil
}

// handleMessages reads and dispatches messages in a loop
func (c *Client) handleMessages() {
	cfg := config.Get()
	for {
		msg, err := ReadMessage(c.stdout)
		if err != nil {
			if cfg.Options.DebugLSP {
				slog.Error("Error reading message", "error", err)
			}
			return
		}

		// Handle server->client request (has both Method and ID)
		if msg.Method != "" && msg.ID != 0 {
			if cfg.Options.DebugLSP {
				slog.Debug("Received request from server", "method", msg.Method, "id", msg.ID)
			}

			response := &Message{
				JSONRPC: "2.0",
				ID:      msg.ID,
			}

			// Look up handler for this method
			c.serverHandlersMu.RLock()
			handler, ok := c.serverRequestHandlers[msg.Method]
			c.serverHandlersMu.RUnlock()

			if ok {
				result, err := handler(msg.Params)
				if err != nil {
					response.Error = &ResponseError{
						Code:    -32603,
						Message: err.Error(),
					}
				} else {
					rawJSON, err := json.Marshal(result)
					if err != nil {
						response.Error = &ResponseError{
							Code:    -32603,
							Message: fmt.Sprintf("failed to marshal response: %v", err),
						}
					} else {
						response.Result = rawJSON
					}
				}
			} else {
				response.Error = &ResponseError{
					Code:    -32601,
					Message: fmt.Sprintf("method not found: %s", msg.Method),
				}
			}

			// Send response back to server
			if err := WriteMessage(c.stdin, response); err != nil {
				slog.Error("Error sending response to server", "error", err)
			}

			continue
		}

		// Handle notification (has Method but no ID)
		if msg.Method != "" && msg.ID == 0 {
			c.notificationMu.RLock()
			handler, ok := c.notificationHandlers[msg.Method]
			c.notificationMu.RUnlock()

			if ok {
				if cfg.Options.DebugLSP {
					slog.Debug("Handling notification", "method", msg.Method)
				}
				go handler(msg.Params)
			} else if cfg.Options.DebugLSP {
				slog.Debug("No handler for notification", "method", msg.Method)
			}
			continue
		}

		// Handle response to our request (has ID but no Method)
		if msg.ID != 0 && msg.Method == "" {
			c.handlersMu.RLock()
			ch, ok := c.handlers[msg.ID]
			c.handlersMu.RUnlock()

			if ok {
				if cfg.Options.DebugLSP {
					slog.Debug("Received response for request", "id", msg.ID)
				}
				ch <- msg
				close(ch)
			} else if cfg.Options.DebugLSP {
				slog.Debug("No handler for response", "id", msg.ID)
			}
		}
	}
}

// Call makes a request and waits for the response
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	if !c.IsMethodSupported(method) {
		return fmt.Errorf("method not supported by server: %s", method)
	}
	id := c.nextID.Add(1)

	cfg := config.Get()
	if cfg.Options.DebugLSP {
		slog.Debug("Making call", "method", method, "id", id)
	}

	msg, err := NewRequest(id, method, params)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Create response channel
	ch := make(chan *Message, 1)
	c.handlersMu.Lock()
	c.handlers[id] = ch
	c.handlersMu.Unlock()

	defer func() {
		c.handlersMu.Lock()
		delete(c.handlers, id)
		c.handlersMu.Unlock()
	}()

	// Send request
	if err := WriteMessage(c.stdin, msg); err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	if cfg.Options.DebugLSP {
		slog.Debug("Request sent", "method", method, "id", id)
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-ch:
		if cfg.Options.DebugLSP {
			slog.Debug("Received response", "id", id)
		}

		if resp.Error != nil {
			return fmt.Errorf("request failed: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}

		if result != nil {
			// If result is a json.RawMessage, just copy the raw bytes
			if rawMsg, ok := result.(*json.RawMessage); ok {
				*rawMsg = resp.Result
				return nil
			}
			// Otherwise unmarshal into the provided type
			if err := json.Unmarshal(resp.Result, result); err != nil {
				return fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}

		return nil
	}
}

// Notify sends a notification (a request without an ID that doesn't expect a response)
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	cfg := config.Get()
	if !c.IsMethodSupported(method) {
		if cfg.Options.DebugLSP {
			slog.Debug("Skipping notification: method not supported by server", "method", method)
		}
		return nil
	}
	if cfg.Options.DebugLSP {
		slog.Debug("Sending notification", "method", method)
	}

	msg, err := NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	if err := WriteMessage(c.stdin, msg); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

type (
	NotificationHandler  func(params json.RawMessage)
	ServerRequestHandler func(params json.RawMessage) (any, error)
)
