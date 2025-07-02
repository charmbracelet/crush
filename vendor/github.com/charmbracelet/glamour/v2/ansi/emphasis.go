package ansi

import (
	"fmt"
	"io"
)

// A EmphasisElement is used to render emphasis.
type EmphasisElement struct {
	Children []ElementRenderer
	Level    int
}

// Render renders a EmphasisElement.
func (e *EmphasisElement) Render(w io.Writer, ctx RenderContext) error {
	style := ctx.options.Styles.Emph
	if e.Level > 1 {
		style = ctx.options.Styles.Strong
	}

	return e.doRender(w, ctx, style)
}

// StyleOverrideRender renders a EmphasisElement with a given style.
func (e *EmphasisElement) StyleOverrideRender(w io.Writer, ctx RenderContext, style StylePrimitive) error {
	base := ctx.options.Styles.Emph
	if e.Level > 1 {
		base = ctx.options.Styles.Strong
	}
	return e.doRender(w, ctx, cascadeStylePrimitives(base, style))
}

func (e *EmphasisElement) doRender(w io.Writer, ctx RenderContext, style StylePrimitive) error {
	for _, child := range e.Children {
		if r, ok := child.(StyleOverriderElementRenderer); ok {
			if err := r.StyleOverrideRender(w, ctx, style); err != nil {
				return fmt.Errorf("glamour: error rendering with style: %w", err)
			}
		} else {
			if err := child.Render(w, ctx); err != nil {
				return fmt.Errorf("glamour: error rendering: %w", err)
			}
		}
	}

	return nil
}
