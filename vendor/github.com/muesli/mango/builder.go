package mango

import (
	"time"
)

// Builder is the interface of a man page builder.
type Builder interface {
	Heading(section uint, title, description string, ts time.Time)
	Paragraph()
	Indent(n int)
	IndentEnd()
	TaggedParagraph(indentation int)
	List(text string)
	Section(text string)
	EndSection()
	Text(text string)
	TextBold(text string)
	TextItalic(text string)
	String() string
}
