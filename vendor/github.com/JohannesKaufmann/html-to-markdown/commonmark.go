package md

import (
	"fmt"
	"unicode"

	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/escape"
	"github.com/PuerkitoBio/goquery"
)

var multipleSpacesR = regexp.MustCompile(`  +`)

var commonmark = []Rule{
	{
		Filter: []string{"ul", "ol"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			parent := selec.Parent()

			// we have a nested list, were the ul/ol is inside a list item
			// -> based on work done by @requilence from @anytypeio
			if (parent.Is("li") || parent.Is("ul") || parent.Is("ol")) && parent.Children().Last().IsSelection(selec) {
				// add a line break prefix if the parent's text node doesn't have it.
				// that makes sure that every list item is on its on line
				lastContentTextNode := strings.TrimRight(parent.Nodes[0].FirstChild.Data, " \t")
				if !strings.HasSuffix(lastContentTextNode, "\n") {
					content = "\n" + content
				}

				// remove empty lines between lists
				trimmedSpaceContent := strings.TrimRight(content, " \t")
				if strings.HasSuffix(trimmedSpaceContent, "\n") {
					content = strings.TrimRightFunc(content, unicode.IsSpace)
				}
			} else {
				content = "\n\n" + content + "\n\n"
			}
			return &content
		},
	},
	{
		Filter: []string{"li"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			if strings.TrimSpace(content) == "" {
				return nil
			}

			// remove leading newlines
			content = leadingNewlinesR.ReplaceAllString(content, "")
			// replace trailing newlines with just a single one
			content = trailingNewlinesR.ReplaceAllString(content, "\n")
			// remove leading spaces
			content = strings.TrimLeft(content, " ")

			prefix := selec.AttrOr(attrListPrefix, "")

			// `prefixCount` is not nessesarily the length of the empty string `prefix`
			// but how much space is reserved for the prefixes of the siblings.
			prefixCount, previousPrefixCounts := countListParents(opt, selec)

			// if the prefix is not needed, balance it by adding the usual prefix spaces
			if prefix == "" {
				prefix = strings.Repeat(" ", prefixCount)
			}
			// indent the prefix so that the nested links are represented
			indent := strings.Repeat(" ", previousPrefixCounts)
			prefix = indent + prefix

			content = IndentMultiLineListItem(opt, content, prefixCount+previousPrefixCounts)

			return String(prefix + content + "\n")
		},
	},
	{
		Filter: []string{"#text"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			text := selec.Text()
			if trimmed := strings.TrimSpace(text); trimmed == "" {
				return String("")
			}
			text = tabR.ReplaceAllString(text, " ")

			// replace multiple spaces by one space: dont accidentally make
			// normal text be indented and thus be a code block.
			text = multipleSpacesR.ReplaceAllString(text, " ")

			if opt.EscapeMode == "basic" {
				text = escape.MarkdownCharacters(text)
			}

			// if its inside a list, trim the spaces to not mess up the indentation
			parent := selec.Parent()
			next := selec.Next()
			if IndexWithText(selec) == 0 &&
				(parent.Is("li") || parent.Is("ol") || parent.Is("ul")) &&
				(next.Is("ul") || next.Is("ol")) {
				// trim only spaces and not new lines
				text = strings.Trim(text, ` `)
			}

			return &text
		},
	},
	{
		Filter: []string{"p", "div"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			parent := goquery.NodeName(selec.Parent())
			if IsInlineElement(parent) || parent == "li" {
				content = "\n" + content + "\n"
				return &content
			}

			// remove unnecessary spaces to have clean markdown
			content = TrimpLeadingSpaces(content)

			content = "\n\n" + content + "\n\n"
			return &content
		},
	},
	{
		Filter: []string{"h1", "h2", "h3", "h4", "h5", "h6"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			if strings.TrimSpace(content) == "" {
				return nil
			}

			content = strings.Replace(content, "\n", " ", -1)
			content = strings.Replace(content, "\r", " ", -1)
			content = strings.Replace(content, `#`, `\#`, -1)
			content = strings.TrimSpace(content)

			insideLink := selec.ParentsFiltered("a").Length() > 0
			if insideLink {
				text := opt.StrongDelimiter + content + opt.StrongDelimiter
				text = AddSpaceIfNessesary(selec, text)
				return &text
			}

			node := goquery.NodeName(selec)
			level, err := strconv.Atoi(node[1:])
			if err != nil {
				return nil
			}

			if opt.HeadingStyle == "setext" && level < 3 {
				line := "-"
				if level == 1 {
					line = "="
				}

				underline := strings.Repeat(line, len(content))
				return String("\n\n" + content + "\n" + underline + "\n\n")
			}

			prefix := strings.Repeat("#", level)
			text := "\n\n" + prefix + " " + content + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"strong", "b"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			// only use one bold tag if they are nested
			parent := selec.Parent()
			if parent.Is("strong") || parent.Is("b") {
				return &content
			}

			trimmed := strings.TrimSpace(content)
			if trimmed == "" {
				return &trimmed
			}

			// If there is a newline character between the start and end delimiter
			// the delimiters won't be recognized. Either we remove all newline characters
			// OR on _every_ line we put start & end delimiters.
			trimmed = delimiterForEveryLine(trimmed, opt.StrongDelimiter)

			// Always have a space to the side to recognize the delimiter
			trimmed = AddSpaceIfNessesary(selec, trimmed)

			return &trimmed
		},
	},
	{
		Filter: []string{"i", "em"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			// only use one italic tag if they are nested
			parent := selec.Parent()
			if parent.Is("i") || parent.Is("em") {
				return &content
			}

			trimmed := strings.TrimSpace(content)
			if trimmed == "" {
				return &trimmed
			}

			// If there is a newline character between the start and end delimiter
			// the delimiters won't be recognized. Either we remove all newline characters
			// OR on _every_ line we put start & end delimiters.
			trimmed = delimiterForEveryLine(trimmed, opt.EmDelimiter)

			// Always have a space to the side to recognize the delimiter
			trimmed = AddSpaceIfNessesary(selec, trimmed)

			return &trimmed
		},
	},
	{
		Filter: []string{"img"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			src := selec.AttrOr("src", "")
			src = strings.TrimSpace(src)
			if src == "" {
				return String("")
			}

			src = opt.GetAbsoluteURL(selec, src, opt.domain)

			alt := selec.AttrOr("alt", "")
			alt = strings.Replace(alt, "\n", " ", -1)

			text := fmt.Sprintf("![%s](%s)", alt, src)
			return &text
		},
	},
	{
		Filter: []string{"a"},
		AdvancedReplacement: func(content string, selec *goquery.Selection, opt *Options) (AdvancedResult, bool) {
			// if there is no href, no link is used. So just return the content inside the link
			href, ok := selec.Attr("href")
			if !ok || strings.TrimSpace(href) == "" || strings.TrimSpace(href) == "#" {
				return AdvancedResult{
					Markdown: content,
				}, false
			}

			href = opt.GetAbsoluteURL(selec, href, opt.domain)

			// having multiline content inside a link is a bit tricky
			content = EscapeMultiLine(content)

			var title string
			if t, ok := selec.Attr("title"); ok {
				t = strings.Replace(t, "\n", " ", -1)
				// escape all quotes
				t = strings.Replace(t, `"`, `\"`, -1)
				title = fmt.Sprintf(` "%s"`, t)
			}

			// if there is no link content (for example because it contains an svg)
			// the 'title' or 'aria-label' attribute is used instead.
			if strings.TrimSpace(content) == "" {
				content = selec.AttrOr("title", selec.AttrOr("aria-label", ""))
			}

			// a link without text won't de displayed anyway
			if content == "" {
				return AdvancedResult{}, true
			}

			if opt.LinkStyle == "inlined" {
				md := fmt.Sprintf("[%s](%s%s)", content, href, title)
				md = AddSpaceIfNessesary(selec, md)

				return AdvancedResult{
					Markdown: md,
				}, false
			}

			var replacement string
			var reference string

			switch opt.LinkReferenceStyle {
			case "collapsed":

				replacement = "[" + content + "][]"
				reference = "[" + content + "]: " + href + title
			case "shortcut":
				replacement = "[" + content + "]"
				reference = "[" + content + "]: " + href + title

			default:
				id := selec.AttrOr("data-index", "")
				replacement = "[" + content + "][" + id + "]"
				reference = "[" + id + "]: " + href + title
			}

			replacement = AddSpaceIfNessesary(selec, replacement)
			return AdvancedResult{Markdown: replacement, Footer: reference}, false
		},
	},
	{
		Filter: []string{"code", "kbd", "samp", "tt"},
		Replacement: func(_ string, selec *goquery.Selection, opt *Options) *string {
			code := getCodeContent(selec)

			// Newlines in the text aren't great, since this is inline code and not a code block.
			// Newlines will be stripped anyway in the browser, but it won't be recognized as code
			// from the markdown parser when there is more than one newline.
			// So limit to
			code = multipleNewLinesRegex.ReplaceAllString(code, "\n")

			fenceChar := '`'
			maxCount := calculateCodeFenceOccurrences(fenceChar, code)
			maxCount++

			fence := strings.Repeat(string(fenceChar), maxCount)

			// code block contains a backtick as first character
			if strings.HasPrefix(code, "`") {
				code = " " + code
			}
			// code block contains a backtick as last character
			if strings.HasSuffix(code, "`") {
				code = code + " "
			}

			// TODO: configure delimeter in options?
			text := fence + code + fence
			text = AddSpaceIfNessesary(selec, text)
			return &text
		},
	},
	{
		Filter: []string{"pre"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			codeElement := selec.Find("code")
			language := codeElement.AttrOr("class", "")
			language = strings.Replace(language, "language-", "", 1)

			code := getCodeContent(selec)

			fenceChar, _ := utf8.DecodeRuneInString(opt.Fence)
			fence := CalculateCodeFence(fenceChar, code)

			text := "\n\n" + fence + language + "\n" +
				code +
				"\n" + fence + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"hr"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			// e.g. `## --- Heading` would look weird, so don't render a divider if inside a heading
			insideHeading := selec.ParentsFiltered("h1,h2,h3,h4,h5,h6").Length() > 0
			if insideHeading {
				return String("")
			}

			text := "\n\n" + opt.HorizontalRule + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"br"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			return String("\n\n")
		},
	},
	{
		Filter: []string{"blockquote"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			content = strings.TrimSpace(content)
			if content == "" {
				return nil
			}

			content = multipleNewLinesRegex.ReplaceAllString(content, "\n\n")

			var beginningR = regexp.MustCompile(`(?m)^`)
			content = beginningR.ReplaceAllString(content, "> ")

			text := "\n\n" + content + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"noscript"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			// for now remove the contents of noscript. But in the future we could
			// tell goquery to parse the contents of the tag.
			// -> https://github.com/PuerkitoBio/goquery/issues/139#issuecomment-517526070
			return nil
		},
	},
}
