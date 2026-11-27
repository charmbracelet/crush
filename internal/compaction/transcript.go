package compaction

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/message"
)

const (
	// DirName is the subdirectory of Crush's data directory that holds
	// displaced transcript regions.
	DirName = "compaction"

	// MaxTranscriptBytes bounds one displaced region on disk. Regions are
	// written once and read rarely, so the cap exists to keep a runaway
	// session from filling a home directory rather than to save space.
	MaxTranscriptBytes = 4 << 20
	// KeptTranscripts is how many displaced regions a session keeps before
	// the oldest are dropped.
	KeptTranscripts = 5

	transcriptDirPerm  = 0o700
	transcriptFilePerm = 0o600
)

// TranscriptDir returns where displaced transcript regions live below a data
// directory, or "" when there is no data directory. Callers hand the result
// straight to WriteTranscript, which treats "" as "keep nothing on disk".
func TranscriptDir(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, DirName)
}

// Serialize renders msgs as plain text for storage outside the context
// window. The rendering is meant to be grepped by the assistant that lost the
// originals, so structure (who said what, which tool ran) survives even where
// binary payloads cannot.
//
// When the region is larger than MaxTranscriptBytes the oldest messages are
// dropped, since the newest part of a displaced region sits closest to the
// work still in hand.
func Serialize(msgs []message.Message) string {
	var (
		chunks  = make([]string, 0, len(msgs))
		total   int
		dropped int
	)
	for i := len(msgs) - 1; i >= 0; i-- {
		chunk := renderMessage(i+1, msgs[i])
		if total+len(chunk) > MaxTranscriptBytes {
			dropped = i + 1
			break
		}
		total += len(chunk)
		chunks = append(chunks, chunk)
	}

	slices.Reverse(chunks)
	if dropped > 0 {
		note := fmt.Sprintf(
			"[... %d earlier messages omitted: this transcript is capped at %d bytes ...]\n\n",
			dropped, MaxTranscriptBytes,
		)
		return note + strings.Join(chunks, "")
	}
	return strings.Join(chunks, "")
}

// WriteTranscript stores the region a compaction is about to replace and
// returns the path the checkpoint can point at. It returns an empty path and
// no error when there is nothing to store, so callers can treat a failure as
// "no pointer this time" rather than as a failed compaction.
func WriteTranscript(dir, sessionID, summaryID string, msgs []message.Message) (string, error) {
	if dir == "" || len(msgs) == 0 {
		return "", nil
	}

	sessionDir := filepath.Join(dir, sanitize(sessionID))
	if err := os.MkdirAll(sessionDir, transcriptDirPerm); err != nil {
		return "", fmt.Errorf("failed to create compaction transcript directory: %w", err)
	}

	path := filepath.Join(sessionDir, fmt.Sprintf("%d-%s.txt", time.Now().Unix(), sanitize(shortID(summaryID))))
	if err := os.WriteFile(path, []byte(Serialize(msgs)), transcriptFilePerm); err != nil {
		return "", fmt.Errorf("failed to write compaction transcript: %w", err)
	}

	prune(sessionDir)
	return path, nil
}

// prune keeps the newest KeptTranscripts region files in dir. Names start
// with a unix timestamp, so lexical order is chronological order.
func prune(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) <= KeptTranscripts {
		return
	}

	slices.Sort(names)
	for _, name := range names[:len(names)-KeptTranscripts] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			continue
		}
	}
}

func renderMessage(n int, msg message.Message) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "--- [%d] %s", n, msg.Role)
	if msg.Model != "" {
		fmt.Fprintf(&sb, " model=%s", msg.Model)
	}
	sb.WriteString(" ---\n")

	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			if p.Text == "" {
				continue
			}
			sb.WriteString(p.Text)
			sb.WriteByte('\n')
		case message.ReasoningContent:
			if p.Thinking == "" {
				continue
			}
			fmt.Fprintf(&sb, "<thinking>\n%s\n</thinking>\n", p.Thinking)
		case message.ToolCall:
			fmt.Fprintf(&sb, "<tool_call name=%q id=%q>\n%s\n</tool_call>\n", p.Name, p.ID, p.Input)
		case message.ToolResult:
			if p.IsError {
				fmt.Fprintf(&sb, "<tool_result name=%q id=%q error>\n%s\n</tool_result>\n", p.Name, p.ToolCallID, p.Content)
				continue
			}
			fmt.Fprintf(&sb, "<tool_result name=%q id=%q>\n%s\n</tool_result>\n", p.Name, p.ToolCallID, p.Content)
		case message.ShellCommand:
			fmt.Fprintf(&sb, "<shell command=%q exit_code=%d>\n%s\n</shell>\n", p.Command, p.ExitCode, p.Output)
		case message.BinaryContent:
			fmt.Fprintf(&sb, "<attachment path=%q mime=%q bytes=%d />\n", p.Path, p.MIMEType, len(p.Data))
		case message.ImageURLContent:
			fmt.Fprintf(&sb, "<image url=%q />\n", p.URL)
		}
	}

	sb.WriteByte('\n')
	return sb.String()
}

// sanitize keeps a caller-supplied identifier usable as one path component.
func sanitize(id string) string {
	if id == "" {
		return "session"
	}
	var sb strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			sb.WriteRune(r)
		default:
			sb.WriteByte('_')
		}
	}
	out := sb.String()
	// Leading dots make the region directory hidden, and a dot-only
	// component would escape the data directory entirely.
	if leading := len(out) - len(strings.TrimLeft(out, ".")); leading > 0 {
		out = strings.Repeat("_", leading) + out[leading:]
	}
	if out == "" {
		return "session"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

func shortID(id string) string {
	compact := strings.ReplaceAll(id, "-", "")
	if len(compact) > 12 {
		compact = compact[:12]
	}
	return compact
}
