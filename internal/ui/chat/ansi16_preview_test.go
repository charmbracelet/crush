package chat

import (
	"fmt"
	"strconv"
	"testing"
)

// TestPreviewANSI16Palette prints a swatch grid using the raw ANSI 16
// color codes (the codes terminal programs actually emit). It is not an
// assertion test; it exists so you can eyeball how the codes render:
//
//	go test ./internal/ui/chat/ -run TestPreviewANSI16Palette -v
//
// These are the unmodified SGR codes, so what you see is your terminal's
// own palette, not Crush's remap.
func TestPreviewANSI16Palette(t *testing.T) {
	names := []string{
		"black", "red", "green", "yellow",
		"blue", "magenta", "cyan", "white",
	}

	fmt.Println("\nForeground (normal | bright):")
	for i, name := range names {
		normal := "\x1b[" + strconv.Itoa(30+i) + "m" + pad(name) + "\x1b[0m"
		bright := "\x1b[" + strconv.Itoa(90+i) + "m" + pad("bright "+name) + "\x1b[0m"
		fmt.Printf("  %s  %s\n", normal, bright)
	}

	fmt.Println("\nBackground (normal | bright):")
	for i, name := range names {
		normal := "\x1b[30;" + strconv.Itoa(40+i) + "m" + pad(name) + "\x1b[0m"
		bright := "\x1b[30;" + strconv.Itoa(100+i) + "m" + pad("bright "+name) + "\x1b[0m"
		fmt.Printf("  %s  %s\n", normal, bright)
	}
	fmt.Println()
}

func pad(s string) string {
	const w = 16
	for len(s) < w {
		s += " "
	}
	return " " + s
}
