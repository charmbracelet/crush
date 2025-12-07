package editor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateTextClipboardContent(t *testing.T) {
	t.Parallel()

	content := CreateTextClipboardContent("Hello World")

	// Test wrapper functionality
	require.Equal(t, "text/plain", content.GetMimeType())
	require.Equal(t, []byte("Hello World"), content.GetData())
	require.Equal(t, int64(11), content.GetSize())
	require.NoError(t, content.Validate())
	require.Equal(t, "Hello World", content.String())
}

func TestCreateImageClipboardContent(t *testing.T) {
	t.Parallel()

	// Test with URL and data
	imageData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG header
	content := CreateImageClipboardContent("https://example.com/image.png", imageData, "png")

	require.Equal(t, "image/png", content.GetMimeType())
	require.Equal(t, imageData, content.GetData())
	require.Equal(t, int64(4), content.GetSize())
	require.NoError(t, content.Validate())
	require.Equal(t, "https://example.com/image.png", content.String())
}

func TestCreateFileClipboardContent(t *testing.T) {
	t.Parallel()

	fileData := []byte("file content")
	content := CreateFileClipboardContent("/path/to/file.txt", "file.txt", fileData, "text/plain")

	require.Equal(t, "text/plain", content.GetMimeType())
	require.Equal(t, fileData, content.GetData())
	require.Equal(t, int64(12), content.GetSize())
	require.NoError(t, content.Validate())
	// Note: String() will return empty since it's binary content
}

func TestCreateHTMLClipboardContent(t *testing.T) {
	t.Parallel()

	content := CreateHTMLClipboardContent("<p>Hello <strong>World</strong></p>", "Hello World")

	require.Equal(t, "text/html", content.GetMimeType())
	require.Equal(t, []byte("<p>Hello <strong>World</strong></p>"), content.GetData())
	require.Equal(t, int64(len("<p>Hello <strong>World</strong></p>")), content.GetSize())
	require.NoError(t, content.Validate())
	require.Equal(t, "Hello World", content.String()) // Should return fallback text
}

func TestCreateURIClipboardContent(t *testing.T) {
	t.Parallel()

	uris := []string{
		"https://example.com/file1.txt",
		"https://example.com/file2.txt",
	}
	content := CreateURIClipboardContent(uris)

	require.Equal(t, "text/uri-list", content.GetMimeType())
	expectedData := []byte("https://example.com/file1.txt\nhttps://example.com/file2.txt")
	require.Equal(t, expectedData, content.GetData())
	require.Equal(t, int64(len(expectedData)), content.GetSize())
	require.NoError(t, content.Validate())
	require.Equal(t, "https://example.com/file1.txt\nhttps://example.com/file2.txt", content.String())
}

func TestGetClipboardContentFromSelection(t *testing.T) {
	t.Parallel()

	selectedText := "Selected text for clipboard"
	content := GetClipboardContentFromSelection(selectedText)

	require.Equal(t, "text/plain", content.GetMimeType())
	require.Equal(t, []byte(selectedText), content.GetData())
	require.Equal(t, int64(len(selectedText)), content.GetSize())
	require.NoError(t, content.Validate())
	require.Equal(t, selectedText, content.String())
}

func TestGetClipboardContentForFile(t *testing.T) {
	t.Parallel()

	fileData := []byte("test file content")
	path := "/test/path/test.txt"
	filename := "test.txt"

	content, err := GetClipboardContentForFile(path, filename, fileData, "text/plain")
	require.NoError(t, err)

	require.Equal(t, "text/plain", content.GetMimeType())
	require.Equal(t, fileData, content.GetData())
	require.Equal(t, int64(len(fileData)), content.GetSize())
	require.NoError(t, content.Validate())
}

func TestCreateDataURI(t *testing.T) {
	t.Parallel()

	data := []byte("test data")
	mimeType := "text/plain"

	uri := CreateDataURI(mimeType, data)

	require.Equal(t, "data:text/plain;base64,dGVzdCBkYXRh", uri)
}

func TestClipboardContentValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		content ClipboardContentWrapper
		valid   bool
	}{
		{
			name:    "Valid Text Content",
			content: CreateTextClipboardContent("Hello"),
			valid:   true,
		},
		{
			name:    "Valid Image Content",
			content: CreateImageClipboardContent("https://example.com/image.png", []byte{0x89}, "png"),
			valid:   true,
		},
		{
			name:    "Valid File Content",
			content: CreateFileClipboardContent("/path/file.txt", "file.txt", []byte("content"), "text/plain"),
			valid:   true,
		},
		{
			name:    "Valid HTML Content",
			content: CreateHTMLClipboardContent("<p>test</p>", "test"),
			valid:   true,
		},
		{
			name:    "Valid URI Content",
			content: CreateURIClipboardContent([]string{"https://example.com"}),
			valid:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.content.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
