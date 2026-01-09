package message

import (
	"slices"
	"strings"
)

type Attachment struct {
	FilePath string
	FileName string
	MimeType string
	Content  []byte
}

func (a Attachment) IsText() bool  { return strings.HasPrefix(a.MimeType, "text/") }
func (a Attachment) IsImage() bool { return strings.HasPrefix(a.MimeType, "image/") }

// ContainsTextAttachment returns true if any of the attachments is a text attachments.
func ContainsTextAttachment(attachments []Attachment) bool {
	return slices.ContainsFunc(attachments, func(a Attachment) bool {
		return a.IsText()
	})
}
