package message

import "strings"

type Attachment struct {
	FilePath string
	FileName string
	MimeType string
	Content  []byte
}

func (a Attachment) IsText() bool    { return strings.HasPrefix(a.MimeType, "text/") }
func (a Attachment) IsImage() bool   { return strings.HasPrefix(a.MimeType, "image/") }
func (a Attachment) IsMention() bool { return strings.HasPrefix(a.MimeType, "crush/mention") }
