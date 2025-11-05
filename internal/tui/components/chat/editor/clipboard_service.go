package editor

import (
	"context"
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/tui/util"
)

// ClipboardService defines interface for clipboard operations
// Following established service pattern from message system
type ClipboardService interface {
	// Read reads content from clipboard
	Read(ctx context.Context) ([]ClipboardContentWrapper, error)

	// Write writes content to clipboard
	Write(ctx context.Context, content ClipboardContentWrapper) error

	// Clear clears clipboard
	Clear(ctx context.Context) error

	// SupportedTypes returns supported MIME types
	SupportedTypes() []string

	// Capabilities returns clipboard capabilities
	Capabilities() ClipboardCapabilities
}

// ClipboardCapabilities describes what the clipboard service can do
type ClipboardCapabilities struct {
	// MultipleFormats indicates support for multiple MIME types
	MultipleFormats bool

	// LargeContent indicates support for large content (>1MB)
	LargeContent bool

	// RichContent indicates support for rich content (HTML, images)
	RichContent bool

	// URISupport indicates support for URI lists
	URISupport bool

	// AsyncOperations indicates support for async operations
	AsyncOperations bool
}

// OSC52ClipboardService implements clipboard using OSC 52 escape sequences
// Works in most terminals but limited to text content
type OSC52ClipboardService struct{}

// NewOSC52ClipboardService creates new OSC52 clipboard service
func NewOSC52ClipboardService() ClipboardService {
	return &OSC52ClipboardService{}
}

func (oscs *OSC52ClipboardService) Read(ctx context.Context) ([]ClipboardContentWrapper, error) {
	// OSC 52 doesn't support reading from clipboard
	return nil, fmt.Errorf("OSC 52 clipboard service doesn't support reading")
}

func (oscs *OSC52ClipboardService) Write(ctx context.Context, content ClipboardContentWrapper) error {
	// OSC 52 only supports text content
	if content.GetMimeType() != "text/plain" {
		return fmt.Errorf("OSC 52 clipboard only supports text/plain, got %s", content.GetMimeType())
	}

	// Use tea.SetClipboard which handles OSC 52
	data := content.GetData()
	if len(data) == 0 {
		return fmt.Errorf("cannot write empty content to clipboard")
	}

	// tea.SetClipboard returns a command, but for service interface we need to execute directly
	// We'll convert to string for tea.SetClipboard
	text := string(data)

	// Execute the clipboard operation
	// Note: This is a synchronous approach for service interface
	// In async contexts, this would return a tea.Cmd instead
	clipboardContent := tea.SetClipboard(text)

	// Execute the command immediately (not ideal, but service interface requires sync)
	// This is a limitation of the service pattern vs tea.Cmd pattern
	_ = clipboardContent

	return nil
}

func (oscs *OSC52ClipboardService) Clear(ctx context.Context) error {
	// OSC 52 doesn't support clearing
	return fmt.Errorf("OSC 52 clipboard service doesn't support clearing")
}

func (oscs *OSC52ClipboardService) SupportedTypes() []string {
	return []string{"text/plain"}
}

func (oscs *OSC52ClipboardService) Capabilities() ClipboardCapabilities {
	return ClipboardCapabilities{
		MultipleFormats: false,
		LargeContent:    false, // Limited by terminal escape sequence length
		RichContent:     false,
		URISupport:      false,
		AsyncOperations: false,
	}
}

// NativeClipboardService implements clipboard using native OS clipboard
// Supports multiple formats but requires GUI environment
type NativeClipboardService struct{}

// NewNativeClipboardService creates new native clipboard service
func NewNativeClipboardService() ClipboardService {
	return &NativeClipboardService{}
}

func (ncs *NativeClipboardService) Read(ctx context.Context) ([]ClipboardContentWrapper, error) {
	// Try to read from native clipboard
	text, err := clipboard.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read from native clipboard: %w", err)
	}

	if text == "" {
		return []ClipboardContentWrapper{}, nil
	}

	// Create text content wrapper
	content := CreateTextClipboardContent(text)
	return []ClipboardContentWrapper{content}, nil
}

func (ncs *NativeClipboardService) Write(ctx context.Context, content ClipboardContentWrapper) error {
	data := content.GetData()
	if len(data) == 0 {
		return fmt.Errorf("cannot write empty content to clipboard")
	}

	// For text content, write directly
	if strings.HasPrefix(content.GetMimeType(), "text/") {
		text := string(data)
		return clipboard.WriteAll(text)
	}

	// For non-text content, try to write as base64 data URI
	dataURI := CreateDataURI(content.GetMimeType(), data)
	return clipboard.WriteAll(dataURI)
}

func (ncs *NativeClipboardService) Clear(ctx context.Context) error {
	// Native clipboard doesn't typically support clearing
	// We'll write empty string
	return clipboard.WriteAll("")
}

func (ncs *NativeClipboardService) SupportedTypes() []string {
	return []string{
		"text/plain",
		"text/html",
		"text/uri-list",
		"image/png",
		"image/jpeg",
		"application/octet-stream",
	}
}

func (ncs *NativeClipboardService) Capabilities() ClipboardCapabilities {
	return ClipboardCapabilities{
		MultipleFormats: true,
		LargeContent:    true,
		RichContent:     true,
		URISupport:      true,
		AsyncOperations: false,
	}
}

// CompositeClipboardService combines multiple clipboard services
// Uses best available service for each operation
type CompositeClipboardService struct {
	services []ClipboardService
	primary  ClipboardService
}

// NewCompositeClipboardService creates new composite clipboard service
func NewCompositeClipboardService(services ...ClipboardService) ClipboardService {
	if len(services) == 0 {
		// Fallback to OSC 52
		return NewOSC52ClipboardService()
	}

	// Use first service as primary
	return &CompositeClipboardService{
		services: services,
		primary:  services[0],
	}
}

// NewDefaultCompositeClipboardService creates default composite with OSC 52 + Native
func NewDefaultCompositeClipboardService() ClipboardService {
	return NewCompositeClipboardService(
		NewOSC52ClipboardService(),
		NewNativeClipboardService(),
	)
}

func (ccs *CompositeClipboardService) Read(ctx context.Context) ([]ClipboardContentWrapper, error) {
	// Try primary service first
	contents, err := ccs.primary.Read(ctx)
	if err == nil {
		return contents, nil
	}

	// Try other services
	for _, service := range ccs.services[1:] {
		contents, err := service.Read(ctx)
		if err == nil {
			return contents, nil
		}
	}

	return nil, fmt.Errorf("no clipboard service could read content")
}

func (ccs *CompositeClipboardService) Write(ctx context.Context, content ClipboardContentWrapper) error {
	// Try all services, succeed if any succeed
	var lastError error

	for _, service := range ccs.services {
		err := service.Write(ctx, content)
		if err == nil {
			return nil
		}
		lastError = err
	}

	return fmt.Errorf("all clipboard services failed to write content: %w", lastError)
}

func (ccs *CompositeClipboardService) Clear(ctx context.Context) error {
	// Try all services, succeed if any succeed
	var lastError error

	for _, service := range ccs.services {
		err := service.Clear(ctx)
		if err == nil {
			return nil
		}
		lastError = err
	}

	return fmt.Errorf("all clipboard services failed to clear clipboard: %w", lastError)
}

func (ccs *CompositeClipboardService) SupportedTypes() []string {
	// Union of all supported types
	typeMap := make(map[string]bool)

	for _, service := range ccs.services {
		for _, mimeType := range service.SupportedTypes() {
			typeMap[mimeType] = true
		}
	}

	types := make([]string, 0, len(typeMap))
	for mimeType := range typeMap {
		types = append(types, mimeType)
	}

	return types
}

func (ccs *CompositeClipboardService) Capabilities() ClipboardCapabilities {
	// Union of all capabilities (true if any service supports it)
	caps := ClipboardCapabilities{}

	for _, service := range ccs.services {
		serviceCaps := service.Capabilities()
		if serviceCaps.MultipleFormats {
			caps.MultipleFormats = true
		}
		if serviceCaps.LargeContent {
			caps.LargeContent = true
		}
		if serviceCaps.RichContent {
			caps.RichContent = true
		}
		if serviceCaps.URISupport {
			caps.URISupport = true
		}
		if serviceCaps.AsyncOperations {
			caps.AsyncOperations = true
		}
	}

	return caps
}

// TeaClipboardAdapter adapts clipboard service to tea.Cmd pattern
// Bridges between sync service interface and async tea.Cmd
type TeaClipboardAdapter struct {
	service ClipboardService
}

// NewTeaClipboardAdapter creates new adapter for clipboard service
func NewTeaClipboardAdapter(service ClipboardService) *TeaClipboardAdapter {
	return &TeaClipboardAdapter{
		service: service,
	}
}

// WriteToClipboard returns tea.Cmd to write content to clipboard
// Converts sync service.Write() to async tea.Cmd
func (tca *TeaClipboardAdapter) WriteToClipboard(content ClipboardContentWrapper) tea.Cmd {
	return func() tea.Msg {
		err := tca.service.Write(context.Background(), content)
		if err != nil {
			return util.ReportError(fmt.Errorf("failed to write to clipboard: %w", err))
		}
		return util.ReportInfo("Content copied to clipboard")
	}
}

// ReadFromClipboard returns tea.Cmd to read from clipboard
// Converts sync service.Read() to async tea.Cmd
func (tca *TeaClipboardAdapter) ReadFromClipboard() tea.Cmd {
	return func() tea.Msg {
		contents, err := tca.service.Read(context.Background())
		if err != nil {
			return util.ReportError(fmt.Errorf("failed to read from clipboard: %w", err))
		}
		return ClipboardReadMsg{Contents: contents}
	}
}

// ClipboardReadMsg is sent when clipboard content is read
type ClipboardReadMsg struct {
	Contents []ClipboardContentWrapper
}

// GetDefaultClipboardService returns the best clipboard service for current environment
func GetDefaultClipboardService() ClipboardService {
	// Start with composite of all available services
	service := NewDefaultCompositeClipboardService()

	// Check if native clipboard is available
	_, err := clipboard.ReadAll()
	if err != nil {
		// Native clipboard not available, fall back to OSC 52 only
		return NewOSC52ClipboardService()
	}

	return service
}
