package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClipboardPathCandidates(t *testing.T) {
	t.Parallel()

	paths := clipboardPathCandidates("C:\\a.png\r\n\"C:\\b space.jpg\"\n\x00")
	require.Equal(t, []string{"C:\\a.png", "\"C:\\b space.jpg\""}, paths)
}

func TestNormalizeClipboardPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "quoted", in: "\"C:\\Users\\me\\shot.png\"", want: `C:\Users\me\shot.png`},
		{name: "escaped space", in: `C:\Users\me\shot\ image.png`, want: `C:\Users\me\shot image.png`},
		{name: "file uri", in: "file:///C:/Users/me/Pictures/shot%20one.png", want: `C:\Users\me\Pictures\shot one.png`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, normalizeClipboardPath(tt.in))
		})
	}
}
