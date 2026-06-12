package agent

import (
	"strings"
)

// stripAnalysisBlock removes <analysis>...</analysis> blocks from summary text.
// Used as a safety net when loading previously stored summaries that may contain
// analysis blocks from before the streaming router was implemented.
func stripAnalysisBlock(text string) string {
	for {
		start := strings.Index(text, "<analysis>")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], "</analysis>")
		if end == -1 {
			break
		}
		end += start + len("</analysis>")
		// Trim trailing whitespace after the closing tag.
		rest := strings.TrimLeft(text[end:], " \t\n\r")
		text = text[:start] + rest
	}
	return strings.TrimSpace(text)
}

// analysisRouter streams text deltas and routes <analysis>...</analysis> content
// to reasoning (thinking display) while passing everything else to normal content.
// Tags may span multiple deltas so it buffers partial matches.
//
// Used for all models during summarization. For non-thinking models, the summary
// prompt instructs the model to wrap analysis in <analysis> tags. For thinking
// models, this acts as a defensive safety net in case <analysis> tags leak into
// text output alongside native reasoning tokens.
type analysisRouter struct {
	inAnalysis  bool
	buffer      strings.Builder
	onReasoning func(string) error
	onContent   func(string) error
	lastErr     error
}

func newAnalysisRouter(onReasoning, onContent func(string) error) *analysisRouter {
	return &analysisRouter{onReasoning: onReasoning, onContent: onContent}
}

func (r *analysisRouter) write(delta string) error {
	r.buffer.WriteString(delta)
	for r.buffer.Len() > 0 {
		text := r.buffer.String()
		if r.inAnalysis {
			found, err := r.processSide(text, "</analysis>", r.onReasoning, false)
			if err != nil {
				return err
			}
			if !found && r.buffer.Len() == len(text) {
				return nil
			}
		} else {
			found, err := r.processSide(text, "<analysis>", r.onContent, true)
			if err != nil {
				return err
			}
			if !found && r.buffer.Len() == len(text) {
				return nil
			}
		}
	}
	return nil
}

// processSide handles one side of the tag routing. It searches for tag in text,
// emits content before the tag via handler, then transitions state. If the tag
// is not found, it emits the safe prefix (everything except the last len(tag)
// bytes which might be a partial tag) and returns false.
func (r *analysisRouter) processSide(text, tag string, handler func(string) error, enteringAnalysis bool) (bool, error) {
	if idx := strings.Index(text, tag); idx != -1 {
		if idx > 0 {
			if err := handler(text[:idx]); err != nil {
				r.lastErr = err
				return false, err
			}
		}
		if enteringAnalysis {
			// Transitioning into analysis mode; no extra newline needed.
			r.buffer.Reset()
			r.buffer.WriteString(text[idx+len(tag):])
			r.inAnalysis = true
		} else {
			// Exiting analysis mode; append a newline to separate reasoning blocks.
			if err := handler("\n"); err != nil {
				r.lastErr = err
				return false, err
			}
			r.buffer.Reset()
			r.buffer.WriteString(text[idx+len(tag):])
			r.inAnalysis = false
		}
		return true, nil
	}
	tagLen := len(tag)
	if len(text) > tagLen {
		safe := text[:len(text)-tagLen]
		if err := handler(safe); err != nil {
			r.lastErr = err
			return false, err
		}
		r.buffer.Reset()
		r.buffer.WriteString(text[len(text)-tagLen:])
	}
	return false, nil
}

// flush sends any remaining buffered content to the appropriate handler.
func (r *analysisRouter) flush() error {
	text := r.buffer.String()
	if text == "" {
		return r.lastErr
	}
	var err error
	if r.inAnalysis {
		err = r.onReasoning(text)
	} else {
		err = r.onContent(text)
	}
	r.buffer.Reset()
	if err != nil {
		r.lastErr = err
		return err
	}
	return r.lastErr
}
