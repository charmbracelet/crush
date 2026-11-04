package plan

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTitle(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{name: "h1", text: "# Fix login timeout\n\nSteps.", want: "Fix login timeout"},
		{name: "leading blank lines", text: "\n\n  # Spaced title  \n", want: "Spaced title"},
		{name: "no title", text: "Just prose.\n\n## Subheading\n", want: ""},
		{name: "empty", text: "", want: ""},
		{name: "skips marker lines", text: StartMarker + "\n# Real title\n" + ReadyMarker, want: "Real title"},
		{name: "skips fenced heading", text: "```\n# Not this\n```\n# Real title\n", want: "Real title"},
		{name: "h2 only", text: "## Secondary\n", want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Title(tc.text))
		})
	}
}

func TestFilename(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, time.September, 19, 15, 30, 45, 0, time.UTC)
	for _, tc := range []struct {
		name string
		text string
		want string
	}{
		{name: "titled", text: "# Fix login timeout\n", want: "2026-09-19-153045-fix-login-timeout.md"},
		{name: "no title", text: "Just prose.", want: "2026-09-19-153045-plan.md"},
		{
			name: "punctuation collapsed",
			text: "# Fix login: timeout & retries!\n",
			want: "2026-09-19-153045-fix-login-timeout-retries.md",
		},
		{name: "unicode preserved", text: "# Исправить таймаут входа\n", want: "2026-09-19-153045-исправить-таймаут-входа.md"},
		{name: "long title truncated", text: "# " + longTitle, want: "2026-09-19-153045-" + longSlug + ".md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, Filename(tc.text, at))
		})
	}
}

const (
	longTitle = "aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff gggggggggg"
	longSlug  = "aaaaaaaaaa-bbbbbbbbbb-cccccccccc-dddddddddd-eeeeeeeeee-fffff"
)
