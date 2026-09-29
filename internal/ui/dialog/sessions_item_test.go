package dialog

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
)

func TestTitleInputCursorX(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		value  string
		cursor int
		width  int
		want   int
	}{
		{name: "ascii", value: "hello", cursor: 5, width: 0, want: 5},
		{name: "cjk at end", value: "重命名对话abc", cursor: 8, width: 0, want: 13},
		{name: "cjk mid", value: "重命名对话abc", cursor: 4, width: 0, want: 8},
		{name: "cjk clamped", value: "重命名对话abc", cursor: 8, width: 10, want: 10},
		{name: "empty", value: "", cursor: 0, width: 0, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := textinput.New()
			in.SetVirtualCursor(false)
			in.SetValue(tt.value)
			in.SetCursor(tt.cursor)
			in.SetWidth(tt.width)

			if got := titleInputCursorX(in); got != tt.want {
				t.Fatalf("titleInputCursorX() = %d, want %d", got, tt.want)
			}
		})
	}
}
