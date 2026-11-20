package common

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestShouldQueryCapabilities(t *testing.T) {
	t.Parallel()

	t.Run("does not query unknown xterm terminals", func(t *testing.T) {
		t.Parallel()

		env := uv.Environ{
			"TERM=xterm-256color",
		}

		if shouldQueryCapabilities(env) {
			t.Fatal("expected unknown xterm terminal not to be queried")
		}
	})
}
