package model

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/imageutil"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/ui/attachments"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/stretchr/testify/require"
)

func TestClipboardPathCandidates(t *testing.T) {
	t.Parallel()

	paths := clipboardPathCandidates("C:\\a.png\r\n\"C:\\b space.jpg\"\n\x00")
	require.Equal(t, []string{"C:\\a.png", "\"C:\\b space.jpg\""}, paths)
}

func TestNormalizeClipboardPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "quoted", in: "\"C:\\Users\\me\\shot.png\"", want: `C:\Users\me\shot.png`},
		{name: "escaped space", in: `C:\Users\me\shot\ image.png`, want: `C:\Users\me\shot image.png`},
		{name: "file uri", in: "file:///C:/Users/me/Pictures/shot%20one.png", want: `C:\Users\me\Pictures\shot one.png`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, normalizeClipboardPath(tt.in))
		})
	}
}

func TestHandleClipboardImageMsgCompressesBeforeSizeLimit(t *testing.T) {
	oversized := oversizedCompressibleJPEG(t)
	ui := &UI{
		attachments: attachments.New(
			attachments.NewRenderer(
				lipgloss.NewStyle(),
				lipgloss.NewStyle(),
				lipgloss.NewStyle(),
				lipgloss.NewStyle(),
			),
			attachments.Keymap{},
		),
	}

	cmd := ui.handleClipboardImageMsg(clipboardImageMsg{imageData: oversized})
	require.NotNil(t, cmd)

	result := cmd()
	attachment, ok := result.(message.Attachment)
	require.True(t, ok)
	require.LessOrEqual(t, int64(len(attachment.Content)), common.MaxAttachmentSize)
	require.Equal(t, "image/jpeg", attachment.MimeType)
}

func TestAttachmentFromClipboardPathCompressesBeforeSizeLimit(t *testing.T) {
	oversized := oversizedCompressibleJPEG(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "pasted.jpg")
	require.NoError(t, os.WriteFile(path, oversized, 0o644))

	attachment, err := attachmentFromClipboardPath(path)
	require.NoError(t, err)
	require.LessOrEqual(t, int64(len(attachment.Content)), common.MaxAttachmentSize)
	require.Equal(t, "image/jpeg", attachment.MimeType)
}

func oversizedCompressibleJPEG(t *testing.T) []byte {
	t.Helper()

	for _, size := range []int{2800, 3200, 3600} {
		data := buildJPEG(t, size)
		if int64(len(data)) <= common.MaxAttachmentSize {
			continue
		}
		result, err := imageutil.CompressImage(data, "image/jpeg", imageutil.DefaultCompressionConfig())
		require.NoError(t, err)
		if int64(len(result.Data)) <= common.MaxAttachmentSize {
			return data
		}
	}

	t.Skip("unable to generate oversized but compressible image fixture")
	return nil
}

func buildJPEG(t *testing.T, size int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x*31 + y*17) % 256),
				G: uint8((x*13 + y*29) % 256),
				B: uint8((x*7 + y*11) % 256),
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}))
	return buf.Bytes()
}
