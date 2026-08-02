package callback

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWritePreview is a developer aid, not an assertion: set
// CRUSH_CALLBACK_PREVIEW to a directory and it writes each page state
// there so the result can be opened in a browser and eyeballed.
func TestWritePreview(t *testing.T) {
	dir := os.Getenv("CRUSH_CALLBACK_PREVIEW")
	if dir == "" {
		t.Skip("set CRUSH_CALLBACK_PREVIEW=<dir> to render preview pages")
	}
	cases := map[string]Result{
		"ok":     {Subject: "linear"},
		"bare":   {},
		"denied": {Subject: "linear", ErrorCode: "access_denied", ErrorDescription: "The resource owner or authorization server denied the request."},
		"terse":  {Subject: "sentry", ErrorCode: "server_error"},
	}
	for name, result := range cases {
		f, err := os.Create(filepath.Join(dir, name+".html"))
		if err != nil {
			t.Fatal(err)
		}
		if err := Write(f, result); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
