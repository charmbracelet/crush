package ansi

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/charmbracelet/glamour/v2/internal/autolink"
	east "github.com/yuin/goldmark-emoji/ast"
	"github.com/yuin/goldmark/ast"
	astext "github.com/yuin/goldmark/extension/ast"
)

// ElementRenderer is called when entering a markdown node.
type ElementRenderer interface {
	Render(w io.Writer, ctx RenderContext) error
}

// StyleOverriderElementRenderer is called when entering a markdown node with a specific style.
type StyleOverriderElementRenderer interface {
	StyleOverrideRender(w io.Writer, ctx RenderContext, style StylePrimitive) error
}

// ElementFinisher is called when leaving a markdown node.
type ElementFinisher interface {
	Finish(w io.Writer, ctx RenderContext) error
}

// An Element is used to instruct the renderer how to handle individual markdown
// nodes.
type Element struct {
	Entering string
	Exiting  string
	Renderer ElementRenderer
	Finisher ElementFinisher
}

// NewElement returns the appropriate render Element for a given node.
func (tr *ANSIRenderer) NewElement(node ast.Node, source []byte) Element {
	ctx := tr.context

	switch node.Kind() {
	// Document
	case ast.KindDocument:
		e := &BlockElement{
			Block:  &bytes.Buffer{},
			Style:  ctx.options.Styles.Document,
			Margin: true,
		}
		return Element{
			Renderer: e,
			Finisher: e,
		}

	// Heading
	case ast.KindHeading:
		n := node.(*ast.Heading)
		he := &HeadingElement{
			Level: n.Level,
			First: node.PreviousSibling() == nil,
		}
		return Element{
			Exiting:  "",
			Renderer: he,
			Finisher: he,
		}

	// Paragraph
	case ast.KindParagraph:
		if node.Parent() != nil {
			kind := node.Parent().Kind()
			if kind == ast.KindListItem {
				return Element{}
			}
		}
		return Element{
			Renderer: &ParagraphElement{
				First: node.PreviousSibling() == nil,
			},
			Finisher: &ParagraphElement{},
		}

	// Blockquote
	case ast.KindBlockquote:
		e := &BlockElement{
			Block:  &bytes.Buffer{},
			Style:  cascadeStyle(ctx.blockStack.Current().Style, ctx.options.Styles.BlockQuote, false),
			Margin: true,
		}
		return Element{
			Entering: "\n",
			Renderer: e,
			Finisher: e,
		}

	// Lists
	case ast.KindList:
		s := ctx.options.Styles.List.StyleBlock
		if s.Indent == nil {
			var i uint
			s.Indent = &i
		}
		n := node.Parent()
		for n != nil {
			if n.Kind() == ast.KindList {
				i := ctx.options.Styles.List.LevelIndent
				s.Indent = &i
				break
			}
			n = n.Parent()
		}

		e := &BlockElement{
			Block:   &bytes.Buffer{},
			Style:   cascadeStyle(ctx.blockStack.Current().Style, s, false),
			Margin:  true,
			Newline: true,
		}
		return Element{
			Entering: "\n",
			Renderer: e,
			Finisher: e,
		}

	case ast.KindListItem:
		var l uint
		var e uint
		l = 1
		n := node
		for n.PreviousSibling() != nil && (n.PreviousSibling().Kind() == ast.KindListItem) {
			l++
			n = n.PreviousSibling()
		}
		if node.Parent().(*ast.List).IsOrdered() {
			e = l
			if node.Parent().(*ast.List).Start != 1 {
				e += uint(node.Parent().(*ast.List).Start) - 1 //nolint: gosec
			}
		}

		post := "\n"
		if (node.LastChild() != nil && node.LastChild().Kind() == ast.KindList) ||
			node.NextSibling() == nil {
			post = ""
		}

		if node.FirstChild() != nil &&
			node.FirstChild().FirstChild() != nil &&
			node.FirstChild().FirstChild().Kind() == astext.KindTaskCheckBox {
			nc := node.FirstChild().FirstChild().(*astext.TaskCheckBox)

			return Element{
				Exiting: post,
				Renderer: &TaskElement{
					Checked: nc.IsChecked,
				},
			}
		}

		return Element{
			Exiting: post,
			Renderer: &ItemElement{
				IsOrdered:   node.Parent().(*ast.List).IsOrdered(),
				Enumeration: e,
			},
		}

	// Text Elements
	case ast.KindText:
		n := node.(*ast.Text)
		s := string(n.Segment.Value(source))

		if n.HardLineBreak() || (n.SoftLineBreak()) {
			s += "\n"
		}
		return Element{
			Renderer: &BaseElement{
				Token: html.UnescapeString(s),
				Style: ctx.options.Styles.Text,
			},
		}

	case ast.KindEmphasis:
		n := node.(*ast.Emphasis)
		var children []ElementRenderer
		nn := n.FirstChild()
		for nn != nil {
			children = append(children, tr.NewElement(nn, source).Renderer)
			nn = nn.NextSibling()
		}
		return Element{
			Renderer: &EmphasisElement{
				Level:    n.Level,
				Children: children,
			},
		}

	case astext.KindStrikethrough:
		n := node.(*astext.Strikethrough)
		s := string(n.Text(source)) //nolint: staticcheck
		style := ctx.options.Styles.Strikethrough

		return Element{
			Renderer: &BaseElement{
				Token: html.UnescapeString(s),
				Style: style,
			},
		}

	case ast.KindThematicBreak:
		return Element{
			Entering: "",
			Exiting:  "",
			Renderer: &BaseElement{
				Style: ctx.options.Styles.HorizontalRule,
			},
		}

	// Links
	case ast.KindLink:
		n := node.(*ast.Link)
		isFooterLinks := !ctx.options.InlineTableLinks && isInsideTable(node)

		var children []ElementRenderer
		content, err := nodeContent(node, source)

		if isFooterLinks && err == nil {
			text := string(content)
			tl := tableLink{
				content:  text,
				href:     string(n.Destination),
				title:    string(n.Title),
				linkType: linkTypeRegular,
			}
			text = linkWithSuffix(tl, ctx.table.tableLinks)
			children = []ElementRenderer{&BaseElement{Token: text}}
		} else {
			nn := n.FirstChild()
			for nn != nil {
				children = append(children, tr.NewElement(nn, source).Renderer)
				nn = nn.NextSibling()
			}
		}

		return Element{
			Renderer: &LinkElement{
				BaseURL:  ctx.options.BaseURL,
				URL:      string(n.Destination),
				Children: children,
				SkipHref: isFooterLinks,
			},
		}
	case ast.KindAutoLink:
		n := node.(*ast.AutoLink)
		u := string(n.URL(source))
		isFooterLinks := !ctx.options.InlineTableLinks && isInsideTable(node)

		var children []ElementRenderer
		nn := n.FirstChild()
		for nn != nil {
			children = append(children, tr.NewElement(nn, source).Renderer)
			nn = nn.NextSibling()
		}

		if len(children) == 0 {
			children = append(children, &BaseElement{Token: u})
		}

		if n.AutoLinkType == ast.AutoLinkEmail && !strings.HasPrefix(strings.ToLower(u), "mailto:") {
			u = "mailto:" + u
		}

		var renderer ElementRenderer
		if isFooterLinks {
			domain := linkDomain(u)
			tl := tableLink{
				content:  domain,
				href:     u,
				linkType: linkTypeAuto,
			}
			if shortned, ok := autolink.Detect(u); ok {
				tl.content = shortned
			}
			text := linkWithSuffix(tl, ctx.table.tableLinks)

			renderer = &LinkElement{
				Children: []ElementRenderer{&BaseElement{Token: text}},
				URL:      u,
				SkipHref: true,
			}
		} else {
			renderer = &LinkElement{
				Children: children,
				URL:      u,
				SkipText: n.AutoLinkType != ast.AutoLinkEmail,
			}
		}
		return Element{Renderer: renderer}

	// Images
	case ast.KindImage:
		n := node.(*ast.Image)
		text := string(n.Text(source)) //nolint: staticcheck
		isFooterLinks := !ctx.options.InlineTableLinks && isInsideTable(node)

		if isFooterLinks {
			if text == "" {
				text = linkDomain(string(n.Destination))
			}
			tl := tableLink{
				title:    string(n.Title),
				content:  text,
				href:     string(n.Destination),
				linkType: linkTypeImage,
			}
			text = linkWithSuffix(tl, ctx.table.tableImages)
		}

		return Element{
			Renderer: &ImageElement{
				Text:     text,
				BaseURL:  ctx.options.BaseURL,
				URL:      string(n.Destination),
				TextOnly: isFooterLinks,
			},
		}

	// Code
	case ast.KindFencedCodeBlock:
		n := node.(*ast.FencedCodeBlock)
		l := n.Lines().Len()
		s := ""
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			s += string(line.Value(source))
		}
		return Element{
			Entering: "\n",
			Renderer: &CodeBlockElement{
				Code:     s,
				Language: string(n.Language(source)),
			},
		}

	case ast.KindCodeBlock:
		n := node.(*ast.CodeBlock)
		l := n.Lines().Len()
		s := ""
		for i := 0; i < l; i++ {
			line := n.Lines().At(i)
			s += string(line.Value(source))
		}
		return Element{
			Entering: "\n",
			Renderer: &CodeBlockElement{
				Code: s,
			},
		}

	case ast.KindCodeSpan:
		n := node.(*ast.CodeSpan)
		s := string(n.Text(source)) //nolint: staticcheck
		return Element{
			Renderer: &CodeSpanElement{
				Text:  html.UnescapeString(s),
				Style: cascadeStyle(ctx.blockStack.Current().Style, ctx.options.Styles.Code, false).StylePrimitive,
			},
		}

	// Tables
	case astext.KindTable:
		table := node.(*astext.Table)
		te := &TableElement{
			table:  table,
			source: source,
		}
		return Element{
			Entering: "\n",
			Exiting:  "\n",
			Renderer: te,
			Finisher: te,
		}

	case astext.KindTableCell:
		n := node.(*astext.TableCell)
		var children []ElementRenderer
		nn := n.FirstChild()
		for nn != nil {
			children = append(children, tr.NewElement(nn, source).Renderer)
			nn = nn.NextSibling()
		}

		r := &TableCellElement{
			Children: children,
			Head:     node.Parent().Kind() == astext.KindTableHeader,
		}
		return Element{
			Renderer: r,
		}

	case astext.KindTableHeader:
		return Element{
			Finisher: &TableHeadElement{},
		}
	case astext.KindTableRow:
		return Element{
			Finisher: &TableRowElement{},
		}

	// HTML Elements
	case ast.KindHTMLBlock:
		n := node.(*ast.HTMLBlock)
		return Element{
			Renderer: &BaseElement{
				Token: ctx.SanitizeHTML(string(n.Text(source)), true), //nolint: staticcheck
				Style: ctx.options.Styles.HTMLBlock.StylePrimitive,
			},
		}
	case ast.KindRawHTML:
		n := node.(*ast.RawHTML)
		return Element{
			Renderer: &BaseElement{
				Token: ctx.SanitizeHTML(string(n.Text(source)), true), //nolint: staticcheck
				Style: ctx.options.Styles.HTMLSpan.StylePrimitive,
			},
		}

	// Definition Lists
	case astext.KindDefinitionList:
		e := &BlockElement{
			Block:   &bytes.Buffer{},
			Style:   cascadeStyle(ctx.blockStack.Current().Style, ctx.options.Styles.DefinitionList, false),
			Margin:  true,
			Newline: true,
		}
		return Element{
			Renderer: e,
			Finisher: e,
		}

	case astext.KindDefinitionTerm:
		return Element{
			Entering: "\n",
			Renderer: &BaseElement{
				Style: ctx.options.Styles.DefinitionTerm,
			},
		}

	case astext.KindDefinitionDescription:
		return Element{
			Exiting: "\n",
			Renderer: &BaseElement{
				Style: ctx.options.Styles.DefinitionDescription,
			},
		}

	// Handled by parents
	case astext.KindTaskCheckBox:
		// handled by KindListItem
		return Element{}
	case ast.KindTextBlock:
		return Element{}

	case east.KindEmoji:
		n := node.(*east.Emoji)
		return Element{
			Renderer: &BaseElement{
				Token: string(n.Value.Unicode),
			},
		}

	// Unknown case
	default:
		fmt.Println("Warning: unhandled element", node.Kind().String())
		return Element{}
	}
}
