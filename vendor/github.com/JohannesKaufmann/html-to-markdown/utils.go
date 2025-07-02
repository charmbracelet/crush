package md

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

/*
WARNING: The functions from this file can be used externally
but there is no garanty that they will stay exported.
*/

// CollectText returns the text of the node and all its children
func CollectText(n *html.Node) string {
	text := &bytes.Buffer{}
	collectText(n, text)
	return text.String()
}
func collectText(n *html.Node, buf *bytes.Buffer) {
	if n.Type == html.TextNode {
		buf.WriteString(n.Data)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectText(c, buf)
	}
}

func getName(node *html.Node) string {
	selec := &goquery.Selection{Nodes: []*html.Node{node}}
	return goquery.NodeName(selec)
}

// What elements automatically trim their content?
// Don't add another space if the other element is going to add a
// space already.
func isTrimmedElement(name string) bool {
	nodes := []string{
		"a",
		"strong", "b",
		"i", "em",
		"del", "s", "strike",
		"code",
	}

	for _, node := range nodes {
		if name == node {
			return true
		}
	}
	return false
}

func getPrevNodeText(node *html.Node) (string, bool) {
	if node == nil {
		return "", false
	}

	for ; node != nil; node = node.PrevSibling {
		text := CollectText(node)

		name := getName(node)
		if name == "br" {
			return "\n", true
		}

		// if the content is empty, try our luck with the next node
		if strings.TrimSpace(text) == "" {
			continue
		}

		if isTrimmedElement(name) {
			text = strings.TrimSpace(text)
		}

		return text, true
	}
	return "", false
}
func getNextNodeText(node *html.Node) (string, bool) {
	if node == nil {
		return "", false
	}

	for ; node != nil; node = node.NextSibling {
		text := CollectText(node)

		name := getName(node)
		if name == "br" {
			return "\n", true
		}

		// if the content is empty, try our luck with the next node
		if strings.TrimSpace(text) == "" {
			continue
		}

		// if you have "a a a", three elements that are trimmed, then only add
		// a space to one side, since the other's are also adding a space.
		if isTrimmedElement(name) {
			text = " "
		}

		return text, true
	}
	return "", false
}

// AddSpaceIfNessesary adds spaces to the text based on the neighbors.
// That makes sure that there is always a space to the side, to recognize the delimiter.
func AddSpaceIfNessesary(selec *goquery.Selection, markdown string) string {
	if len(selec.Nodes) == 0 {
		return markdown
	}
	rootNode := selec.Nodes[0]

	prev, hasPrev := getPrevNodeText(rootNode.PrevSibling)
	if hasPrev {
		lastChar, size := utf8.DecodeLastRuneInString(prev)
		if size > 0 && !unicode.IsSpace(lastChar) {
			markdown = " " + markdown
		}
	}

	next, hasNext := getNextNodeText(rootNode.NextSibling)
	if hasNext {
		firstChar, size := utf8.DecodeRuneInString(next)
		if size > 0 && !unicode.IsSpace(firstChar) && !unicode.IsPunct(firstChar) {
			markdown = markdown + " "
		}
	}

	return markdown
}

func isLineCodeDelimiter(chars []rune) bool {
	if len(chars) < 3 {
		return false
	}

	// TODO: If it starts with 4 (instead of 3) fence characters, we should only end it
	// if we see the same amount of ending fence characters.
	return chars[0] == '`' && chars[1] == '`' && chars[2] == '`'
}

// TrimpLeadingSpaces removes spaces from the beginning of a line
// but makes sure that list items and code blocks are not affected.
func TrimpLeadingSpaces(text string) string {
	var insideCodeBlock bool

	lines := strings.Split(text, "\n")
	for index := range lines {
		chars := []rune(lines[index])

		if isLineCodeDelimiter(chars) {
			if !insideCodeBlock {
				// start the code block
				insideCodeBlock = true
			} else {
				// end the code block
				insideCodeBlock = false
			}
		}
		if insideCodeBlock {
			// We are inside a code block and don't want to
			// disturb that formatting (e.g. python indentation)
			continue
		}

		var spaces int
		for i := 0; i < len(chars); i++ {
			if unicode.IsSpace(chars[i]) {
				if chars[i] == '	' {
					spaces = spaces + 4
				} else {
					spaces++
				}
				continue
			}

			// this seems to be a list item
			if chars[i] == '-' {
				break
			}

			// this seems to be a code block
			if spaces >= 4 {
				break
			}

			// remove the space characters from the string
			chars = chars[i:]
			break
		}
		lines[index] = string(chars)
	}

	return strings.Join(lines, "\n")
}

// TrimTrailingSpaces removes unnecessary spaces from the end of lines.
func TrimTrailingSpaces(text string) string {
	parts := strings.Split(text, "\n")
	for i := range parts {
		parts[i] = strings.TrimRightFunc(parts[i], func(r rune) bool {
			return unicode.IsSpace(r)
		})

	}

	return strings.Join(parts, "\n")
}

// The same as `multipleNewLinesRegex`, but applies to escaped new lines inside a link `\n\`
var multipleNewLinesInLinkRegex = regexp.MustCompile(`(\n\\){1,}`) // `([\n\r\s]\\)`

// EscapeMultiLine deals with multiline content inside a link
func EscapeMultiLine(content string) string {
	content = strings.TrimSpace(content)
	content = strings.Replace(content, "\n", `\`+"\n", -1)

	content = multipleNewLinesInLinkRegex.ReplaceAllString(content, "\n\\")

	return content
}

func calculateCodeFenceOccurrences(fenceChar rune, content string) int {
	var occurrences []int

	var charsTogether int
	for _, char := range content {
		// we encountered a fence character, now count how many
		// are directly afterwards
		if char == fenceChar {
			charsTogether++
		} else if charsTogether != 0 {
			occurrences = append(occurrences, charsTogether)
			charsTogether = 0
		}
	}

	// if the last element in the content was a fenceChar
	if charsTogether != 0 {
		occurrences = append(occurrences, charsTogether)
	}

	return findMax(occurrences)
}

// CalculateCodeFence can be passed the content of a code block and it returns
// how many fence characters (` or ~) should be used.
//
// This is useful if the html content includes the same fence characters
// for example ```
// -> https://stackoverflow.com/a/49268657
func CalculateCodeFence(fenceChar rune, content string) string {
	repeat := calculateCodeFenceOccurrences(fenceChar, content)

	// the outer fence block always has to have
	// at least one character more than any content inside
	repeat++

	// you have to have at least three fence characters
	// to be recognized as a code block
	if repeat < 3 {
		repeat = 3
	}

	return strings.Repeat(string(fenceChar), repeat)
}

func findMax(a []int) (max int) {
	for i, value := range a {
		if i == 0 {
			max = a[i]
		}

		if value > max {
			max = value
		}
	}
	return max
}

func getCodeWithoutTags(startNode *html.Node) []byte {
	var buf bytes.Buffer

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && (n.Data == "style" || n.Data == "script" || n.Data == "textarea") {
			return
		}
		if n.Type == html.ElementNode && (n.Data == "br" || n.Data == "div") {
			buf.WriteString("\n")
		}

		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}

	f(startNode)

	return buf.Bytes()
}

// getCodeContent gets the content of pre/code and unescapes the encoded characters.
// Returns "" if there is an error.
func getCodeContent(selec *goquery.Selection) string {
	if len(selec.Nodes) == 0 {
		return ""
	}

	code := getCodeWithoutTags(selec.Nodes[0])

	return string(code)
}

// delimiterForEveryLine puts the delimiter not just at the start and end of the string
// but if the text is divided on multiple lines, puts the delimiters on every line with content.
//
// Otherwise the bold/italic delimiters won't be recognized if it contains new line characters.
func delimiterForEveryLine(text string, delimiter string) string {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// Skip empty lines
			continue
		}

		lines[i] = delimiter + line + delimiter
	}
	return strings.Join(lines, "\n")
}

// isWrapperListItem returns wether the list item has own
// content or is just a wrapper for another list.
// e.g. "<li><ul>..."
func isWrapperListItem(s *goquery.Selection) bool {
	directText := s.Contents().Not("ul").Not("ol").Text()

	noOwnText := strings.TrimSpace(directText) == ""
	childIsList := s.ChildrenFiltered("ul").Length() > 0 || s.ChildrenFiltered("ol").Length() > 0

	return noOwnText && childIsList
}

// getListStart returns the integer from which the counting
// for for the list items should start from.
// -> https://developer.mozilla.org/en-US/docs/Web/HTML/Element/ol#start
func getListStart(parent *goquery.Selection) int {
	val := parent.AttrOr("start", "")
	if val == "" {
		return 1
	}

	num, err := strconv.Atoi(val)
	if err != nil {
		return 1
	}

	if num < 0 {
		return 1
	}
	return num
}

// getListPrefix returns the appropriate prefix for the list item.
// For example "- ", "* ", "1. ", "01. ", ...
func getListPrefix(opt *Options, s *goquery.Selection) string {
	if isWrapperListItem(s) {
		return ""
	}

	parent := s.Parent()
	if parent.Is("ul") {
		return opt.BulletListMarker + " "
	} else if parent.Is("ol") {
		start := getListStart(parent)
		currentIndex := start + s.Index()

		lastIndex := parent.Children().Last().Index() + 1
		maxLength := len(strconv.Itoa(lastIndex))

		// pad the numbers so that all prefix numbers in the list take up the same space
		// `%02d.` -> "01. "
		format := `%0` + strconv.Itoa(maxLength) + `d. `
		return fmt.Sprintf(format, currentIndex)
	}
	// If the HTML is malformed and the list element isn't in a ul or ol, return no prefix
	return ""
}

// countListParents counts how much space is reserved for the prefixes at all the parent lists.
// This is useful to calculate the correct level of indentation for nested lists.
func countListParents(opt *Options, selec *goquery.Selection) (int, int) {
	var values []int
	for n := selec.Parent(); n != nil; n = n.Parent() {
		if n.Is("li") {
			continue
		}
		if !n.Is("ul") && !n.Is("ol") {
			break
		}

		prefix := n.Children().First().AttrOr(attrListPrefix, "")

		values = append(values, len(prefix))
	}

	// how many spaces are reserved for the prefixes of my siblings
	var prefixCount int

	// how many spaces are reserved in total for all of the other
	// list parents up the tree
	var previousPrefixCounts int

	for i, val := range values {
		if i == 0 {
			prefixCount = val
			continue
		}

		previousPrefixCounts += val
	}

	return prefixCount, previousPrefixCounts
}

// IndentMultiLineListItem makes sure that multiline list items
// are properly indented.
func IndentMultiLineListItem(opt *Options, text string, spaces int) string {
	parts := strings.Split(text, "\n")
	for i := range parts {
		// dont touch the first line since its indented through the prefix
		if i == 0 {
			continue
		}

		if isListItem(opt, parts[i]) {
			return strings.Join(parts, "\n")
		}

		indent := strings.Repeat(" ", spaces)
		parts[i] = indent + parts[i]
	}

	return strings.Join(parts, "\n")
}

// isListItem checks wether the line is a markdown list item
func isListItem(opt *Options, line string) bool {
	b := []rune(line)

	bulletMarker := []rune(opt.BulletListMarker)[0]

	var hasNumber bool
	var hasMarker bool
	var hasSpace bool

	for i := 0; i < len(b); i++ {
		// A marker followed by a space qualifies as a list item
		if hasMarker && hasSpace {
			if b[i] == bulletMarker {
				// But if another BulletListMarker is found, it
				// might be a HorizontalRule
				return false
			}

			if !unicode.IsSpace(b[i]) {
				// Now we have some text
				return true
			}
		}

		if hasMarker {
			if unicode.IsSpace(b[i]) {
				hasSpace = true
				continue
			}
			// A marker like "1." that is not immediately followed by a space
			// is probably a false positive
			return false
		}

		if b[i] == bulletMarker {
			hasMarker = true
			continue
		}

		if hasNumber && b[i] == '.' {
			hasMarker = true
			continue
		}
		if unicode.IsDigit(b[i]) {
			hasNumber = true
			continue
		}

		if unicode.IsSpace(b[i]) {
			continue
		}

		// If we encouter any other character
		// before finding an indicator, its
		// not a list item
		return false
	}
	return false
}

// IndexWithText is similar to goquery's Index function but
// returns the index of the current element while
// NOT counting the empty elements beforehand.
func IndexWithText(s *goquery.Selection) int {
	return s.PrevAll().FilterFunction(func(i int, s *goquery.Selection) bool {
		return strings.TrimSpace(s.Text()) != ""
	}).Length()
}
