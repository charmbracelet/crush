package model

import (
	"strings"
	"testing"
	"charm.land/bubbles/v2/textarea"
	"github.com/stretchr/testify/require"
)

func TestCursorByteIndex(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		actions    func(*textarea.Model)
		want       int
	}{
		{
			name:  "empty",
			value: "",
			want:  0,
		},
		{
			name:  "first line mid",
			value: "hello world",
			actions: func(ta *textarea.Model) {
				ta.CursorUp()
				ta.SetCursorColumn(5)
			},
			want: 5,
		},
		{
			name:  "second line mid",
			value: "hello\nworld",
			actions: func(ta *textarea.Model) {
				ta.CursorDown()
				ta.SetCursorColumn(5)
			},
			want: 11,
		},
		{
			name:  "multibyte characters",
			value: "hëllo\nwørld",
			actions: func(ta *textarea.Model) {
				ta.CursorUp()
				ta.SetCursorColumn(5)
			},
			want: 6, // 'ë' is 2 bytes + 4 more runes (1 byte each) = 6 bytes
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := textarea.New()
			ta.SetValue(tt.value)
			if tt.actions != nil {
				tt.actions(&ta)
			}
			got := cursorByteIndex(ta)
			t.Logf("Row: %d, Col: %d, Value: %q, Lines: %v", ta.Line(), ta.Column(), ta.Value(), strings.Split(ta.Value(), "\n"))
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIsWhitespace(t *testing.T) {
	require.True(t, isWhitespace(' '))
	require.True(t, isWhitespace('\t'))
	require.True(t, isWhitespace('\n'))
	require.True(t, isWhitespace('\r'))
	require.False(t, isWhitespace('a'))
	require.False(t, isWhitespace('0'))
}

