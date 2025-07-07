package golden

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/aymanbagabas/go-udiff"
)

var update = flag.Bool("update", false, "update .golden files")

// RequireEqual is a helper function to assert the given output is
// the expected from the golden files, printing its diff in case it is not.
//
// Golden files contain the raw expected output of your tests, which can
// contain control codes and escape sequences. When comparing the output of
// your tests, [RequireEqual] will escape the control codes and sequences
// before comparing the output with the golden files.
//
// You can update the golden files by running your tests with the -update flag.
func RequireEqual(tb testing.TB, out []byte) {
	tb.Helper()

	golden := filepath.Join("testdata", tb.Name()+".golden")
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil { //nolint: gomnd
			tb.Fatal(err)
		}
		if err := os.WriteFile(golden, out, 0o600); err != nil { //nolint: gomnd
			tb.Fatal(err)
		}
	}

	goldenBts, err := os.ReadFile(golden)
	if err != nil {
		tb.Fatal(err)
	}

	goldenStr := normalizeWindowsLineBreaks(string(goldenBts))
	goldenStr = escapeSeqs(goldenStr)
	outStr := escapeSeqs(string(out))

	diff := udiff.Unified("golden", "run", goldenStr, outStr)
	if diff != "" {
		tb.Fatalf("output does not match, expected:\n\n%s\n\ngot:\n\n%s\n\ndiff:\n\n%s", goldenStr, outStr, diff)
	}
}

// RequireEqualEscape is a helper function to assert the given output is
// the expected from the golden files, printing its diff in case it is not.
//
// Deprecated: Use [RequireEqual] instead.
func RequireEqualEscape(tb testing.TB, out []byte, escapes bool) {
	RequireEqual(tb, out)
}

// escapeSeqs escapes control codes and escape sequences from the given string.
// The only preserved exception is the newline character.
func escapeSeqs(in string) string {
	s := strings.Split(in, "\n")
	for i, l := range s {
		q := strconv.Quote(l)
		q = strings.TrimPrefix(q, `"`)
		q = strings.TrimSuffix(q, `"`)
		s[i] = q
	}
	return strings.Join(s, "\n")
}

// normalizeWindowsLineBreaks replaces all \r\n with \n.
// This is needed because Git for Windows checks out with \r\n by default.
func normalizeWindowsLineBreaks(str string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(str, "\r\n", "\n")
	}
	return str
}
