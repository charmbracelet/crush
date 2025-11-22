package notification_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/log"
	"github.com/charmbracelet/crush/internal/notification"
	"github.com/stretchr/testify/require"
)

func TestSend_Disabled(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Setup a temporary config with DisableNotifications = true
	tempDir := t.TempDir()
	cfg, err := config.Init(tempDir, tempDir, false)
	require.NoError(t, err)

	// Explicitly disable notifications
	cfg.Options.DisableNotifications = true

	// Call Send
	// This should return nil immediately because notifications are disabled
	err = notification.Send("Test Title", "Test Message")
	require.NoError(t, err)
}

func TestSend_Focused(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Reset globals after test
	defer func() {
		notification.SetFocusSupport(false)
		notification.SetFocused(true)
	}()

	// Setup a temporary config with DisableNotifications = false
	tempDir := t.TempDir()
	cfg, err := config.Init(tempDir, tempDir, false)
	require.NoError(t, err)

	cfg.Options.DisableNotifications = false

	// Set up focus state so notification should be skipped
	notification.SetFocusSupport(true)
	notification.SetFocused(true)

	// Call Send
	// This should return nil immediately because window is focused
	err = notification.Send("Test Title", "Test Message")
	require.NoError(t, err)
}

func TestSend_Success(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Reset globals after test
	defer func() {
		notification.SetFocusSupport(false)
		notification.SetFocused(true)
		notification.ResetNotifyFunc()
	}()

	// Setup a temporary config with DisableNotifications = false
	tempDir := t.TempDir()
	cfg, err := config.Init(tempDir, tempDir, false)
	require.NoError(t, err)

	cfg.Options.DisableNotifications = false

	// Set up focus state so notification should NOT be skipped
	notification.SetFocusSupport(true)
	notification.SetFocused(false)

	// Mock the notify function
	var capturedTitle, capturedMessage string
	var capturedIcon any
	mockNotify := func(title, message string, icon any) error {
		capturedTitle = title
		capturedMessage = message
		capturedIcon = icon
		return nil
	}
	notification.SetNotifyFunc(mockNotify)

	// Call Send
	err = notification.Send("Hello", "World")
	require.NoError(t, err)

	// Verify mock was called with correct arguments
	require.Equal(t, "Hello", capturedTitle)
	require.Equal(t, "World", capturedMessage)
	require.NotNil(t, capturedIcon)
}

func TestSend_FocusNotSupported(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Reset globals after test
	defer func() {
		notification.SetFocusSupport(false)
		notification.SetFocused(true)
		notification.ResetNotifyFunc()
	}()

	// Setup a temporary config with DisableNotifications = false
	tempDir := t.TempDir()
	cfg, err := config.Init(tempDir, tempDir, false)
	require.NoError(t, err)

	cfg.Options.DisableNotifications = false

	// Focus support disabled, but "focused" is true (simulate default state where we assume focused but can't verify, or just focus tracking disabled)
	// The logic says: "Do NOT send if focus reporting is not supported."
	notification.SetFocusSupport(false)
	notification.SetFocused(true)

	// Mock the notify function
	called := false
	mockNotify := func(title, message string, icon any) error {
		called = true
		return nil
	}
	notification.SetNotifyFunc(mockNotify)

	// Call Send
	err = notification.Send("Title", "Message")
	require.NoError(t, err)
	require.False(t, called, "Should NOT send notification if focus support is disabled")
}

func TestSend_Error(t *testing.T) {
	// Pre-initialize logger to os.DevNull to prevent file lock on Windows.
	log.Setup(os.DevNull, false)

	// Reset globals after test
	defer func() {
		notification.SetFocusSupport(false)
		notification.SetFocused(true)
		notification.ResetNotifyFunc()
	}()

	// Setup a temporary config with DisableNotifications = false
	tempDir := t.TempDir()
	cfg, err := config.Init(tempDir, tempDir, false)
	require.NoError(t, err)

	cfg.Options.DisableNotifications = false

	// Ensure we try to send
	notification.SetFocusSupport(true)
	notification.SetFocused(false)

	// Mock error
	expectedErr := fmt.Errorf("mock error")
	mockNotify := func(title, message string, icon any) error {
		return expectedErr
	}
	notification.SetNotifyFunc(mockNotify)

	// Call Send
	err = notification.Send("Title", "Message")
	require.Equal(t, expectedErr, err)
}
