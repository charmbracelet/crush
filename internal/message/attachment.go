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

// textMimePrefixes are MIME type prefixes that should be treated as text.
var textMimePrefixes = []string{
	"text/",
	"application/json",
	"application/xml",
	"application/yaml",
	"application/x-yaml",
	"application/javascript",
	"application/typescript",
	"application/x-sh",
	"application/x-shellscript",
	"application/toml",
	"application/x-toml",
	"application/sql",
	"application/x-sql",
	"application/graphql",
	"application/x-graphql",
}

// IsText reports whether the attachment should be treated as text and
// inlined into the prompt. This includes standard text/* MIME types as
// well as common application/* types that are fundamentally text (JSON,
// YAML, XML, etc.).
func (a Attachment) IsText() bool {
	for _, prefix := range textMimePrefixes {
		if strings.HasPrefix(a.MimeType, prefix) {
			return true
		}
	}
	return false
}

func (a Attachment) IsImage() bool    { return strings.HasPrefix(a.MimeType, "image/") }
func (a Attachment) IsMarkdown() bool { return a.MimeType == "text/markdown" }

// ContainsTextAttachment returns true if any of the attachments is a text attachment.
func ContainsTextAttachment(attachments []Attachment) bool {
	return slices.ContainsFunc(attachments, func(a Attachment) bool {
		return a.IsText()
	})
}
