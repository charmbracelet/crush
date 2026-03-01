package attachments

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/require"
)

// createTestRenderer creates a renderer with test styles.
func createTestRenderer() *Renderer {
	baseStyle := lipgloss.NewStyle()
	normalStyle := baseStyle.Padding(0, 1).MarginRight(1).Background(lipgloss.Color("#555")).Foreground(lipgloss.Color("#fff"))
	deletingStyle := baseStyle.Padding(0, 1).Bold(true).Background(lipgloss.Color("#f00")).Foreground(lipgloss.Color("#fff"))
	copySelectingStyle := baseStyle.Padding(0, 1).Bold(true).Background(lipgloss.Color("#00f")).Foreground(lipgloss.Color("#fff"))
	imageStyle := baseStyle.Foreground(lipgloss.Color("#0f0")).Background(lipgloss.Color("#555")).Padding(0, 1)
	textStyle := baseStyle.Foreground(lipgloss.Color("#ff0")).Background(lipgloss.Color("#555")).Padding(0, 1)

	return NewRenderer(normalStyle, deletingStyle, copySelectingStyle, imageStyle, textStyle)
}

// createTestAttachments creates test attachments.
func createTestAttachments(n int) []message.Attachment {
	attachments := make([]message.Attachment, n)
	for i := range n {
		mimeType := "text/plain"
		if i%2 == 1 {
			mimeType = "image/png"
		}
		attachments[i] = message.Attachment{
			FilePath: "/path/to/file",
			FileName: "file" + string(rune('0'+i)) + ".txt",
			MimeType: mimeType,
			Content:  []byte("content"),
		}
	}
	return attachments
}

func TestRenderer_Render_NormalMode(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	atts := createTestAttachments(3)

	result := renderer.Render(atts, false, 100)

	require.NotEmpty(t, result)
	// Should not contain indices in normal mode
	require.NotContains(t, result, "[0]")
	require.NotContains(t, result, "[1]")
	require.NotContains(t, result, "[2]")
}

func TestRenderer_Render_DeleteMode(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	atts := createTestAttachments(3)

	result := renderer.Render(atts, true, 100)

	require.NotEmpty(t, result)
	// In delete mode, indices should be visible
	require.Contains(t, result, "0")
	require.Contains(t, result, "1")
	require.Contains(t, result, "2")
}

func TestRenderer_RenderWithMode_CopySelecting(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	atts := createTestAttachments(3)

	result := renderer.RenderWithMode(atts, false, true, 100)

	require.NotEmpty(t, result)
	// In copy selecting mode, indices should be visible
	require.Contains(t, result, "0")
	require.Contains(t, result, "1")
	require.Contains(t, result, "2")
}

func TestRenderer_RenderWithMode_Normal(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	atts := createTestAttachments(3)

	result := renderer.RenderWithMode(atts, false, false, 100)

	require.NotEmpty(t, result)
	// Should not contain indices in normal mode
	// (checking that the index numbers aren't there as standalone tokens)
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		// The result should not have bare numbers that look like indices
		// This is a heuristic check
		trimmed := strings.TrimSpace(line)
		if trimmed == "0" || trimmed == "1" || trimmed == "2" {
			t.Errorf("Normal mode should not show indices, found: %s", trimmed)
		}
	}
}

func TestRenderer_RenderWithMode_DeleteAndCopy(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	atts := createTestAttachments(2)

	// When both deleting and copySelecting are true, deleting takes precedence
	result := renderer.RenderWithMode(atts, true, true, 100)

	require.NotEmpty(t, result)
	// Should still render with indices
	require.Contains(t, result, "0")
	require.Contains(t, result, "1")
}

func TestRenderer_Render_EmptyAttachments(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	var atts []message.Attachment

	result := renderer.Render(atts, false, 100)

	require.Empty(t, result)
}

func TestRenderer_RenderWithMode_EmptyAttachments(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	var atts []message.Attachment

	result := renderer.RenderWithMode(atts, false, true, 100)

	require.Empty(t, result)
}

func TestRenderer_Render_LongFilenameTruncation(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	atts := []message.Attachment{
		{
			FilePath: "/path/to/very-long-filename-that-should-be-truncated.txt",
			FileName: "very-long-filename-that-should-be-truncated.txt",
			MimeType: "text/plain",
		},
	}

	result := renderer.Render(atts, false, 100)

	require.NotEmpty(t, result)
	// The result should not contain the full long filename
	require.NotContains(t, result, "very-long-filename-that-should-be-truncated.txt")
}

func TestRenderer_Icon_TextAttachment(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	textAtt := message.Attachment{
		FilePath: "/path/to/file.txt",
		FileName: "file.txt",
		MimeType: "text/plain",
	}

	icon := renderer.icon(textAtt)

	// Should return text style (not image style)
	// We can't directly compare styles, but we can verify it's not nil
	require.NotNil(t, icon)
}

func TestRenderer_Icon_ImageAttachment(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	imageAtt := message.Attachment{
		FilePath: "/path/to/image.png",
		FileName: "image.png",
		MimeType: "image/png",
	}

	icon := renderer.icon(imageAtt)

	// Should return image style
	require.NotNil(t, icon)
}

func TestRenderer_Render_MoreAttachmentsIndicator(t *testing.T) {
	t.Parallel()

	renderer := createTestRenderer()
	// Create many attachments to trigger the "more..." indicator
	atts := createTestAttachments(10)

	// Render with a narrow width to force the "more..." indicator
	result := renderer.Render(atts, false, 50)

	require.NotEmpty(t, result)
	// Should contain "more" indicator
	require.Contains(t, strings.ToLower(result), "more")
}
