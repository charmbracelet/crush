package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
)

// MarginWriter is a Writer that applies indentation and padding around
// whatever you write to it.
type MarginWriter struct {
	w  io.Writer
	iw *IndentWriter
}

// NewMarginWriter returns a new MarginWriter.
func NewMarginWriter(ctx RenderContext, w io.Writer, rules StyleBlock) *MarginWriter {
	bs := ctx.blockStack

	var indentation uint
	var margin uint
	if rules.Indent != nil {
		indentation = *rules.Indent
	}
	if rules.Margin != nil {
		margin = *rules.Margin
	}

	pw := NewPaddingWriter(w, int(bs.Width(ctx)), func(_ io.Writer) { //nolint:gosec
		_, _ = renderText(w, rules.StylePrimitive, " ")
	})

	ic := " "
	if rules.IndentToken != nil {
		ic = *rules.IndentToken
	}
	iw := NewIndentWriter(pw, int(indentation+margin), func(_ io.Writer) { //nolint:gosec
		_, _ = renderText(w, bs.Parent().Style.StylePrimitive, ic)
	})

	return &MarginWriter{
		w:  cellbuf.NewPenWriter(w),
		iw: iw,
	}
}

func (w *MarginWriter) Write(b []byte) (int, error) {
	n, err := w.iw.Write(b)
	if err != nil {
		return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
	}
	return n, nil
}

// PaddingFunc is a function that applies padding around whatever you write to it.
type PaddingFunc = func(w io.Writer)

// PaddingWriter is a writer that applies padding around whatever you write to
// it.
type PaddingWriter struct {
	Padding int
	PadFunc PaddingFunc
	w       *cellbuf.PenWriter
	cache   bytes.Buffer
}

// NewPaddingWriter returns a new PaddingWriter.
func NewPaddingWriter(w io.Writer, padding int, padFunc PaddingFunc) *PaddingWriter {
	return &PaddingWriter{
		Padding: padding,
		PadFunc: padFunc,
		w:       cellbuf.NewPenWriter(w),
	}
}

// Write writes to the padding writer.
func (w *PaddingWriter) Write(p []byte) (int, error) {
	for i := 0; i < len(p); i++ {
		if p[i] == '\n' { //nolint:nestif
			line := w.cache.String()
			linew := ansi.StringWidth(line)
			if w.Padding > 0 && linew < w.Padding {
				if w.PadFunc != nil {
					for n := 0; n < w.Padding-linew; n++ {
						w.PadFunc(w.w)
					}
				} else {
					_, err := io.WriteString(w.w, strings.Repeat(" ", w.Padding-linew))
					if err != nil {
						return 0, fmt.Errorf("glamour: error writing padding: %w", err)
					}
				}
			}
			w.cache.Reset()
		} else {
			w.cache.WriteByte(p[i])
		}

		_, err := w.w.Write(p[i : i+1])
		if err != nil {
			return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
		}
	}

	return len(p), nil
}

// IndentFunc is a function that applies indentation around whatever you write to
// it.
type IndentFunc = func(w io.Writer)

// IndentWriter is a writer that applies indentation around whatever you write to
// it.
type IndentWriter struct {
	Indent     int
	IndentFunc PaddingFunc
	w          io.Writer
	pw         *cellbuf.PenWriter
	skipIndent bool
}

// NewIndentWriter returns a new IndentWriter.
func NewIndentWriter(w io.Writer, indent int, indentFunc IndentFunc) *IndentWriter {
	return &IndentWriter{
		Indent:     indent,
		IndentFunc: indentFunc,
		pw:         cellbuf.NewPenWriter(w),
		w:          w,
	}
}

func (w *IndentWriter) resetPen() {
	style := w.pw.Style()
	link := w.pw.Link()
	if !style.Empty() {
		_, _ = io.WriteString(w.w, ansi.ResetStyle)
	}
	if !link.Empty() {
		_, _ = io.WriteString(w.w, ansi.ResetHyperlink())
	}
}

func (w *IndentWriter) restorePen() {
	style := w.pw.Style()
	link := w.pw.Link()
	if !style.Empty() {
		_, _ = io.WriteString(w.w, style.Sequence())
	}
	if !link.Empty() {
		_, _ = io.WriteString(w.w, ansi.SetHyperlink(link.URL, link.Params))
	}
}

// Write writes to the indentation writer.
func (w *IndentWriter) Write(p []byte) (int, error) {
	for i := 0; i < len(p); i++ {
		if !w.skipIndent {
			w.resetPen()
			if w.IndentFunc != nil {
				for i := 0; i < w.Indent; i++ {
					w.IndentFunc(w.pw)
				}
			} else {
				_, err := io.WriteString(w.pw, strings.Repeat(" ", w.Indent))
				if err != nil {
					return 0, fmt.Errorf("glamour: error writing indentation: %w", err)
				}
			}

			w.skipIndent = true
			w.restorePen()
		}

		if p[i] == '\n' {
			w.skipIndent = false
		}

		_, err := w.pw.Write(p[i : i+1])
		if err != nil {
			return 0, fmt.Errorf("glamour: error writing bytes: %w", err)
		}
	}

	return len(p), nil
}
