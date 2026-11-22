package format

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/x/ansi"
)

// Spinner renders a non-interactive activity animation to stderr.
type Spinner struct {
	ctx      context.Context
	anim     *anim.Anim
	out      io.Writer
	done     chan struct{}
	stop     chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
}

func NewSpinner(ctx context.Context, _ context.CancelFunc, animSettings anim.Settings) *Spinner {
	return newSpinnerWithOutput(ctx, animSettings, os.Stderr)
}

func newSpinnerWithOutput(ctx context.Context, animSettings anim.Settings, out io.Writer) *Spinner {
	return &Spinner{
		ctx:  ctx,
		anim: anim.New(animSettings),
		out:  out,
		done: make(chan struct{}),
		stop: make(chan struct{}),
	}
}

// Start begins the spinner animation.
func (s *Spinner) Start() {
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(anim.FrameInterval())
		defer ticker.Stop()
		defer s.clear()
		s.render()
		for {
			select {
			case <-ticker.C:
				s.mu.Lock()
				s.anim.Advance()
				s.mu.Unlock()
				s.render()
			case <-s.stop:
				return
			case <-s.ctx.Done():
				return
			}
		}
	}()
}

// Stop ends the spinner animation.
func (s *Spinner) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
	<-s.done
}

// SetLabel changes the text displayed beside the spinner.
func (s *Spinner) SetLabel(label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.anim.SetLabel(label)
}

func (s *Spinner) render() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprint(s.out, "\r"+ansi.EraseEntireLine+s.anim.Render())
}

func (s *Spinner) clear() {
	_, _ = fmt.Fprint(s.out, "\r"+ansi.EraseEntireLine)
}
