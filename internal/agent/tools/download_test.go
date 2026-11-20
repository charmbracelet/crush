package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"charm.land/fantasy"
	"github.com/stretchr/testify/require"
)

func TestDownloadToolPreservesDestinationOnIncompleteResponse(t *testing.T) {
	t.Parallel()

	for _, existing := range []bool{false, true} {
		name := "new file"
		if existing {
			name = "existing file"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			workingDir := t.TempDir()
			filePath := filepath.Join(workingDir, "download.txt")
			if existing {
				require.NoError(t, os.WriteFile(filePath, []byte("original content"), 0o600))
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "100")
				_, _ = w.Write([]byte("partial content"))
			}))
			defer server.Close()

			ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
			tool := NewDownloadTool(&mockPermissionService{}, workingDir, server.Client())
			input, err := json.Marshal(DownloadParams{URL: server.URL, FilePath: "download.txt"})
			require.NoError(t, err)
			resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "download-call", Name: DownloadToolName, Input: string(input)})
			require.NoError(t, err)
			require.True(t, resp.IsError)
			require.Contains(t, resp.Content, "unexpected EOF")

			if existing {
				content, err := os.ReadFile(filePath)
				require.NoError(t, err)
				require.Equal(t, "original content", string(content))
			} else {
				_, err := os.Stat(filePath)
				require.True(t, os.IsNotExist(err), "an incomplete download must not create the destination")
			}
			entries, err := os.ReadDir(workingDir)
			require.NoError(t, err)
			if existing {
				require.Len(t, entries, 1)
			} else {
				require.Empty(t, entries)
			}
		})
	}
}

func TestDownloadToolWritesCompleteResponse(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"new file", "existing file", "symbolic link", "hard link"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			workingDir := t.TempDir()
			filePath := filepath.Join(workingDir, "download.bin")
			targetPath := filePath
			if kind == "symbolic link" || kind == "hard link" {
				targetPath = filepath.Join(workingDir, "target.bin")
			}
			var originalInfo os.FileInfo
			if kind != "new file" {
				require.NoError(t, os.WriteFile(targetPath, []byte("original content"), 0o600))
				var err error
				originalInfo, err = os.Stat(targetPath)
				require.NoError(t, err)
			}
			switch kind {
			case "symbolic link":
				err := os.Symlink(targetPath, filePath)
				if err != nil && runtime.GOOS == "windows" {
					t.Skipf("symbolic links are unavailable: %v", err)
				}
				require.NoError(t, err)
			case "hard link":
				require.NoError(t, os.Link(targetPath, filePath))
			}

			body := []byte{0, 1, 2, 0xff, '\n'}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(body)
			}))
			defer server.Close()

			ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
			tool := NewDownloadTool(&mockPermissionService{}, workingDir, server.Client())
			input, err := json.Marshal(DownloadParams{URL: server.URL, FilePath: "download.bin"})
			require.NoError(t, err)
			resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "download-call", Name: DownloadToolName, Input: string(input)})
			require.NoError(t, err)
			require.False(t, resp.IsError)
			require.Contains(t, resp.Content, "Successfully downloaded 5 bytes to download.bin")

			content, err := os.ReadFile(filePath)
			require.NoError(t, err)
			require.Equal(t, body, content)
			if originalInfo != nil {
				info, err := os.Stat(targetPath)
				require.NoError(t, err)
				require.Equal(t, originalInfo.Mode(), info.Mode())
				require.True(t, os.SameFile(originalInfo, info))
				content, err := os.ReadFile(targetPath)
				require.NoError(t, err)
				require.Equal(t, body, content)
			}
			if kind == "symbolic link" {
				info, err := os.Lstat(filePath)
				require.NoError(t, err)
				require.NotZero(t, info.Mode()&os.ModeSymlink)
			}
		})
	}
}
