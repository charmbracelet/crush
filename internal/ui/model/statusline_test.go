package model

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/ui/common"
	uv "github.com/charmbracelet/ultraviolet"
)

func newTestStatusLine(width int) *StatusLine {
	s := NewStatusLine(common.DefaultCommon(nil))
	s.SetWidth(width)
	return s
}

func TestStatusLineDraw(t *testing.T) {
	s := newTestStatusLine(40)
	s.SetContent("hello crush")
	scr := uv.NewScreenBuffer(40, 1)
	s.Draw(scr, uv.Rect(0, 0, 40, 1))
	if got := strings.TrimRight(scr.Render(), " "); got != "hello crush" {
		t.Errorf("expected %q, got %q", "hello crush", got)
	}
}

func TestStatusLineDrawEmpty(t *testing.T) {
	s := newTestStatusLine(40)
	scr := uv.NewScreenBuffer(40, 1)
	s.Draw(scr, uv.Rect(0, 0, 40, 1))
	if got := scr.Render(); strings.TrimSpace(got) != "" {
		t.Errorf("expected empty screen, got %q", got)
	}
}

func TestStatusLineDrawTruncates(t *testing.T) {
	s := newTestStatusLine(10)
	s.SetContent("this line is way too long for the status bar")
	scr := uv.NewScreenBuffer(10, 1)
	s.Draw(scr, uv.Rect(0, 0, 10, 1))
	got := scr.Render()
	if len([]rune(strings.ReplaceAll(got, "…", ""))) > 10 {
		t.Errorf("expected truncated output, got %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("expected ellipsis, got %q", got)
	}
}

func TestStatusLineDrawFirstLineOnly(t *testing.T) {
	s := newTestStatusLine(40)
	s.SetContent("first\nsecond")
	scr := uv.NewScreenBuffer(40, 1)
	s.Draw(scr, uv.Rect(0, 0, 40, 1))
	if got := strings.TrimRight(scr.Render(), " "); got != "first" {
		t.Errorf("expected only the first line, got %q", got)
	}
}

func TestRunStatusLineOutput(t *testing.T) {
	cmd := runStatusLine("printf 'hello from status line'", []byte(`{}`))
	msg := cmd()
	out, ok := msg.(statusLineOutputMsg)
	if !ok {
		t.Fatalf("expected statusLineOutputMsg, got %T", msg)
	}
	if out.content != "hello from status line" {
		t.Errorf("unexpected content %q", out.content)
	}
}

func TestRunStatusLineReceivesStdin(t *testing.T) {
	cmd := runStatusLine("cat", []byte(`{"session_id":"abc"}`))
	msg := cmd()
	out, ok := msg.(statusLineOutputMsg)
	if !ok {
		t.Fatalf("expected statusLineOutputMsg, got %T", msg)
	}
	if out.content != `{"session_id":"abc"}` {
		t.Errorf("unexpected content %q", out.content)
	}
}

func TestRunStatusLineErrorShowsStderr(t *testing.T) {
	cmd := runStatusLine("echo broken >&2; exit 1", []byte(`{}`))
	msg := cmd()
	out, ok := msg.(statusLineOutputMsg)
	if !ok {
		t.Fatalf("expected statusLineOutputMsg, got %T", msg)
	}
	if out.content != "broken" {
		t.Errorf("expected stderr in content, got %q", out.content)
	}
}

func TestStatusLineTick(t *testing.T) {
	cmd := statusLineTick()
	// The tick fires after statusLineInterval; just assert it returns a
	// command that eventually produces a statusLineTickMsg.
	done := make(chan any, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if _, ok := msg.(statusLineTickMsg); !ok {
			t.Errorf("expected statusLineTickMsg, got %T", msg)
		}
	case <-time.After(3 * statusLineInterval):
		t.Fatal("status line tick did not fire")
	}
}
