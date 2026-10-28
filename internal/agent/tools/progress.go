package tools

import (
	"context"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/shell"
)

const (
	// progressPollInterval is how often a running command's output is
	// checked and, if it changed, published. It also caps the update rate,
	// so a chatty command cannot flood the UI.
	progressPollInterval = 200 * time.Millisecond

	// progressMaxBytes bounds the trailing output carried by one event.
	progressMaxBytes = 64 * 1024
)

// ToolProgress carries live output for a running tool call. It is display
// only: it is never persisted or sent to the model, and the tool result
// replaces it.
type ToolProgress struct {
	SessionID  string `json:"session_id"`
	ToolCallID string `json:"tool_call_id"`
	ShellID    string `json:"shell_id,omitempty"`
	Output     string `json:"output"`
}

var progressBroker = pubsub.NewBroker[ToolProgress]()

// SubscribeProgress returns a channel of live tool output events.
func SubscribeProgress(ctx context.Context) <-chan pubsub.Event[ToolProgress] {
	return progressBroker.Subscribe(ctx)
}

// progressReporter publishes size-limited live output for one tool call.
type progressReporter struct {
	sessionID  string
	toolCallID string
	publish    func(ToolProgress)

	lastOutput string
}

func newProgressReporter(sessionID, toolCallID string) *progressReporter {
	return &progressReporter{
		sessionID:  sessionID,
		toolCallID: toolCallID,
		publish: func(p ToolProgress) {
			progressBroker.Publish(pubsub.UpdatedEvent, p)
		},
	}
}

// report publishes the shell's current output if it changed since the last
// event.
func (r *progressReporter) report(bgShell *shell.BackgroundShell) {
	stdout, stderr, _, _ := bgShell.GetOutput()
	r.publishOutput(bgShell.ID, stdout, stderr)
}

func (r *progressReporter) publishOutput(shellID, stdout, stderr string) {
	if r.toolCallID == "" {
		return
	}
	output := combineProgressOutput(stdout, stderr)
	if output == r.lastOutput {
		return
	}
	r.lastOutput = output
	r.publish(ToolProgress{
		SessionID:  r.sessionID,
		ToolCallID: r.toolCallID,
		ShellID:    shellID,
		Output:     tailProgressOutput(output),
	})
}

// combineProgressOutput joins stdout and stderr the way the final tool
// result shows them.
func combineProgressOutput(stdout, stderr string) string {
	stdout = strings.TrimRight(stdout, "\n")
	stderr = strings.TrimRight(stderr, "\n")
	switch {
	case stdout == "":
		return stderr
	case stderr == "":
		return stdout
	default:
		return stdout + "\n" + stderr
	}
}

// tailProgressOutput keeps the last progressMaxBytes of output, starting on
// a line boundary when one is available.
func tailProgressOutput(output string) string {
	if len(output) <= progressMaxBytes {
		return output
	}
	tail := output[len(output)-progressMaxBytes:]
	if idx := strings.IndexByte(tail, '\n'); idx >= 0 && idx < len(tail)-1 {
		tail = tail[idx+1:]
	}
	return tail
}
