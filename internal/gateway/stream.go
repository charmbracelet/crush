package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/charmbracelet/crush/internal/jq"
)

// eventConverter holds the last JSON event until a successor or EOF resolves its finality.
type eventConverter struct {
	upstream io.ReadCloser
	reader   *bufio.Reader
	ctx      context.Context
	program  *jq.Program
	base     map[string]any
	last     *streamEvent
	trailing bytes.Buffer // Passthrough frames after last must keep their order.
	output   bytes.Buffer
	err      error
}

func newEventConverter(ctx context.Context, program *jq.Program, base map[string]any, upstream io.ReadCloser) *eventConverter {
	return &eventConverter{upstream: upstream, reader: bufio.NewReader(upstream), ctx: ctx, program: program, base: base}
}

func (c *eventConverter) Read(p []byte) (int, error) {
	for len(p) != 0 && c.output.Len() == 0 && c.err == nil {
		c.err = c.next()
	}
	if c.output.Len() > 0 {
		return c.output.Read(p)
	}
	return 0, c.err
}

func (c *eventConverter) Close() error {
	return c.upstream.Close()
}

func (c *eventConverter) next() error {
	event, readErr := readEvent(c.reader)
	if event.rewrite {
		if err := c.flush(false); err != nil {
			return err
		}
		c.last = &event
	} else if c.last != nil {
		if c.trailing.Len()+len(event.raw) > maxBody {
			return fmt.Errorf("stream passthrough held behind an event exceeds %d bytes", maxBody)
		}
		c.trailing.WriteString(event.raw)
	} else {
		c.output.WriteString(event.raw)
	}
	if readErr == nil {
		return nil
	}
	if err := c.flush(readErr == io.EOF); err != nil {
		if readErr == io.EOF {
			return err
		}
		return errors.Join(err, readErr)
	}
	return readErr
}

func (c *eventConverter) flush(final bool) error {
	if c.last == nil {
		return nil
	}
	event := c.last
	c.last = nil
	input := maps.Clone(c.base)
	input["event"] = true
	input["data"] = event.data
	input["final"] = final
	output, err := c.program.Run(c.ctx, input)
	if err != nil {
		return fmt.Errorf("rewrite stream event: %w", err)
	}
	events, ok := output.([]any)
	if !ok {
		return fmt.Errorf("rewrite stream event: program output must be an array of events, got %T", output)
	}
	if event.framing != "" {
		c.output.WriteString(event.framing)
		if !strings.HasSuffix(event.framing, "\n") || len(events) == 0 {
			c.output.WriteByte('\n')
		}
	}
	for _, replacement := range events {
		encoded, err := json.Marshal(replacement)
		if err != nil {
			return fmt.Errorf("encode rewritten event: %w", err)
		}
		if event.name != "" {
			fmt.Fprintf(&c.output, "event: %s\n", event.name)
		}
		fmt.Fprintf(&c.output, "data: %s\n\n", encoded)
	}
	c.trailing.WriteTo(&c.output)
	return nil
}

type streamEvent struct {
	name    string
	data    any
	rewrite bool
	raw     string
	framing string
}

// readEvent parses one SSE block and keeps the original bytes for passthrough.
func readEvent(r *bufio.Reader) (streamEvent, error) {
	var name string
	var raw, data, framing strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return streamEvent{}, err
		}
		raw.WriteString(line)
		trimmed := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if trimmed != "" {
			field, value, _ := strings.Cut(trimmed, ":")
			value = strings.TrimPrefix(value, " ")
			switch field {
			case "event":
				name = value
			case "data":
				data.WriteString(value)
				data.WriteByte('\n')
			default:
				framing.WriteString(line)
			}
		}
		if trimmed == "" || err == io.EOF {
			if data.Len() > 0 {
				if payload, ok := parseJSON([]byte(data.String())); ok {
					return streamEvent{name: name, data: payload, rewrite: true, framing: framing.String()}, err
				}
			}
			return streamEvent{raw: raw.String()}, err
		}
	}
}
