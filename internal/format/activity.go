package format

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/tone"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

const (
	activityFrameThinking = iota
	activityFrameTool
)

const (
	activityAnimationInterval = 500 * time.Millisecond
	generatingLabel           = "Generating…"
)

// ActivityPresenterOptions configures non-interactive stderr presentation.
type ActivityPresenterOptions struct {
	Output      io.Writer
	WorkingDir  string
	TTY         bool
	HideSpinner bool
	Styles      *styles.Styles
	Width       int
}

type activityLine struct {
	kind      int
	id        string
	text      string
	rendered  string
	startedAt time.Time
	finished  bool
}

// ActivityPresenter renders activity lines above static progress text.
// Completed events are deduplicated for terminal reconciliation.
type ActivityPresenter struct {
	out         io.Writer
	workingDir  string
	tty         bool
	hideSpinner bool
	sty         *styles.Styles
	width       int

	activities         []activityLine
	animationFrame     uint64
	textStreaming      bool
	thinkingStarted    map[string]struct{}
	thinkingPrinted    map[string]struct{}
	toolsStarted       map[string]struct{}
	toolsPrinted       map[string]struct{}
	staticThinkingLine bool
	closed             bool

	animationStop     chan struct{}
	animationDone     chan struct{}
	animationStopOnce sync.Once
	renderMu          sync.Mutex
}

func NewActivityPresenter(ctx context.Context, _ context.CancelFunc, opts ActivityPresenterOptions) *ActivityPresenter {
	output := opts.Output
	if output == nil {
		output = io.Discard
	}
	sty := opts.Styles
	if sty == nil {
		defaultStyles := styles.CharmtonePantera()
		sty = &defaultStyles
	}
	width := opts.Width
	if width <= 0 {
		width = 100
	}
	p := &ActivityPresenter{
		out:             output,
		workingDir:      opts.WorkingDir,
		tty:             opts.TTY,
		hideSpinner:     opts.HideSpinner,
		sty:             sty,
		width:           width,
		thinkingStarted: make(map[string]struct{}),
		thinkingPrinted: make(map[string]struct{}),
		toolsStarted:    make(map[string]struct{}),
		toolsPrinted:    make(map[string]struct{}),
	}
	if p.animated() {
		p.animationStop = make(chan struct{})
		p.animationDone = make(chan struct{})
		_, _ = fmt.Fprint(p.out, generatingLabel)
		go p.animate(ctx)
	}
	return p
}

func (p *ActivityPresenter) animated() bool {
	return p.tty && !p.hideSpinner
}

func (p *ActivityPresenter) Handle(activity notify.RunActivity) {
	p.renderMu.Lock()
	defer p.renderMu.Unlock()
	if p.closed {
		return
	}
	switch activity.Type {
	case notify.RunActivityThinkingStarted:
		p.thinking(activity)
	case notify.RunActivityThinkingFinished:
		p.thought(activity)
	case notify.RunActivityToolStarted:
		p.toolStarted(activity)
	case notify.RunActivityToolFinished:
		p.tool(activity)
	}
}

func (p *ActivityPresenter) Reconcile(activities []notify.RunActivity) {
	for _, activity := range activities {
		switch activity.Type {
		case notify.RunActivityThinkingFinished, notify.RunActivityToolFinished:
			p.Handle(activity)
		}
	}
}

func (p *ActivityPresenter) thinking(activity notify.RunActivity) {
	if _, started := p.thinkingStarted[activity.MessageID]; started {
		return
	}
	if _, printed := p.thinkingPrinted[activity.MessageID]; printed {
		return
	}
	p.thinkingStarted[activity.MessageID] = struct{}{}
	if p.textStreaming {
		return
	}
	if p.animated() {
		index, added := p.upsert(activityFrameThinking, activity.MessageID, "", false)
		p.displayActivity(index, added)
		return
	}
	if !p.staticThinkingLine {
		p.print("✦ Thinking…")
		p.staticThinkingLine = true
	}
}

func (p *ActivityPresenter) thought(activity notify.RunActivity) {
	if _, printed := p.thinkingPrinted[activity.MessageID]; printed {
		return
	}
	p.thinkingPrinted[activity.MessageID] = struct{}{}
	p.staticThinkingLine = false
	text := p.sty.Messages.ThinkingFooterTitle.Render("✦ Thought for ") +
		p.sty.Messages.ThinkingFooterDuration.Render(activity.ThinkingDuration.String())
	if p.textStreaming {
		p.print(text)
		return
	}
	p.completeActivity(activityFrameThinking, activity.MessageID, text)
}

func (p *ActivityPresenter) toolStarted(activity notify.RunActivity) {
	toolCallID := activityToolCallID(activity)
	if _, printed := p.toolsPrinted[toolCallID]; printed {
		return
	}
	if _, started := p.toolsStarted[toolCallID]; started {
		return
	}
	p.toolsStarted[toolCallID] = struct{}{}
	if p.textStreaming {
		return
	}
	line := ToolSummaryWithStatus(p.sty, activity.ToolCall, ToolStatusRunning, p.toolWidth(), p.workingDir)
	if p.animated() {
		index, added := p.upsert(activityFrameTool, toolCallID, line, false)
		p.displayActivity(index, added)
		return
	}
	p.print(line)
}

func (p *ActivityPresenter) tool(activity notify.RunActivity) {
	toolCallID := activityToolCallID(activity)
	if _, printed := p.toolsPrinted[toolCallID]; printed {
		return
	}
	p.toolsPrinted[toolCallID] = struct{}{}
	delete(p.toolsStarted, toolCallID)
	status := ToolStatusSuccess
	if activity.Cancelled {
		status = ToolStatusCanceled
	} else if activity.ToolResult.IsError {
		status = ToolStatusError
	}
	line := ToolSummaryWithStatus(p.sty, activity.ToolCall, status, p.toolWidth(), p.workingDir)
	if p.textStreaming {
		p.print(line)
		return
	}
	p.completeActivity(activityFrameTool, toolCallID, line)
}

func (p *ActivityPresenter) completeActivity(kind int, id, text string) {
	if !p.animated() {
		p.print(text)
		return
	}
	index, added := p.upsert(kind, id, text, true)
	p.displayActivity(index, added)
}

func (p *ActivityPresenter) upsert(kind int, id, text string, finished bool) (int, bool) {
	for i := range p.activities {
		if p.activities[i].kind == kind && p.activities[i].id == id {
			p.activities[i].text = text
			p.activities[i].finished = finished
			return i, false
		}
	}
	p.activities = append(p.activities, activityLine{
		kind:      kind,
		id:        id,
		text:      text,
		startedAt: time.Now(),
		finished:  finished,
	})
	return len(p.activities) - 1, true
}

func (p *ActivityPresenter) toolWidth() int {
	return max(20, min(p.width-2, 100))
}

func activityToolCallID(activity notify.RunActivity) string {
	if activity.ToolResult.ToolCallID != "" {
		return activity.ToolResult.ToolCallID
	}
	if activity.ToolCall.ID != "" {
		return activity.ToolCall.ID
	}
	return activity.MessageID + ":" + activity.ToolCall.Name
}

// TextStarted stops transient output before assistant text reaches stdout.
func (p *ActivityPresenter) TextStarted() {
	p.renderMu.Lock()
	defer p.renderMu.Unlock()
	if p.textStreaming {
		return
	}
	p.textStreaming = true
	if !p.animated() {
		return
	}
	previousCount := len(p.activities)
	p.activities = slices.DeleteFunc(p.activities, func(activity activityLine) bool {
		return !activity.finished
	})
	p.redraw(previousCount)
	if len(p.activities) > 0 {
		_, _ = fmt.Fprint(p.out, "\n")
	}
}

// StopProgress stops animations and removes only the progress label, keeping
// activity rows intact for completion reconciliation.
func (p *ActivityPresenter) StopProgress() {
	if p.animationDone == nil {
		return
	}
	p.animationStopOnce.Do(func() { close(p.animationStop) })
	p.clearProgress()
}

// Close stops animations without erasing partial assistant output.
func (p *ActivityPresenter) Close() {
	p.renderMu.Lock()
	p.closed = true
	p.renderMu.Unlock()
	if p.animationDone != nil {
		p.animationStopOnce.Do(func() { close(p.animationStop) })
		<-p.animationDone
	}
}

func (p *ActivityPresenter) animate(ctx context.Context) {
	defer close(p.animationDone)
	ticker := time.NewTicker(activityAnimationInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			p.animateChangedLines()
		case <-ctx.Done():
			p.clearProgress()
			return
		case <-p.animationStop:
			p.clearProgress()
			return
		}
	}
}

func (p *ActivityPresenter) animateChangedLines() {
	p.renderMu.Lock()
	defer p.renderMu.Unlock()
	if p.textStreaming || p.closed {
		return
	}
	p.animationFrame++
	for i := range p.activities {
		if !p.activities[i].finished {
			p.renderLine(i)
		}
	}
}

func (p *ActivityPresenter) displayActivity(index int, added bool) {
	if added {
		text := p.activityText(p.activities[index])
		p.activities[index].rendered = text
		if index > 0 {
			_, _ = fmt.Fprint(p.out, ansi.CursorUp(1)+"\r"+ansi.EraseDisplay(0))
		} else {
			_, _ = fmt.Fprint(p.out, "\r"+ansi.EraseEntireLine)
		}
		_, _ = fmt.Fprint(p.out, text+"\n\n"+generatingLabel)
		return
	}
	p.renderLine(index)
}

func (p *ActivityPresenter) renderLine(index int) {
	line := &p.activities[index]
	text := p.activityText(*line)
	if text == line.rendered {
		return
	}
	distance := len(p.activities) - index + 1
	_, _ = fmt.Fprint(p.out, ansi.CursorUp(distance)+"\r"+ansi.EraseEntireLine+text+ansi.CursorDown(distance)+"\r")
	line.rendered = text
}

func (p *ActivityPresenter) redraw(previousCount int) {
	if previousCount > 0 {
		_, _ = fmt.Fprint(p.out, ansi.CursorUp(previousCount+1))
	}
	_, _ = fmt.Fprint(p.out, "\r"+ansi.EraseDisplay(0))
	for _, activity := range p.activities {
		text := p.activityText(activity)
		_, _ = fmt.Fprintln(p.out, text)
	}
	if !p.textStreaming && !p.closed {
		if len(p.activities) > 0 {
			_, _ = fmt.Fprint(p.out, "\n")
		}
		_, _ = fmt.Fprint(p.out, generatingLabel)
	}
}

func (p *ActivityPresenter) clearProgress() {
	p.renderMu.Lock()
	defer p.renderMu.Unlock()
	if p.textStreaming {
		return
	}
	_, _ = fmt.Fprint(p.out, "\r"+ansi.EraseEntireLine)
}

func (p *ActivityPresenter) activityText(activity activityLine) string {
	if activity.finished {
		return activity.text
	}
	switch activity.kind {
	case activityFrameThinking:
		return p.sty.Messages.ThinkingFooterTitle.Render("✦ Thinking…")
	case activityFrameTool:
		color := tone.Julep
		if p.animationFrame%2 != 0 {
			color = tone.Guac
		}
		icon := lipgloss.NewStyle().Foreground(color).Render("●")
		return strings.Replace(activity.text, ToolIcon(p.sty, ToolStatusRunning), icon, 1)
	default:
		return activity.text
	}
}

func (p *ActivityPresenter) print(line string) {
	if !p.tty {
		line = ansi.Strip(line)
	}
	line = strings.TrimRight(line, "\r\n")
	_, _ = fmt.Fprintln(p.out, line)
}
