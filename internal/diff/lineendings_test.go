package diff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// An edit is applied to the LF-normalized form of a file, so putting the file
// back together means keeping the raw bytes of every line the edit did not
// touch and giving the lines it did introduce the ending of the region they
// landed in.
func TestRestoreLineEndings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		raw    string
		before string
		after  string
		want   string
	}{
		{
			name:   "an LF file is already the normalized form",
			raw:    "a\nb\nc\n",
			before: "a\nb\nc\n",
			after:  "a\nB\nc\n",
			want:   "a\nB\nc\n",
		},
		{
			name:   "a CRLF file comes back CRLF",
			raw:    "a\r\nb\r\nc\r\n",
			before: "a\nb\nc\n",
			after:  "a\nB\nc\n",
			want:   "a\r\nB\r\nc\r\n",
		},
		{
			name:   "one CRLF in an LF file stays where it was",
			raw:    "a\nb\r\nc\n",
			before: "a\nb\nc\n",
			after:  "a\nB\nc\n",
			want:   "a\nB\r\nc\n",
		},
		{
			name:   "one LF in a CRLF file stays where it was",
			raw:    "a\r\nb\nc\r\n",
			before: "a\nb\nc\n",
			after:  "a\nB\nc\n",
			want:   "a\r\nB\nc\r\n",
		},
		{
			name:   "an added line follows the line before it",
			raw:    "a\nb\r\nc\n",
			before: "a\nb\nc\n",
			after:  "a\nb\nB\nc\n",
			want:   "a\nb\r\nB\r\nc\n",
		},
		{
			name:   "a line added before anything follows the first ending",
			raw:    "a\r\nb\r\n",
			before: "a\nb\n",
			after:  "X\na\nb\n",
			want:   "X\r\na\r\nb\r\n",
		},
		{
			name:   "a removed line takes its own ending with it",
			raw:    "a\nb\r\nc\n",
			before: "a\nb\nc\n",
			after:  "a\nc\n",
			want:   "a\nc\n",
		},
		{
			name:   "each replaced region keeps its own ending",
			raw:    "x\r\nb\ny\nb\r\nz\n",
			before: "x\nb\ny\nb\nz\n",
			after:  "x\nB\ny\nB\nz\n",
			want:   "x\r\nB\ny\nB\r\nz\n",
		},
		{
			// new_string can carry a CRLF of its own; it joins the region
			// instead of arriving with a doubled CR.
			name:   "a CRLF inside the inserted text follows the region",
			raw:    "a\nb\r\nc\n",
			before: "a\nb\nc\n",
			after:  "a\nB\r\nB2\nc\n",
			want:   "a\nB\r\nB2\r\nc\n",
		},
		{
			name:   "an unterminated last line keeps its neighbours",
			raw:    "a\r\nb",
			before: "a\nb",
			after:  "a\nB",
			want:   "a\r\nB",
		},
		{
			name:   "a line added after an unterminated line takes the ending before it",
			raw:    "a\r\nb",
			before: "a\nb",
			after:  "a\nb\nB",
			want:   "a\r\nb\r\nB",
		},
		{
			name:   "a file with no ending at all stays LF",
			raw:    "a\r",
			before: "a\r",
			after:  "b\r",
			want:   "b\r",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := RestoreLineEndings(tt.raw, tt.before, tt.after)
			require.Equal(t, tt.want, got)
			// Whatever the file ended up holding, its lines are the edited ones:
			// the reconstruction is only allowed to decide line endings. The
			// expectation is normalized too, because a CRLF that came from
			// new_string is folded into the ending of the region it landed in.
			want := strings.ReplaceAll(tt.after, "\r\n", "\n")
			require.Equal(t, want, strings.ReplaceAll(got, "\r\n", "\n"), "the edit has to survive the reconstruction intact")
		})
	}
}
