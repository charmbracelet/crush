package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/x/cellbuf"
)

// A ParagraphElement is used to render individual paragraphs.
type ParagraphElement struct {
	First bool
}

// Render renders a ParagraphElement.
func (e *ParagraphElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	rules := ctx.options.Styles.Paragraph

	if !e.First {
		_, _ = io.WriteString(w, "\n")
	}
	be := BlockElement{
		Block: &bytes.Buffer{},
		Style: cascadeStyle(bs.Current().Style, rules, false),
	}
	bs.Push(be)

	_, _ = renderText(w, bs.Parent().Style.StylePrimitive, rules.BlockPrefix)
	_, _ = renderText(bs.Current().Block, bs.Current().Style.StylePrimitive, rules.Prefix)
	return nil
}

// Finish finishes rendering a ParagraphElement.
func (e *ParagraphElement) Finish(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	rules := bs.Current().Style

	mw := NewMarginWriter(ctx, w, rules)
	if len(strings.TrimSpace(bs.Current().Block.String())) > 0 {
		blk := bs.Current().Block.String()
		if !ctx.options.PreserveNewLines {
			blk = strings.ReplaceAll(blk, "\n", " ")
		}
		flow := cellbuf.Wrap(blk, int(bs.Width(ctx)), "") //nolint: gosec

		_, err := io.WriteString(mw, flow)
		if err != nil {
			return fmt.Errorf("glamour: error writing to writer: %w", err)
		}
		_, _ = io.WriteString(mw, "\n")
	}

	_, _ = renderText(w, bs.Current().Style.StylePrimitive, rules.Suffix)
	_, _ = renderText(w, bs.Parent().Style.StylePrimitive, rules.BlockSuffix)

	bs.Current().Block.Reset()
	bs.Pop()
	return nil
}
