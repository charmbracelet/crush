package binary

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func archiveFixture(t *testing.T, name string, kind byte) ([]byte, artifact) {
	t.Helper()
	content := []byte("test executable bytes")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	writer := tar.NewWriter(gz)
	header := &tar.Header{Name: name, Typeflag: kind, Mode: 0o700}
	if kind == tar.TypeReg {
		header.Size = int64(len(content))
	} else {
		header.Linkname = "outside"
	}
	require.NoError(t, writer.WriteHeader(header))
	if kind == tar.TypeReg {
		_, err := writer.Write(content)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, gz.Close())
	data := buf.Bytes()
	hash := sha256.Sum256(content)
	integrity := sha512.Sum512(data)
	return data, artifact{Member: "package/vendor/bin/filesnap", SHA256: hex.EncodeToString(hash[:]), Integrity: "sha512-" + base64.StdEncoding.EncodeToString(integrity[:])}
}

func TestDownloadVerifiesPinnedArchiveAndReportsFailures(t *testing.T) {
	t.Parallel()
	data, a := archiveFixture(t, "package/vendor/bin/filesnap", tar.TypeReg)
	for _, tc := range []struct {
		name    string
		status  int
		body    []byte
		message string
	}{
		{"valid", http.StatusOK, data, ""},
		{"corrupt", http.StatusOK, []byte("tampered"), "integrity mismatch"},
		{"missing", http.StatusNotFound, nil, "HTTP 404"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status); _, _ = w.Write(tc.body) }))
			defer server.Close()
			selected := a
			selected.URL = server.URL
			got, err := download(t.Context(), server.Client(), selected)
			if tc.message != "" {
				require.ErrorContains(t, err, tc.message)
				return
			}
			require.NoError(t, err)
			require.Equal(t, data, got)
			cancelled, cancel := context.WithCancel(t.Context())
			cancel()
			_, err = download(cancelled, server.Client(), selected)
			require.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestExtractionNeverAcceptsOtherPathsLinksOrChangedExecutables(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		kind    byte
		member  string
		badHash bool
	}{
		{"traversal", tar.TypeReg, "../../filesnap", false},
		{"symlink", tar.TypeSymlink, "package/vendor/bin/filesnap", false},
		{"wrong binary", tar.TypeReg, "package/vendor/bin/filesnap", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, a := archiveFixture(t, tc.member, tc.kind)
			if tc.badHash {
				a.SHA256 = "wrong"
			}
			_, err := extract(data, a)
			require.Error(t, err)
		})
	}
}

func TestProvisionIsAtomicReusesVerifiedCacheAndRejectsTampering(t *testing.T) {
	t.Parallel()
	data, a := archiveFixture(t, "package/vendor/bin/filesnap", tar.TypeReg)
	directory := t.TempDir()
	var calls atomic.Int32
	source := func(context.Context) ([]byte, error) { calls.Add(1); return data, nil }
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for range 8 {
		wg.Go(func() {
			executable, err := provision(t.Context(), directory, a, source)
			if err == nil {
				var content []byte
				content, err = os.ReadFile(executable)
				if err == nil && !matches(content, a.SHA256) {
					err = fmt.Errorf("partial executable")
				}
			}
			errors <- err
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), calls.Load())
	executable, err := provision(t.Context(), directory, a, func(context.Context) ([]byte, error) { return nil, fmt.Errorf("offline") })
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(filepath.Dir(executable), "LICENSE"))
	require.FileExists(t, filepath.Join(filepath.Dir(executable), "NOTICE"))
	require.NoError(t, os.WriteFile(executable, []byte("tampered"), 0o700))
	_, err = provision(t.Context(), directory, a, source)
	require.ErrorContains(t, err, "checksum mismatch")
	require.Equal(t, int32(1), calls.Load())
}
