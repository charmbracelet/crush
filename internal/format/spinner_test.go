package format

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/ui/anim"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

type spinnerBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *spinnerBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *spinnerBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *spinnerBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func TestSpinnerRendersLabelsAndStops(t *testing.T) {
	var output spinnerBuffer
	spinner := newSpinnerWithOutput(t.Context(), anim.Settings{Label: "Generating"}, &output)
	spinner.Start()
	require.Eventually(t, func() bool {
		return strings.Contains(output.String(), "Generating")
	}, 3*time.Second, 20*time.Millisecond)

	spinner.SetLabel("Thinking...")
	require.Eventually(t, func() bool {
		return strings.Contains(output.String(), "Thinking...")
	}, 3*time.Second, 20*time.Millisecond)
	spinner.Stop()
	spinner.Stop()
	require.Contains(t, output.String(), "\r"+ansi.EraseEntireLine)
}
