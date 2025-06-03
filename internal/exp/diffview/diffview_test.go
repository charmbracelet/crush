package diffview_test

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"
	"github.com/opencode-ai/opencode/internal/exp/diffview"
)

func TestDefault(t *testing.T) {
	dv := diffview.New().
		Before("test.txt", "This is the original content.").
		After("test.txt", "This is the modified content.")
	golden.RequireEqual(t, []byte(dv.String()))
}
