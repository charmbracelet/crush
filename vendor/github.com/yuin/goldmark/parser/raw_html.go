package parser

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type rawHTMLParser struct {
}

var defaultRawHTMLParser = &rawHTMLParser{}

// NewRawHTMLParser return a new InlineParser that can parse
// inline htmls.
func NewRawHTMLParser() InlineParser {
	return defaultRawHTMLParser
}

func (s *rawHTMLParser) Trigger() []byte {
	return []byte{'<'}
}

func (s *rawHTMLParser) Parse(parent ast.Node, block text.Reader, pc Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) > 1 && util.IsAlphaNumeric(line[1]) {
		return s.parseMultiLineRegexp(openTagRegexp, block, pc)
	}
	if len(line) > 2 && line[1] == '/' && util.IsAlphaNumeric(line[2]) {
		return s.parseMultiLineRegexp(closeTagRegexp, block, pc)
	}
	if bytes.HasPrefix(line, openComment) {
		return s.parseComment(block, pc)
	}
	if bytes.HasPrefix(line, openProcessingInstruction) {
		return s.parseUntil(block, closeProcessingInstruction, pc)
	}
	if len(line) > 2 && line[1] == '!' && line[2] >= 'A' && line[2] <= 'Z' {
		return s.parseUntil(block, closeDecl, pc)
	}
	if bytes.HasPrefix(line, openCDATA) {
		return s.parseUntil(block, closeCDATA, pc)
	}
	return nil
}

var tagnamePattern = `([A-Za-z][A-Za-z0-9-]*)`
var spaceOrOneNewline = `(?:[ \t]|(?:\r\n|\n){0,1})`
var attributePattern = `(?:[\r\n \t]+[a-zA-Z_:][a-zA-Z0-9:._-]*(?:[\r\n \t]*=[\r\n \t]*(?:[^\"'=<>` + "`" + `\x00-\x20]+|'[^']*'|"[^"]*"))?)` //nolint:golint,lll
var openTagRegexp = regexp.MustCompile("^<" + tagnamePattern + attributePattern + `*` + spaceOrOneNewline + `*/?>`)
var closeTagRegexp = regexp.MustCompile("^</" + tagnamePattern + spaceOrOneNewline + `*>`)

var openProcessingInstruction = []byte("<?")
var closeProcessingInstruction = []byte("?>")
var openCDATA = []byte("<![CDATA[")
var closeCDATA = []byte("]]>")
var closeDecl = []byte(">")
var emptyComment1 = []byte("<!-->")
var emptyComment2 = []byte("<!--->")
var openComment = []byte("<!--")
var closeComment = []byte("-->")

func (s *rawHTMLParser) parseComment(block text.Reader, pc Context) ast.Node {
	savedLine, savedSegment := block.Position()
	node := ast.NewRawHTML()
	line, segment := block.PeekLine()
	if bytes.HasPrefix(line, emptyComment1) {
		node.Segments.Append(segment.WithStop(segment.Start + len(emptyComment1)))
		block.Advance(len(emptyComment1))
		return node
	}
	if bytes.HasPrefix(line, emptyComment2) {
		node.Segments.Append(segment.WithStop(segment.Start + len(emptyComment2)))
		block.Advance(len(emptyComment2))
		return node
	}
	offset := len(openComment)
	line = line[offset:]
	for {
		index := bytes.Index(line, closeComment)
		if index > -1 {
			node.Segments.Append(segment.WithStop(segment.Start + offset + index + len(closeComment)))
			block.Advance(offset + index + len(closeComment))
			return node
		}
		offset = 0
		node.Segments.Append(segment)
		block.AdvanceLine()
		line, segment = block.PeekLine()
		if line == nil {
			break
		}
	}
	block.SetPosition(savedLine, savedSegment)
	return nil
}

func (s *rawHTMLParser) parseUntil(block text.Reader, closer []byte, pc Context) ast.Node {
	savedLine, savedSegment := block.Position()
	node := ast.NewRawHTML()
	for {
		line, segment := block.PeekLine()
		if line == nil {
			break
		}
		index := bytes.Index(line, closer)
		if index > -1 {
			node.Segments.Append(segment.WithStop(segment.Start + index + len(closer)))
			block.Advance(index + len(closer))
			return node
		}
		node.Segments.Append(segment)
		block.AdvanceLine()
	}
	block.SetPosition(savedLine, savedSegment)
	return nil
}

func (s *rawHTMLParser) parseMultiLineRegexp(reg *regexp.Regexp, block text.Reader, pc Context) ast.Node {
	sline, ssegment := block.Position()
	if block.Match(reg) {
		node := ast.NewRawHTML()
		eline, esegment := block.Position()
		block.SetPosition(sline, ssegment)
		for {
			line, segment := block.PeekLine()
			if line == nil {
				break
			}
			l, _ := block.Position()
			start := segment.Start
			if l == sline {
				start = ssegment.Start
			}
			end := segment.Stop
			if l == eline {
				end = esegment.Start
			}

			node.Segments.Append(text.NewSegment(start, end))
			if l == eline {
				block.Advance(end - start)
				break
			}
			block.AdvanceLine()
		}
		return node
	}
	return nil
}
