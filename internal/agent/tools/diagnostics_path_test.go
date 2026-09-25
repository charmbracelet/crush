package tools

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestSameDiagnosticPath(t *testing.T) {
	t.Parallel()

	base := filepath.Join("proj", "src", "main.py")
	if !sameDiagnosticPath(base, base) {
		t.Fatalf("identical paths should match")
	}
	dotted := filepath.Join("proj", "src", ".", "main.py")
	if !sameDiagnosticPath(dotted, base) {
		t.Fatalf("Clean should collapse . segments: %q vs %q", dotted, base)
	}
	if sameDiagnosticPath(base, "") {
		t.Fatalf("empty filePath is project-wide, not current-file")
	}
	if sameDiagnosticPath(base, filepath.Join("proj", "src", "other.py")) {
		t.Fatalf("different files must not match")
	}

	slashy := filepath.ToSlash(base)
	if !sameDiagnosticPath(slashy, base) {
		t.Fatalf("ToSlash form should match native separators: %q vs %q", slashy, base)
	}

	if runtime.GOOS == "windows" {
		winA := `C:\Users\x\proj\main.py`
		winB := `C:/Users/x/proj/main.py`
		if !sameDiagnosticPath(winA, winB) {
			t.Fatalf("Windows mixed separators should match: %q vs %q", winA, winB)
		}
		if !sameDiagnosticPath(`C:\Users\X\proj\main.py`, `c:\users\x\proj\main.py`) {
			t.Fatalf("Windows path compare should be case-insensitive")
		}
	}
}
