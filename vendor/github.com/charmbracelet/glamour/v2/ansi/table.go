package ansi

import (
	"bytes"
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/lipgloss/v2/table"
	astext "github.com/yuin/goldmark/extension/ast"
)

// A TableElement is used to render tables.
type TableElement struct {
	lipgloss *table.Table
	table    *astext.Table
	header   []string
	row      []string
	source   []byte

	tableImages []tableLink
	tableLinks  []tableLink
}

// A TableRowElement is used to render a single row in a table.
type TableRowElement struct{}

// A TableHeadElement is used to render a table's head element.
type TableHeadElement struct{}

// A TableCellElement is used to render a single cell in a row.
type TableCellElement struct {
	Children []ElementRenderer
	Head     bool
}

// Render renders a TableElement.
func (e *TableElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack

	var indentation uint
	var margin uint
	rules := ctx.options.Styles.Table
	if rules.Indent != nil {
		indentation = *rules.Indent
	}
	if rules.Margin != nil {
		margin = *rules.Margin
	}

	iw := NewIndentWriter(w, int(indentation+margin), func(_ io.Writer) { //nolint:gosec
		_, _ = renderText(w, bs.Current().Style.StylePrimitive, " ")
	})

	style := bs.With(rules.StylePrimitive)

	_, _ = renderText(iw, bs.Current().Style.StylePrimitive, rules.BlockPrefix)
	_, _ = renderText(iw, style, rules.Prefix)
	width := int(ctx.blockStack.Width(ctx)) //nolint: gosec

	wrap := true
	if ctx.options.TableWrap != nil {
		wrap = *ctx.options.TableWrap
	}
	ctx.table.lipgloss = table.New().Width(width).Wrap(wrap)

	if err := e.collectLinksAndImages(ctx); err != nil {
		return err
	}

	return nil
}

func (e *TableElement) setStyles(ctx RenderContext) {
	docRules := ctx.options.Styles.Document
	if docRules.BackgroundColor != nil {
		baseStyle := lipgloss.NewStyle().Background(lipgloss.Color(*docRules.BackgroundColor))
		ctx.table.lipgloss.BaseStyle(baseStyle)
	}

	ctx.table.lipgloss = ctx.table.lipgloss.StyleFunc(func(_, col int) lipgloss.Style {
		st := lipgloss.NewStyle().Inline(false)
		// Default Styles
		st = st.Margin(0, 1)

		// Override with custom styles
		if m := ctx.options.Styles.Table.Margin; m != nil {
			st = st.Padding(0, int(*m)) //nolint: gosec
		}
		switch e.table.Alignments[col] {
		case astext.AlignLeft:
			st = st.Align(lipgloss.Left).PaddingRight(0)
		case astext.AlignCenter:
			st = st.Align(lipgloss.Center)
		case astext.AlignRight:
			st = st.Align(lipgloss.Right).PaddingLeft(0)
		case astext.AlignNone:
			// do nothing
		}

		return st
	})
}

func (e *TableElement) setBorders(ctx RenderContext) {
	rules := ctx.options.Styles.Table
	border := lipgloss.NormalBorder()

	if rules.RowSeparator != nil && rules.ColumnSeparator != nil {
		border = lipgloss.Border{
			Top:    *rules.RowSeparator,
			Bottom: *rules.RowSeparator,
			Left:   *rules.ColumnSeparator,
			Right:  *rules.ColumnSeparator,
			Middle: *rules.CenterSeparator,
		}
	}
	ctx.table.lipgloss.Border(border)
	ctx.table.lipgloss.BorderTop(false)
	ctx.table.lipgloss.BorderLeft(false)
	ctx.table.lipgloss.BorderRight(false)
	ctx.table.lipgloss.BorderBottom(false)
}

// Finish finishes rendering a TableElement.
func (e *TableElement) Finish(_ io.Writer, ctx RenderContext) error {
	defer func() {
		ctx.table.lipgloss = nil
		ctx.table.tableImages = nil
		ctx.table.tableLinks = nil
	}()

	rules := ctx.options.Styles.Table

	e.setStyles(ctx)
	e.setBorders(ctx)

	ow := ctx.blockStack.Current().Block
	if _, err := ow.WriteString(ctx.table.lipgloss.String()); err != nil {
		return fmt.Errorf("glamour: error writing to buffer: %w", err)
	}

	_, _ = renderText(ow, ctx.blockStack.With(rules.StylePrimitive), rules.Suffix)
	_, _ = renderText(ow, ctx.blockStack.Current().Style.StylePrimitive, rules.BlockSuffix)

	e.printTableLinks(ctx)

	ctx.table.lipgloss = nil
	return nil
}

// Finish finishes rendering a TableRowElement.
func (e *TableRowElement) Finish(_ io.Writer, ctx RenderContext) error {
	if ctx.table.lipgloss == nil {
		return nil
	}

	ctx.table.lipgloss.Row(ctx.table.row...)
	ctx.table.row = []string{}
	return nil
}

// Finish finishes rendering a TableHeadElement.
func (e *TableHeadElement) Finish(_ io.Writer, ctx RenderContext) error {
	if ctx.table.lipgloss == nil {
		return nil
	}

	ctx.table.lipgloss.Headers(ctx.table.header...)
	ctx.table.header = []string{}
	return nil
}

// Render renders a TableCellElement.
func (e *TableCellElement) Render(_ io.Writer, ctx RenderContext) error {
	var b bytes.Buffer
	style := ctx.options.Styles.Table.StylePrimitive
	for _, child := range e.Children {
		if r, ok := child.(StyleOverriderElementRenderer); ok {
			if err := r.StyleOverrideRender(&b, ctx, style); err != nil {
				return fmt.Errorf("glamour: error rendering with style: %w", err)
			}
		} else {
			var bb bytes.Buffer
			if err := child.Render(&bb, ctx); err != nil {
				return fmt.Errorf("glamour: error rendering: %w", err)
			}
			el := &BaseElement{
				Token: bb.String(),
				Style: style,
			}
			if err := el.Render(&b, ctx); err != nil {
				return err
			}
		}
	}

	if e.Head {
		ctx.table.header = append(ctx.table.header, b.String())
	} else {
		ctx.table.row = append(ctx.table.row, b.String())
	}

	return nil
}
