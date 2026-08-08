package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsAllowedAttachmentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		// Image types.
		{name: "png", path: "photo.png", want: true},
		{name: "jpg", path: "photo.jpg", want: true},
		{name: "jpeg", path: "photo.jpeg", want: true},
		{name: "PNG uppercase", path: "PHOTO.PNG", want: true},

		// Text file types.
		{name: "json", path: "config.json", want: true},
		{name: "yaml", path: "docker-compose.yaml", want: true},
		{name: "yml", path: "docker-compose.yml", want: true},
		{name: "md", path: "README.md", want: true},
		{name: "txt", path: "notes.txt", want: true},
		{name: "css", path: "style.css", want: true},
		{name: "html", path: "index.html", want: true},
		{name: "py", path: "main.py", want: true},
		{name: "go", path: "main.go", want: true},
		{name: "ts", path: "app.ts", want: true},
		{name: "sql", path: "migration.sql", want: true},
		{name: "toml", path: "Cargo.toml", want: true},
		{name: "csv", path: "data.csv", want: true},
		{name: "xml", path: "pom.xml", want: true},
		{name: "env", path: ".env", want: true},
		{name: "conf", path: "nginx.conf", want: true},
		{name: "tf", path: "main.tf", want: true},
		{name: "log", path: "app.log", want: true},

		// Case insensitivity.
		{name: "JSON uppercase", path: "CONFIG.JSON", want: true},
		{name: "MD mixed case", path: "ReadMe.Md", want: true},

		// Full paths.
		{name: "full path", path: "/home/user/project/config.json", want: true},
		{name: "relative path", path: "../src/main.go", want: true},

		// Unsupported types.
		{name: "pdf", path: "doc.pdf", want: false},
		{name: "zip", path: "archive.zip", want: false},
		{name: "docx", path: "report.docx", want: false},
		{name: "exe", path: "binary.exe", want: false},
		{name: "no extension", path: "Makefile", want: false},
		{name: "mp4", path: "video.mp4", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, IsAllowedAttachmentType(tt.path))
		})
	}
}

func TestIsAllowedImageType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "png", path: "photo.png", want: true},
		{name: "jpg", path: "photo.jpg", want: true},
		{name: "jpeg", path: "photo.jpeg", want: true},
		{name: "PNG uppercase", path: "PHOTO.PNG", want: true},
		{name: "json", path: "config.json", want: false},
		{name: "pdf", path: "doc.pdf", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, IsAllowedImageType(tt.path))
		})
	}
}

func TestAllAllowedAttachmentTypes(t *testing.T) {
	t.Parallel()

	types := AllAllowedAttachmentTypes()

	// Should include all image types.
	for _, ext := range AllowedImageTypes {
		require.Contains(t, types, ext)
	}

	// Should include all text file types.
	for _, ext := range AllowedTextFileTypes {
		require.Contains(t, types, ext)
	}

	// Should have combined length.
	require.Len(t, types, len(AllowedImageTypes)+len(AllowedTextFileTypes))
}
