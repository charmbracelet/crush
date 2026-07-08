package clipboard

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var onePixelPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde,
}

func TestReadBridgeImage(t *testing.T) {
	const token = "test-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(onePixelPNG)
	}))
	t.Cleanup(server.Close)
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_URL", server.URL)
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN", token)
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN_FILE", "")

	data, err := readBridgeImage()
	require.NoError(t, err)
	require.True(t, bytes.Equal(onePixelPNG, data))
}

func TestReadBridgeImageNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_URL", server.URL)
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN", "test-token")

	_, err := readBridgeImage()
	require.ErrorIs(t, err, ErrEmpty)
}

func TestLoadBridgeConfigFromFile(t *testing.T) {
	configDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_URL", "")
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN", "")
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN_FILE", "")

	require.NoError(t, os.MkdirAll(filepath.Join(configDir, "crush"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, "token"), []byte("from-file\n"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(configDir, "crush", "clip-bridge.json"),
		[]byte(`{"url":"http://127.0.0.1:48731","token_file":"~/token"}`),
		0o600,
	))

	cfg, err := loadBridgeConfig()
	require.NoError(t, err)
	require.Equal(t, "from-file", cfg.Token)
	require.Equal(t, "http://127.0.0.1:48731", cfg.URL)
}

func TestReadBridgeImageRejectsNonLoopbackURL(t *testing.T) {
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_URL", "https://example.com")
	t.Setenv("CRUSH_CLIPBOARD_BRIDGE_TOKEN", "must-not-leak")

	_, err := readBridgeImage()
	require.EqualError(t, err, "clipboard bridge URL must use a loopback host")
}
