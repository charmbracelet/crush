package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceTextJoinsSmallContents(t *testing.T) {
	t.Parallel()

	got := ResourceText([]*ResourceContents{
		{URI: "file:///a", Text: "first"},
		nil,
		{URI: "file:///b", Blob: []byte("second")},
		{URI: "file:///c"},
	}, t.TempDir())
	require.Equal(t, "first\nsecond", got)
	require.Equal(t, "", ResourceText(nil, t.TempDir()))
}

func TestResourceTextOmitsBinaryBlobs(t *testing.T) {
	t.Parallel()

	got := ResourceText([]*ResourceContents{
		{URI: "file:///img.png", MIMEType: "image/png", Blob: []byte{0x89, 'P', 'N', 'G', 0, 0xff}},
	}, t.TempDir())
	require.Equal(t, "[binary resource file:///img.png, image/png, 6 bytes omitted]", got)
}

func TestResourceTextCapsOversizedContents(t *testing.T) {
	t.Parallel()

	// Many parts that are each small must still be bounded as a whole.
	part := strings.Repeat("a line of resource text\n", 500)
	var contents []*ResourceContents
	for range 20 {
		contents = append(contents, &ResourceContents{URI: "file:///big", Text: part})
	}
	full := strings.TrimSuffix(strings.Repeat(part+"\n", 20), "\n")

	got := ResourceText(contents, t.TempDir())
	require.Less(t, len(got), len(full))

	marker := "full output: "
	require.Contains(t, got, marker)
	rest := got[strings.LastIndex(got, marker)+len(marker):]
	path := strings.Fields(strings.SplitN(rest, "\n", 2)[0])[0]
	require.True(t, filepath.IsAbs(path))

	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, full, string(saved))
}
