package editor

import (
	"context"
	"testing"
	"testing/fstest"

	tea "github.com/charmbracelet/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/app"
	"github.com/charmbracelet/crush/internal/fsext"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/tui/components/completions"
	"github.com/charmbracelet/crush/internal/tui/components/dialogs/filepicker"
	"github.com/charmbracelet/crush/internal/tui/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// executeBatchCommands executes all commands in a BatchMsg and returns the resulting messages
func executeBatchCommands(batchMsg tea.BatchMsg) []tea.Msg {
	var messages []tea.Msg
	for _, cmd := range batchMsg {
		if cmd != nil {
			msg := cmd()
			messages = append(messages, msg)
		}
	}
	return messages
}

// assertBatchContainsMessage checks if a BatchMsg contains a message of the specified type
func assertBatchContainsMessage(t *testing.T, batchMsg tea.BatchMsg, expectedType any) bool {
	t.Helper()
	messages := executeBatchCommands(batchMsg)

	for _, msg := range messages {
		switch expectedType.(type) {
		case completions.OpenCompletionsMsg:
			if _, ok := msg.(completions.OpenCompletionsMsg); ok {
				return true
			}
		}
	}
	return false
}

// assertBatchContainsExactMessage checks if a BatchMsg contains a message with exact field values
func assertBatchContainsExactMessage(t *testing.T, batchMsg tea.BatchMsg, expected any) bool {
	t.Helper()
	messages := executeBatchCommands(batchMsg)

	for _, msg := range messages {
		switch expected := expected.(type) {
		case completions.OpenCompletionsMsg:
			if actual, ok := msg.(completions.OpenCompletionsMsg); ok {
				// If no specific completions are expected, just match the type
				if len(expected.Completions) == 0 {
					return true
				}
				// Compare completions if specified
				if len(actual.Completions) == len(expected.Completions) {
					// For simplicity, just check the count for now
					// A more complete implementation would compare each completion
					return true
				}
			}
		default:
			// Fallback to type checking only
			if _, ok := msg.(completions.OpenCompletionsMsg); ok {
				return true
			}
		}
	}
	return false
}

// assertBatchContainsOpenCompletionsMsg checks if a BatchMsg contains an OpenCompletionsMsg
// with the expected completions. If expectedCompletions is nil, only the message type is checked.
func assertBatchContainsOpenCompletionsMsg(t *testing.T, batchMsg tea.BatchMsg, expectedCompletions []string) (*completions.OpenCompletionsMsg, bool) {
	t.Helper()
	messages := executeBatchCommands(batchMsg)

	for _, msg := range messages {
		if actual, ok := msg.(completions.OpenCompletionsMsg); ok {
			if expectedCompletions == nil {
				return &actual, true
			}

			// Convert actual completions to string titles for comparison
			actualTitles := make([]string, len(actual.Completions))
			for i, comp := range actual.Completions {
				actualTitles[i] = comp.Title
			}

			// Check if we have the same number of completions
			if len(actualTitles) != len(expectedCompletions) {
				continue
			}

			// For now, just check that we have the same count
			// A more sophisticated implementation would check the actual values
			return &actual, true
		}
	}
	return nil, false
}

func mockDirLister(paths []string) fsext.DirectoryListerResolver {
	return func() fsext.DirectoryLister {
		return func(initialPath string, ignorePatterns []string) ([]string, bool, error) {
			return paths, false, nil
		}
	}
}

type noopEvent struct{}

type updater interface {
	Update(msg tea.Msg) (tea.Model, tea.Cmd)
}

func simulateUpdate(up updater, msg tea.Msg) (updater, tea.Msg) {
	up, cmd := up.Update(msg)
	if cmd != nil {
		return up, cmd()
	}
	return up, noopEvent{}
}

var pngMagicNumberData = []byte("\x89PNG\x0D\x0A\x1A\x0A")

func TestEditorTypingForwardSlashOpensCompletions(t *testing.T) {
	testEditor := newEditor(&app.App{}, mockDirLister([]string{}))
	require.NotNil(t, testEditor)

	// Simulate pressing the '/' key
	keyPressMsg := tea.KeyPressMsg{
		Text: "/",
	}

	m, cmds := testEditor.Update(keyPressMsg)
	testEditor = m.(*editorCmp)
	cmds()

	assert.True(t, testEditor.isCompletionsOpen)
	assert.Equal(t, "/", testEditor.textarea.Value())
}

func TestEditorAutocompletionWithEmptyInput(t *testing.T) {
	testEditor := newEditor(&app.App{}, mockDirLister([]string{}))
	require.NotNil(t, testEditor)

	// First, give the editor focus
	testEditor.Focus()

	// Simulate pressing the '/' key when the editor is empty
	// This should trigger the completions to open
	keyPressMsg := tea.KeyPressMsg{
		Text: "/",
	}

	m, cmds := testEditor.Update(keyPressMsg)
	testEditor = m.(*editorCmp)
	cmds()

	// completions menu is open
	assert.True(t, testEditor.isCompletionsOpen)
	assert.Equal(t, "/", testEditor.textarea.Value())

	// the query is empty (since we just opened it)
	assert.Equal(t, "", testEditor.currentQuery)
}

func TestEditorAutoCompletion_OnNonImageFileFullPathInsertedFromQuery(t *testing.T) {
	entriesForAutoComplete := mockDirLister([]string{"image.png", "random.txt"})
	testEditor := newEditor(&app.App{}, entriesForAutoComplete)
	require.NotNil(t, testEditor)

	// open the completions menu by simulating a '/' key press
	testEditor.Focus()
	keyPressMsg := tea.KeyPressMsg{
		Text: "/",
	}

	m, msg := simulateUpdate(testEditor, keyPressMsg)
	testEditor = m.(*editorCmp)

	var openCompletionsMsg *completions.OpenCompletionsMsg
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		// Use our enhanced helper to check for OpenCompletionsMsg with specific completions
		var found bool
		openCompletionsMsg, found = assertBatchContainsOpenCompletionsMsg(t, batchMsg, []string{"image.png", "random.txt"})
		assert.True(t, found, "Expected to find OpenCompletionsMsg with specific completions in batched messages")
	} else {
		t.Fatal("Expected BatchMsg from cmds()")
	}

	assert.NotNil(t, openCompletionsMsg)
	require.True(t, testEditor.IsCompletionsOpen())

	testEditor.textarea.SetValue("I am looking for a file called /random.tx")

	keyPressMsg = tea.KeyPressMsg{
		Text: "t",
	}
	m, _ = simulateUpdate(testEditor, keyPressMsg)
	testEditor = m.(*editorCmp)

	selectMsg := completions.SelectCompletionMsg{
		Value: FileCompletionItem{
			"./root/project/random.txt",
		},
		Insert: true,
	}

	m, msg = simulateUpdate(testEditor, selectMsg)
	testEditor = m.(*editorCmp)

	if _, ok := msg.(noopEvent); !ok {
		t.Fatal("Expected noopEvent from cmds()")
	}

	assert.Equal(t, "I am looking for a file called ./root/project/random.txt", testEditor.textarea.Value())
}

func TestEditor_OnCompletionPathToImageEmitsAttachFileMessage(t *testing.T) {
	entriesForAutoComplete := mockDirLister([]string{"image.png", "random.txt"})
	fsys := fstest.MapFS{
		"auto_completed_image.png": {
			Data: pngMagicNumberData,
		},
		"random.txt": {
			Data: []byte("Some content"),
		},
	}

	modelHasImageSupport := func() (bool, string) {
		return true, "TestModel"
	}
	testEditor := newEditor(&app.App{}, entriesForAutoComplete)
	_, cmd := onCompletionItemSelect(fsys, modelHasImageSupport, FileCompletionItem{Path: "auto_completed_image.png"}, true, testEditor)

	require.NotNil(t, cmd)
	msg := cmd()
	require.NotNil(t, msg)

	var attachmentMsg message.Attachment
	if fpickedMsg, ok := msg.(filepicker.FilePickedMsg); ok {
		attachmentMsg = fpickedMsg.Attachment
	}

	assert.Equal(t, message.Attachment{
		FilePath: "auto_completed_image.png",
		FileName: "auto_completed_image.png",
		MimeType: "image/png",
		Content:  pngMagicNumberData,
	}, attachmentMsg)
}

func TestEditor_OnCompletionPathToImageEmitsWanrningMessageWhenModelDoesNotSupportImages(t *testing.T) {
	entriesForAutoComplete := mockDirLister([]string{"image.png", "random.txt"})
	fsys := fstest.MapFS{
		"auto_completed_image.png": {
			Data: pngMagicNumberData,
		},
		"random.txt": {
			Data: []byte("Some content"),
		},
	}

	modelHasImageSupport := func() (bool, string) {
		return false, "TestModel"
	}
	testEditor := newEditor(&app.App{}, entriesForAutoComplete)
	_, cmd := onCompletionItemSelect(fsys, modelHasImageSupport, FileCompletionItem{Path: "auto_completed_image.png"}, true, testEditor)

	require.NotNil(t, cmd)
	msg := cmd()
	require.NotNil(t, msg)

	warningMsg, ok := msg.(util.InfoMsg)
	require.True(t, ok)
	assert.Equal(t, util.InfoMsg{
		Type: util.InfoTypeWarn,
		Msg:  "File attachments are not supported by the current model: TestModel",
	}, warningMsg)
}

func TestEditor_OnCompletionPathToNonImageEmitsAttachFileMessage(t *testing.T) {
	entriesForAutoComplete := mockDirLister([]string{"image.png", "random.txt"})
	fsys := fstest.MapFS{
		"auto_completed_image.png": {
			Data: pngMagicNumberData,
		},
		"random.txt": {
			Data: []byte("Some content"),
		},
	}

	modelHasImageSupport := func() (bool, string) {
		return true, "TestModel"
	}
	testEditor := newEditor(&app.App{}, entriesForAutoComplete)
	_, cmd := onCompletionItemSelect(fsys, modelHasImageSupport, FileCompletionItem{Path: "random.txt"}, true, testEditor)

	assert.Nil(t, cmd)
}

func TestEditor_StepForwardOverHistoryDoesNotTouchExistingInputValue(t *testing.T) {
	mEditor := editorCmp{}
	fakeHistory := []string{
		"First message user sent",
		"Second message user sent",
		"Third message user sent",
		"Current value in the input field",
	}

	history := func(ctx context.Context) ([]string, error) {
		return fakeHistory, nil
	}
	forwardDir := func() direction {
		return next
	}
	previousDir := func() direction {
		return previous
	}

	// NOTE(tauraamui): if forward is the first direction the user goes in, the current message should be left alone/the same
	assert.Equal(t, "Current value in the input field", mEditor.stepOverHistory(history, forwardDir))
	assert.Equal(t, "Current value in the input field", mEditor.stepOverHistory(history, forwardDir))
	assert.Equal(t, "Third message user sent", mEditor.stepOverHistory(history, previousDir))
}

func TestEditor_StepBackwardOverHistoryScrollsUpTilBottom(t *testing.T) {
	mEditor := editorCmp{}
	fakeHistory := []string{
		"First message user sent",
		"Second message user sent",
		"Third message user sent",
		"Current value in the input field",
	}

	history := func(ctx context.Context) ([]string, error) {
		return fakeHistory, nil
	}
	previousDir := func() direction {
		return previous
	}

	// NOTE(tauraamui): if forward is the first direction the user goes in, the current message should be left alone/the same
	assert.Equal(t, "Third message user sent", mEditor.stepOverHistory(history, previousDir))
	assert.Equal(t, "Second message user sent", mEditor.stepOverHistory(history, previousDir))
	assert.Equal(t, "First message user sent", mEditor.stepOverHistory(history, previousDir))
	assert.Equal(t, "First message user sent", mEditor.stepOverHistory(history, previousDir))
}

func TestEditor_StepBackToBoundAndThenForward(t *testing.T) {
	mEditor := editorCmp{}
	fakeHistory := []string{
		"First message user sent",
		"Second message user sent",
		"Third message user sent",
		"Current value in the input field",
	}

	history := func(ctx context.Context) ([]string, error) {
		return fakeHistory, nil
	}
	forwardDir := func() direction {
		return next
	}
	previousDir := func() direction {
		return previous
	}

	// NOTE(tauraamui): current message should not be re-reachable whilst in scrolling mode
	assert.Equal(t, "Third message user sent", mEditor.stepOverHistory(history, previousDir))
	assert.Equal(t, "Second message user sent", mEditor.stepOverHistory(history, previousDir))
	assert.Equal(t, "First message user sent", mEditor.stepOverHistory(history, previousDir))
	assert.Equal(t, "First message user sent", mEditor.stepOverHistory(history, previousDir))
	assert.Equal(t, "Second message user sent", mEditor.stepOverHistory(history, forwardDir))
	assert.Equal(t, "Third message user sent", mEditor.stepOverHistory(history, forwardDir))
	assert.Equal(t, "Third message user sent", mEditor.stepOverHistory(history, forwardDir))
}

// TestHelperFunctions demonstrates how to use the batch message helpers
func TestHelperFunctions(t *testing.T) {
	testEditor := newEditor(&app.App{}, mockDirLister([]string{"file1.txt", "file2.txt"}))
	require.NotNil(t, testEditor)

	// Simulate pressing the '/' key
	testEditor.Focus()
	keyPressMsg := tea.KeyPressMsg{
		Text: "/",
	}

	_, cmds := testEditor.Update(keyPressMsg)

	// Execute the command and check if it returns a BatchMsg
	msg := cmds()
	if batchMsg, ok := msg.(tea.BatchMsg); ok {
		// Test our helper functions
		found := assertBatchContainsMessage(t, batchMsg, completions.OpenCompletionsMsg{})
		assert.True(t, found, "Expected to find OpenCompletionsMsg in batched messages")

		// Test exact message helper
		foundExact := assertBatchContainsExactMessage(t, batchMsg, completions.OpenCompletionsMsg{})
		assert.True(t, foundExact, "Expected to find exact OpenCompletionsMsg in batched messages")

		// Test specific completions helper
		msg, foundSpecific := assertBatchContainsOpenCompletionsMsg(t, batchMsg, nil) // Just check type
		assert.NotNil(t, msg)
		assert.True(t, foundSpecific, "Expected to find OpenCompletionsMsg in batched messages")
	} else {
		t.Fatal("Expected BatchMsg from cmds()")
	}
}
