package logo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUltraWordmark(t *testing.T) {
	t.Parallel()

	wordmark := renderWord(1, -1, LetterU, LetterL, LetterT, LetterR, LetterA)
	require.Equal(t, "█   █ █    ▀▀▀▀ █▀▀▀▄ ▄▀▀▀▄\n█   █ █      █  █▀▀▀▄ █▀▀▀█\n ▀▀▀  ▀▀▀▀   ▀  ▀   ▀ ▀   ▀", wordmark)
}
