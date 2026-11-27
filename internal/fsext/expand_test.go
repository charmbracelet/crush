package fsext

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "hello", want: "hello"},
		{name: "posix path", input: "/tmp/some dir/x", want: "/tmp/some dir/x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Expand(tt.input)
			if err != nil {
				t.Fatalf("Expand(%q): %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Expand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandWindowsPath(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("windows path separators only exist on windows")
	}

	input := `C:\Users\RUNNER~1\some dir\x\001`
	want := filepath.ToSlash(input)
	got, err := Expand(input)
	if err != nil {
		t.Fatalf("Expand(%q): %v", input, err)
	}
	if got != want {
		t.Errorf("Expand(%q) = %q, want %q", input, got, want)
	}
}
