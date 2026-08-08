package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttachment_IsText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mimeType string
		want     bool
	}{
		// Standard text types.
		{name: "text/plain", mimeType: "text/plain", want: true},
		{name: "text/html", mimeType: "text/html", want: true},
		{name: "text/css", mimeType: "text/css", want: true},
		{name: "text/markdown", mimeType: "text/markdown", want: true},
		{name: "text/csv", mimeType: "text/csv", want: true},
		{name: "text/javascript", mimeType: "text/javascript", want: true},
		{name: "text/x-shellscript", mimeType: "text/x-shellscript", want: true},
		{name: "text with charset", mimeType: "text/plain; charset=utf-8", want: true},

		// Application types that are fundamentally text.
		{name: "application/json", mimeType: "application/json", want: true},
		{name: "application/xml", mimeType: "application/xml", want: true},
		{name: "application/yaml", mimeType: "application/yaml", want: true},
		{name: "application/x-yaml", mimeType: "application/x-yaml", want: true},
		{name: "application/javascript", mimeType: "application/javascript", want: true},
		{name: "application/typescript", mimeType: "application/typescript", want: true},
		{name: "application/x-sh", mimeType: "application/x-sh", want: true},
		{name: "application/x-shellscript", mimeType: "application/x-shellscript", want: true},
		{name: "application/toml", mimeType: "application/toml", want: true},
		{name: "application/x-toml", mimeType: "application/x-toml", want: true},
		{name: "application/sql", mimeType: "application/sql", want: true},
		{name: "application/x-sql", mimeType: "application/x-sql", want: true},
		{name: "application/graphql", mimeType: "application/graphql", want: true},
		{name: "application/x-graphql", mimeType: "application/x-graphql", want: true},

		// Non-text types.
		{name: "image/png", mimeType: "image/png", want: false},
		{name: "image/jpeg", mimeType: "image/jpeg", want: false},
		{name: "application/pdf", mimeType: "application/pdf", want: false},
		{name: "application/zip", mimeType: "application/zip", want: false},
		{name: "application/octet-stream", mimeType: "application/octet-stream", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := Attachment{MimeType: tt.mimeType}
			require.Equal(t, tt.want, a.IsText())
		})
	}
}

func TestAttachment_IsImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mimeType string
		want     bool
	}{
		{name: "image/png", mimeType: "image/png", want: true},
		{name: "image/jpeg", mimeType: "image/jpeg", want: true},
		{name: "image/gif", mimeType: "image/gif", want: true},
		{name: "text/plain", mimeType: "text/plain", want: false},
		{name: "application/json", mimeType: "application/json", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := Attachment{MimeType: tt.mimeType}
			require.Equal(t, tt.want, a.IsImage())
		})
	}
}

func TestAttachment_IsMarkdown(t *testing.T) {
	t.Parallel()

	require.True(t, Attachment{MimeType: "text/markdown"}.IsMarkdown())
	require.False(t, Attachment{MimeType: "text/plain"}.IsMarkdown())
}

func TestContainsTextAttachment(t *testing.T) {
	t.Parallel()

	t.Run("no attachments", func(t *testing.T) {
		t.Parallel()
		require.False(t, ContainsTextAttachment(nil))
	})

	t.Run("only image attachments", func(t *testing.T) {
		t.Parallel()
		attachments := []Attachment{
			{MimeType: "image/png"},
			{MimeType: "image/jpeg"},
		}
		require.False(t, ContainsTextAttachment(attachments))
	})

	t.Run("mixed attachments", func(t *testing.T) {
		t.Parallel()
		attachments := []Attachment{
			{MimeType: "image/png"},
			{MimeType: "application/json"},
		}
		require.True(t, ContainsTextAttachment(attachments))
	})

	t.Run("only text attachments", func(t *testing.T) {
		t.Parallel()
		attachments := []Attachment{
			{MimeType: "text/plain"},
			{MimeType: "application/yaml"},
		}
		require.True(t, ContainsTextAttachment(attachments))
	})
}
