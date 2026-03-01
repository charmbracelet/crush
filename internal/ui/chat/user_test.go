package chat

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

// makeKeyMsg creates a tea.KeyPressMsg for testing.
func makeKeyMsg(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{
		Code: code,
		Text: text,
	})
}

// makeSpecialKeyMsg creates a tea.KeyPressMsg for special keys (enter, esc, etc).
func makeSpecialKeyMsg(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{
		Code: code,
		Text: "",
	})
}

// createTestStyles creates a styles object for testing.
func createTestStyles() *styles.Styles {
	s := styles.DefaultStyles()
	return &s
}

// createTestAttachmentRenderer creates an attachment renderer for testing.
func createTestAttachmentRenderer() *attachments.Renderer {
	sty := createTestStyles()
	return attachments.NewRenderer(
		sty.Attachments.Normal,
		sty.Attachments.Deleting,
		sty.Attachments.CopySelecting,
		sty.Attachments.Image,
		sty.Attachments.Text,
	)
}

// createTestMessage creates a test message with optional binary content.
func createTestMessage(id, text string, binaryContent ...message.BinaryContent) *message.Message {
	msg := &message.Message{
		ID: id,
		Parts: []message.ContentPart{
			message.TextContent{Text: text},
		},
	}
	for _, bc := range binaryContent {
		msg.Parts = append(msg.Parts, bc)
	}
	return msg
}

// createTextAttachment creates a text attachment for testing.
func createTextAttachment(path string, data []byte) message.BinaryContent {
	return message.BinaryContent{
		Path:     path,
		MIMEType: "text/plain",
		Data:     data,
	}
}

// createImageAttachment creates an image attachment for testing.
func createImageAttachment(path string) message.BinaryContent {
	return message.BinaryContent{
		Path:     path,
		MIMEType: "image/png",
		Data:     []byte("fake-image-data"),
	}
}

func TestUserMessageItem_HandleKeyEvent_NoAttachments(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage("msg-1", "Hello, world!")
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Press 'c' - should copy message text directly
	keyMsg := makeKeyMsg('c', "c")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	require.Equal(t, copyModeInactive, item.copyMode, "Copy mode should remain inactive")
}

func TestUserMessageItem_HandleKeyEvent_SingleTextAttachment(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	attachmentContent := []byte("Attachment content here")
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file.txt", attachmentContent),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Press 'c' - should copy attachment content directly (shortcut for single text attachment)
	keyMsg := makeKeyMsg('c', "c")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	// Copy mode should remain inactive since we copied directly
	require.Equal(t, copyModeInactive, item.copyMode, "Copy mode should remain inactive for single text attachment shortcut")
}

func TestUserMessageItem_HandleKeyEvent_MultipleAttachments_EntersCopyMode(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
		createTextAttachment("file2.txt", []byte("Content 2")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Press 'c' - should enter copy selection mode
	keyMsg := makeKeyMsg('c', "c")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	require.Equal(t, copyModeSelecting, item.copyMode, "Should enter copy selection mode")
	require.False(t, item.copyModeTimer.IsZero(), "Copy mode timer should be set")
}

func TestUserMessageItem_HandleKeyEvent_CopyMode_CopyAttachmentByIndex(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
		createTextAttachment("file2.txt", []byte("Content 2")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode
	item.copyMode = copyModeSelecting
	item.copyModeTimer = time.Now()

	// Press '1' - should copy attachment at index 1
	keyMsg := makeKeyMsg('1', "1")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	require.Equal(t, copyModeInactive, item.copyMode, "Copy mode should be deactivated after copying")
}

func TestUserMessageItem_HandleKeyEvent_CopyMode_CopyMessageText(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode
	item.copyMode = copyModeSelecting
	item.copyModeTimer = time.Now()

	// Press 'enter' - should copy message text
	keyMsg := makeSpecialKeyMsg(tea.KeyEnter)
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	require.Equal(t, copyModeInactive, item.copyMode, "Copy mode should be deactivated after copying")
}

func TestUserMessageItem_HandleKeyEvent_CopyMode_CopyAllTextAttachments(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
		createTextAttachment("file2.txt", []byte("Content 2")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode
	item.copyMode = copyModeSelecting
	item.copyModeTimer = time.Now()

	// Press 'a' - should copy all text attachment contents
	keyMsg := makeKeyMsg('a', "a")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	require.Equal(t, copyModeInactive, item.copyMode, "Copy mode should be deactivated after copying")
}

func TestUserMessageItem_HandleKeyEvent_CopyMode_Cancel(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode
	item.copyMode = copyModeSelecting
	item.copyModeTimer = time.Now()

	// Press 'esc' - should cancel copy mode
	keyMsg := makeSpecialKeyMsg(tea.KeyEscape)
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.Nil(t, cmd, "Expected no command for cancel")
	require.Equal(t, copyModeInactive, item.copyMode, "Copy mode should be deactivated")
}

func TestUserMessageItem_HandleKeyEvent_CopyMode_InvalidKey(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode
	item.copyMode = copyModeSelecting
	originalTime := time.Now()
	item.copyModeTimer = originalTime

	// Press 'z' (invalid key) - should stay in copy mode and refresh timer
	time.Sleep(10 * time.Millisecond) // Ensure time passes
	keyMsg := makeKeyMsg('z', "z")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected info command for invalid key")
	require.Equal(t, copyModeSelecting, item.copyMode, "Should remain in copy selection mode")
	require.True(t, item.copyModeTimer.After(originalTime), "Copy mode timer should be refreshed")
}

func TestUserMessageItem_HandleKeyEvent_CopyMode_Timeout(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	// Need multiple attachments to ensure copy mode is re-entered after timeout
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
		createTextAttachment("file2.txt", []byte("Content 2")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode with an expired timer
	item.copyMode = copyModeSelecting
	item.copyModeTimer = time.Now().Add(-10 * time.Second) // Expired 10 seconds ago

	// Press 'c' - should be treated as new copy initiation since mode timed out
	keyMsg := makeKeyMsg('c', "c")
	handled, _ := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	// After timeout, copy mode should be re-entered (due to multiple attachments)
	require.Equal(t, copyModeSelecting, item.copyMode, "Should re-enter copy selection mode")
}

func TestUserMessageItem_HandleKeyEvent_BinaryAttachment_CopiesPath(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createImageAttachment("image.png"),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode
	item.copyMode = copyModeSelecting
	item.copyModeTimer = time.Now()

	// Press '0' - should copy attachment path (since it's binary)
	keyMsg := makeKeyMsg('0', "0")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	require.Equal(t, copyModeInactive, item.copyMode, "Copy mode should be deactivated")
}

func TestUserMessageItem_HandleKeyEvent_IndexOutOfRange(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Enter copy mode
	item.copyMode = copyModeSelecting
	item.copyModeTimer = time.Now()

	// Press '5' - index out of range, should stay in copy mode
	keyMsg := makeKeyMsg('5', "5")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected info command for invalid index")
	require.Equal(t, copyModeSelecting, item.copyMode, "Should remain in copy selection mode")
}

func TestUserMessageItem_CopyMode_UsesYKey(t *testing.T) {
	t.Parallel()

	renderer := createTestAttachmentRenderer()
	msg := createTestMessage(
		"msg-1",
		"Message text",
		createTextAttachment("file1.txt", []byte("Content 1")),
		createTextAttachment("file2.txt", []byte("Content 2")),
	)
	item := NewUserMessageItem(createTestStyles(), msg, renderer).(*UserMessageItem)

	// Press 'y' - should also enter copy selection mode
	keyMsg := makeKeyMsg('y', "y")
	handled, cmd := item.HandleKeyEvent(keyMsg)

	require.True(t, handled, "Expected key event to be handled")
	require.NotNil(t, cmd, "Expected a command to be returned")
	require.Equal(t, copyModeSelecting, item.copyMode, "Should enter copy selection mode with 'y' key")
}
