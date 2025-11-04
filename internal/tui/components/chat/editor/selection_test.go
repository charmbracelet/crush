package editor

import (
	"testing"

	"github.com/charmbracelet/bubbles/v2/textarea"
	"github.com/stretchr/testify/require"
)

func TestSelection_NewSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		start int
		end   int
		want  Selection
	}{
		{
			name:  "normal selection",
			start: 2,
			end:   5,
			want:  Selection{Start: 2, End: 5, Active: false},
		},
		{
			name:  "zero selection",
			start: 0,
			end:   0,
			want:  Selection{Start: 0, End: 0, Active: false},
		},
		{
			name:  "negative selection",
			start: -1,
			end:   -1,
			want:  Selection{Start: -1, End: -1, Active: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NewSelection(tt.start, tt.end)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSelection_IsActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    Selection
		want bool
	}{
		{"active selection", Selection{Start: 0, End: 5, Active: false}, true},
		{"same start and end", Selection{Start: 3, End: 3, Active: false}, false},
		{"negative start", Selection{Start: -1, End: 5, Active: false}, false},
		{"negative end", Selection{Start: 0, End: -1, Active: false}, false},
		{"both negative", Selection{Start: -1, End: -1, Active: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.s.IsActive()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSelection_Length(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		s    Selection
		want int
	}{
		{"normal selection", Selection{Start: 2, End: 5, Active: false}, 3},
		{"backward selection", Selection{Start: 5, End: 2, Active: false}, 3},
		{"zero length", Selection{Start: 3, End: 3, Active: false}, 0},
		{"inactive", Selection{Start: -1, End: -1, Active: false}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.s.Length()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestSelection_Bounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		s          Selection
		wantStart  int
		wantEnd    int
	}{
		{"forward selection", Selection{Start: 2, End: 5, Active: false}, 2, 5},
		{"backward selection", Selection{Start: 5, End: 2, Active: false}, 2, 5},
		{"same start and end", Selection{Start: 3, End: 3, Active: false}, 0, 0},
		{"inactive selection", Selection{Start: -1, End: -1, Active: false}, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			start, end := tt.s.Bounds()
			require.Equal(t, tt.wantStart, start)
			require.Equal(t, tt.wantEnd, end)
		})
	}
}

func TestSelection_Contains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s        Selection
		pos      int
		contains bool
	}{
		{"within forward selection", Selection{Start: 2, End: 5, Active: false}, 3, true},
		{"within backward selection", Selection{Start: 5, End: 2, Active: false}, 3, true},
		{"at start boundary", Selection{Start: 2, End: 5, Active: false}, 2, true},
		{"at end boundary", Selection{Start: 2, End: 5, Active: false}, 4, true},
		{"before selection", Selection{Start: 2, End: 5, Active: false}, 1, false},
		{"after selection", Selection{Start: 2, End: 5, Active: false}, 5, false},
		{"inactive selection", Selection{Start: -1, End: -1, Active: false}, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.s.Contains(tt.pos)
			require.Equal(t, tt.contains, got)
		})
	}
}

func TestSelection_Clear(t *testing.T) {
	t.Parallel()

	s := Selection{Start: 2, End: 5, Active: true}
	s.Clear()

	require.Equal(t, -1, s.Start)
	require.Equal(t, -1, s.End)
	require.Equal(t, false, s.Active)
}

func TestSelection_SelectAll(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		start int
		end   int
	}{
		{"empty text", "", 0, 0},
		{"single word", "hello", 0, 5},
		{"multiple words", "hello world", 0, 11},
		{"multiline", "line1\nline2", 0, 11},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := Selection{}
			s.SelectAll(tt.text)
			require.Equal(t, tt.start, s.Start)
			require.Equal(t, tt.end, s.End)
			require.Equal(t, false, s.Active)
		})
	}
}

func TestSelection_GetText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		s        Selection
		text     string
		expected string
	}{
		{"forward selection", Selection{Start: 2, End: 5, Active: false}, "hello", "llo"},
		{"backward selection", Selection{Start: 5, End: 2, Active: false}, "hello", "llo"},
		{"full selection", Selection{Start: 0, End: 5, Active: false}, "hello", "hello"},
		{"empty text", Selection{Start: 0, End: 0, Active: false}, "", ""},
		{"out of bounds", Selection{Start: -5, End: 10, Active: false}, "hello", ""},
		{"inactive selection", Selection{Start: -1, End: -1, Active: false}, "hello", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.s.GetText(tt.text)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestSelectionManager_NewSelectionManager(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	sm := NewSelectionManager(ta)

	require.NotNil(t, sm)
	require.Equal(t, ta, sm.textarea)
	require.False(t, sm.HasSelection())
	require.Empty(t, sm.GetSelectedText())
}

func TestSelectionManager_SelectAll(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("test content")
	sm := NewSelectionManager(ta)

	sm.SelectAll()

	require.True(t, sm.HasSelection())
	require.Equal(t, "test content", sm.GetSelectedText())
	
	selection := sm.GetSelection()
	require.Equal(t, 0, selection.Start)
	require.Equal(t, len("test content"), selection.End)
}

func TestSelectionManager_Clear(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("test content")
	sm := NewSelectionManager(ta)

	// First select something
	sm.SelectAll()
	require.True(t, sm.HasSelection())

	// Then clear
	sm.Clear()
	require.False(t, sm.HasSelection())
	require.Empty(t, sm.GetSelectedText())
}

func TestSelectionManager_SetSelection(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("hello world")
	sm := NewSelectionManager(ta)

	sm.SetSelection(2, 5)

	require.True(t, sm.HasSelection())
	require.Equal(t, "llo", sm.GetSelectedText())
	
	selection := sm.GetSelection()
	require.Equal(t, 2, selection.Start)
	require.Equal(t, 5, selection.End)
	require.False(t, selection.Active)
}

func TestSelectionManager_GetSelection(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("test")
	sm := NewSelectionManager(ta)

	// Initially no selection
	selection := sm.GetSelection()
	require.Equal(t, -1, selection.Start)
	require.Equal(t, -1, selection.End)
	require.False(t, selection.Active)

	// Set selection
	sm.SetSelection(1, 3)
	selection = sm.GetSelection()
	require.Equal(t, 1, selection.Start)
	require.Equal(t, 3, selection.End)
	require.False(t, selection.Active)
}

func TestSelectionManager_GetSelectedText(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("hello world")
	sm := NewSelectionManager(ta)

	// Test empty selection
	text := sm.GetSelectedText()
	require.Empty(t, text)

	// Test partial selection
	sm.SetSelection(2, 7)
	text = sm.GetSelectedText()
	require.Equal(t, "llo w", text)

	// Test select all
	sm.SelectAll()
	text = sm.GetSelectedText()
	require.Equal(t, "hello world", text)

	// Test backward selection
	sm.SetSelection(7, 2)
	text = sm.GetSelectedText()
	require.Equal(t, "llo w", text)
}

func TestSelectionManager_HasSelection(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("test")
	sm := NewSelectionManager(ta)

	// Initially no selection
	require.False(t, sm.HasSelection())

	// Set empty selection (start == end)
	sm.SetSelection(2, 2)
	require.False(t, sm.HasSelection())

	// Set negative selection
	sm.SetSelection(-1, -1)
	require.False(t, sm.HasSelection())

	// Set valid selection
	sm.SetSelection(0, 4)
	require.True(t, sm.HasSelection())
}

// Test edge cases and integration scenarios
func TestSelectionManager_Integration(t *testing.T) {
	t.Parallel()

	ta := textarea.New()
	ta.SetValue("line1\nline2\nline3")
	sm := NewSelectionManager(ta)

	// Test selecting across newlines
	sm.SetSelection(3, 8) // From '1' in line1 to 'n' in line2
	require.True(t, sm.HasSelection())
	require.Equal(t, "e1\nli", sm.GetSelectedText())

	// Test clearing and reselecting
	sm.Clear()
	require.False(t, sm.HasSelection())
	
	sm.SelectAll()
	require.True(t, sm.HasSelection())
	require.Equal(t, "line1\nline2\nline3", sm.GetSelectedText())

	// Test that selection updates with textarea content
	ta.SetValue("new content")
	sm.SelectAll()
	require.Equal(t, "new content", sm.GetSelectedText())
}