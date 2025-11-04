package editor

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/catwalk/pkg/catwalk"
)

// ClipboardContentWrapper wraps existing ContentPart with clipboard functionality
// This approach leverages existing message system without interface conflicts
type ClipboardContentWrapper struct {
	Content      message.ContentPart
	MimeType     string
	RawData      []byte
	OriginalSize int64
}

// MimeType returns the MIME type for clipboard operations
func (ccw ClipboardContentWrapper) GetMimeType() string {
	return ccw.MimeType
}

// Data returns raw data for clipboard operations
func (ccw ClipboardContentWrapper) GetData() []byte {
	if len(ccw.RawData) > 0 {
		return ccw.RawData
	}
	// Use type assertion to get string content
	if text, ok := ccw.Content.(message.TextContent); ok {
		return []byte(text.String())
	}
	return []byte{}
}

// Size returns content size in bytes
func (ccw ClipboardContentWrapper) GetSize() int64 {
	if ccw.OriginalSize > 0 {
		return ccw.OriginalSize
	}
	return int64(len(ccw.GetData()))
}

// Validate checks if content is valid for clipboard operations
func (ccw ClipboardContentWrapper) Validate() error {
	if ccw.Content == nil {
		return errors.New("content cannot be nil")
	}
	if ccw.MimeType == "" {
		return errors.New("MIME type cannot be empty")
	}
	return nil
}

// String returns string representation
func (ccw ClipboardContentWrapper) String() string {
	// Use type assertion to get string content
	if text, ok := ccw.Content.(message.TextContent); ok {
		return text.String()
	}
	if image, ok := ccw.Content.(message.ImageURLContent); ok {
		return image.String()
	}
	// Fallback for other types
	return ""
}

// CreateTextClipboardContent creates clipboard wrapper for text content
func CreateTextClipboardContent(text string) ClipboardContentWrapper {
	content := message.TextContent{Text: text}
	return ClipboardContentWrapper{
		Content:      content,
		MimeType:     "text/plain",
		RawData:      []byte(text),
		OriginalSize: int64(len(text)),
	}
}

// CreateImageClipboardContent creates clipboard wrapper for image content
func CreateImageClipboardContent(url string, data []byte, format string) ClipboardContentWrapper {
	content := message.ImageURLContent{URL: url}
	mimeType := "image/" + format
	if format == "" {
		mimeType = "image/png"
	}
	
	return ClipboardContentWrapper{
		Content:      content,
		MimeType:     mimeType,
		RawData:      data,
		OriginalSize: int64(len(data)),
	}
}

// CreateFileClipboardContent creates clipboard wrapper for file content
func CreateFileClipboardContent(path string, filename string, data []byte, mimeType string) ClipboardContentWrapper {
	content := message.BinaryContent{
		Path:     path,
		MIMEType: mimeType,
		Data:     data,
	}
	
	if mimeType == "" {
		if strings.HasSuffix(strings.ToLower(filename), ".txt") {
			mimeType = "text/plain"
		} else {
			mimeType = "application/octet-stream"
		}
	}
	
	return ClipboardContentWrapper{
		Content:      content,
		MimeType:     mimeType,
		RawData:      data,
		OriginalSize: int64(len(data)),
	}
}

// CreateHTMLClipboardContent creates clipboard wrapper for HTML content
func CreateHTMLClipboardContent(html string, fallbackText string) ClipboardContentWrapper {
	content := message.TextContent{Text: fallbackText} // Use fallback text for string representation
	
	return ClipboardContentWrapper{
		Content:      content,
		MimeType:     "text/html",
		RawData:      []byte(html),
		OriginalSize: int64(len(html)),
	}
}

// CreateURIClipboardContent creates clipboard wrapper for URI list content
func CreateURIClipboardContent(uris []string) ClipboardContentWrapper {
	// Create a text content representation
	textRepresentation := strings.Join(uris, "\n")
	content := message.TextContent{Text: textRepresentation}
	
	uriListData := []byte(strings.Join(uris, "\n"))
	
	return ClipboardContentWrapper{
		Content:      content,
		MimeType:     "text/uri-list",
		RawData:      uriListData,
		OriginalSize: int64(len(uriListData)),
	}
}

// GetClipboardContentFromSelection converts selected text to clipboard wrapper
func GetClipboardContentFromSelection(selectedText string) ClipboardContentWrapper {
	return CreateTextClipboardContent(selectedText)
}

// GetClipboardContentForFile converts file to appropriate clipboard wrapper
func GetClipboardContentForFile(path string, filename string, data []byte, mimeType string) (ClipboardContentWrapper, error) {
	if data == nil {
		return ClipboardContentWrapper{}, errors.New("file data cannot be nil")
	}
	
	// Determine MIME type if not provided
	if mimeType == "" {
		if strings.HasSuffix(strings.ToLower(filename), ".txt") {
			mimeType = "text/plain"
		} else {
			mimeType = "application/octet-stream"
		}
	}
	
	// For text files, create text content
	if strings.HasPrefix(mimeType, "text/") {
		return CreateTextClipboardContent(string(data)), nil
	}
	
	// For other files, create file content
	return CreateFileClipboardContent(path, filename, data, mimeType), nil
}

// CreateDataURI creates data URI for file content (following BinaryContent pattern)
func CreateDataURI(mimeType string, data []byte) string {
	base64Encoded := base64.StdEncoding.EncodeToString(data)
	return "data:" + mimeType + ";base64," + base64Encoded
}

// CreateDataURIForProvider creates data URI respecting provider-specific format
func CreateDataURIForProvider(mimeType string, data []byte, provider catwalk.InferenceProvider) string {
	if provider == catwalk.InferenceProviderOpenAI {
		return CreateDataURI(mimeType, data)
	}
	return base64.StdEncoding.EncodeToString(data)
}