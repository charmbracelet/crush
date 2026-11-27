package dialog

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	"github.com/stretchr/testify/require"
)

// newPickerTestModels builds a models dialog without fetching providers.
func newPickerTestModels() *Models {
	com := newPickerTestCommon()
	m := &Models{com: com, modelType: ModelTypeLarge}
	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()
	m.list = NewModelsList(com.Styles)
	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.Placeholder = m.modelType.Placeholder()
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()
	m.keyMap.Close = CloseKey
	return m
}

// TestModelsNarrowKeepsContext verifies that on a narrow screen the model
// picker keeps the Large/Small switch (with short labels) and ellipsizes
// the placeholder instead of cutting it mid-word.
func TestModelsNarrowKeepsContext(t *testing.T) {
	t.Parallel()
	text := strings.Join(drawFits(t, newPickerTestModels(), 40, 30), "\n")
	require.Contains(t, text, "Large")
	require.Contains(t, text, "Small")
	require.Contains(t, text, "…")

	text = strings.Join(drawFits(t, newPickerTestModels(), 100, 30), "\n")
	require.Contains(t, text, "Large Task")
	require.Contains(t, text, largeModelInputPlaceholder)
}
