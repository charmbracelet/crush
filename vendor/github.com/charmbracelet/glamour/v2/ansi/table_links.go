package ansi

import (
	"bytes"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour/v2/internal/autolink"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/slice"
	"github.com/yuin/goldmark/ast"
	astext "github.com/yuin/goldmark/extension/ast"
)

type tableLink struct {
	href     string
	title    string
	content  string
	linkType linkType
}

type linkType int

const (
	_ linkType = iota
	linkTypeAuto
	linkTypeImage
	linkTypeRegular
)

func (e *TableElement) printTableLinks(ctx RenderContext) {
	if !e.shouldPrintTableLinks(ctx) {
		return
	}

	w := ctx.blockStack.Current().Block
	termWidth := int(ctx.blockStack.Width(ctx)) //nolint: gosec

	renderLinkText := func(link tableLink, position, padding int) string {
		token := strings.Repeat(" ", padding)
		style := ctx.options.Styles.LinkText

		switch link.linkType {
		case linkTypeAuto, linkTypeRegular:
			token += fmt.Sprintf("[%d]: %s", position, link.content)
		case linkTypeImage:
			token += link.content
			style = ctx.options.Styles.ImageText
			style.Prefix = fmt.Sprintf("[%d]: %s", position, style.Prefix)
		}

		var b bytes.Buffer
		el := &BaseElement{Token: token, Style: style}
		_ = el.Render(io.MultiWriter(w, &b), ctx)
		return b.String()
	}

	renderLinkHref := func(link tableLink, linkText string) {
		hyperlink, resetHyperlink, _ := makeHyperlink(link.href)

		style := ctx.options.Styles.Link
		if link.linkType == linkTypeImage {
			style = ctx.options.Styles.Image
		}

		linkMaxWidth := max(termWidth-xansi.StringWidth(linkText)-1, 0)
		token := hyperlink + xansi.Truncate(link.href, linkMaxWidth, "…") + resetHyperlink

		el := &BaseElement{Token: token, Style: style}
		_ = el.Render(w, ctx)
	}

	renderString := func(str string) {
		_, _ = renderText(w, ctx.blockStack.Current().Style.StylePrimitive, str)
	}

	paddingFor := func(total, position int) int {
		totalSize := len(strconv.Itoa(total))
		positionSize := len(strconv.Itoa(position))

		return max(totalSize-positionSize, 0)
	}

	renderList := func(list []tableLink) {
		for i, item := range list {
			position := i + 1
			padding := paddingFor(len(list), position)

			renderString("\n")
			linkText := renderLinkText(item, position, padding)
			renderString(" ")
			renderLinkHref(item, linkText)
		}
	}

	if len(ctx.table.tableLinks) > 0 {
		renderString("\n")
	}
	renderList(ctx.table.tableLinks)

	if len(ctx.table.tableImages) > 0 {
		renderString("\n")
	}
	renderList(ctx.table.tableImages)
}

func (e *TableElement) shouldPrintTableLinks(ctx RenderContext) bool {
	if ctx.options.InlineTableLinks {
		return false
	}
	if len(ctx.table.tableLinks) == 0 && len(ctx.table.tableImages) == 0 {
		return false
	}
	return true
}

func (e *TableElement) collectLinksAndImages(ctx RenderContext) error {
	images := make([]tableLink, 0)
	links := make([]tableLink, 0)

	err := ast.Walk(e.table, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := node.(type) {
		case *ast.AutoLink:
			uri := string(n.URL(e.source))
			autoLink := tableLink{
				href:     uri,
				content:  linkDomain(uri),
				linkType: linkTypeAuto,
			}
			if shortned, ok := autolink.Detect(uri); ok {
				autoLink.content = shortned
			}
			links = append(links, autoLink)
		case *ast.Image:
			content, err := nodeContent(node, e.source)
			if err != nil {
				return ast.WalkStop, err
			}
			image := tableLink{
				href:     string(n.Destination),
				title:    string(n.Title),
				content:  string(content),
				linkType: linkTypeImage,
			}
			if image.content == "" {
				image.content = linkDomain(image.href)
			}
			images = append(images, image)
		case *ast.Link:
			content, err := nodeContent(node, e.source)
			if err != nil {
				return ast.WalkStop, err
			}
			link := tableLink{
				href:     string(n.Destination),
				title:    string(n.Title),
				content:  string(content),
				linkType: linkTypeRegular,
			}
			links = append(links, link)
		}

		return ast.WalkContinue, nil
	})
	if err != nil {
		return fmt.Errorf("glamour: error collecting links: %w", err)
	}

	ctx.table.tableImages = slice.Uniq(images)
	ctx.table.tableLinks = slice.Uniq(links)
	return nil
}

func isInsideTable(node ast.Node) bool {
	parent := node.Parent()
	for parent != nil {
		switch parent.Kind() {
		case astext.KindTable, astext.KindTableHeader, astext.KindTableRow, astext.KindTableCell:
			return true
		default:
			parent = parent.Parent()
		}
	}
	return false
}

func nodeContent(node ast.Node, source []byte) ([]byte, error) {
	var builder bytes.Buffer

	var traverse func(node ast.Node) error
	traverse = func(node ast.Node) error {
		for n := node.FirstChild(); n != nil; n = n.NextSibling() {
			switch nn := n.(type) {
			case *ast.Text:
				if _, err := builder.Write(nn.Segment.Value(source)); err != nil {
					return fmt.Errorf("glamour: error writing text node: %w", err)
				}
			default:
				if err := traverse(nn); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := traverse(node); err != nil {
		return nil, err
	}

	return builder.Bytes(), nil
}

func linkDomain(href string) string {
	if uri, err := url.Parse(href); err == nil {
		return uri.Hostname()
	}
	return "link"
}

func linkWithSuffix(tl tableLink, list []tableLink) string {
	index := slices.Index(list, tl)
	if index == -1 {
		return tl.content
	}
	return fmt.Sprintf("%s[%d]", tl.content, index+1)
}
