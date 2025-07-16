package uv

import (
	"context"

	"github.com/charmbracelet/x/term"
	"golang.org/x/sync/errgroup"
)

// Size represents the size of the terminal window.
type Size struct {
	Width  int
	Height int
}

// Bounds returns the bounds corresponding to the size.
func (s Size) Bounds() Rectangle {
	return Rect(0, 0, s.Width, s.Height)
}

// InputReceiver is an interface for receiving input events from an input source.
type InputReceiver interface {
	// ReceiveEvents read input events and channel them to the given event
	// channel. The listener stops when either the context is done or an error
	// occurs. Caller is responsible for closing the channels.
	ReceiveEvents(ctx context.Context, events chan<- Event) error
}

// InputManager manages input events from multiple input sources. It listens
// for input events from the registered input sources and combines them into a
// single event channel. It also handles errors from the input sources and
// sends them to the error channel.
type InputManager struct {
	receivers []InputReceiver
}

// NewInputManager creates a new InputManager with the input receivers.
func NewInputManager(receivers ...InputReceiver) *InputManager {
	im := &InputManager{
		receivers: receivers,
	}
	return im
}

// RegisterReceiver registers a new input receiver with the input manager.
func (im *InputManager) RegisterReceiver(r InputReceiver) {
	im.receivers = append(im.receivers, r)
}

// ReceiveEvents starts receiving events from the registered input
// receivers. It sends the events to the given event and error channels.
func (im *InputManager) ReceiveEvents(ctx context.Context, events chan<- Event) error {
	errg, ctx := errgroup.WithContext(ctx)
	for _, r := range im.receivers {
		errg.Go(func() error {
			return r.ReceiveEvents(ctx, events)
		})
	}

	// Wait for all receivers to finish
	return errg.Wait()
}

// InitialSizeReceiver query the initial size of the terminal window and sends
// it to the given event channel.
type InitialSizeReceiver struct {
	File term.File
}

// ReceiveEvents queries the initial size of the terminal window and sends it
// to the given event channel. It stops when event is sent, the context is done
// or an error occurs.
func (r *InitialSizeReceiver) ReceiveEvents(ctx context.Context, events chan<- Event) error {
	if r.File == nil {
		return nil // No file set, nothing to do.
	}

	w, h, err := term.GetSize(r.File.Fd())
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case events <- WindowSizeEvent{Width: w, Height: h}:
		return nil
	}
}
