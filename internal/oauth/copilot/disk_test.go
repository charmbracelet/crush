package copilot

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeAppsJSON points the token lookup at a temporary home and seeds
// apps.json with body. It cannot be parallel: t.Setenv forbids it.
func writeAppsJSON(t *testing.T, body string) {
	t.Helper()

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("HOME", dir)
	}

	path := tokenFilePath()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func TestRefreshTokenFromDisk(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		want  string
		found bool
	}{
		{
			name:  "our own app id",
			body:  `{"github.com:Iv1.b507a08c87ecfe98":{"user":"bob","oauth_token":"gho_ours"}}`,
			want:  "gho_ours",
			found: true,
		},
		{
			name:  "another client's app id",
			body:  `{"github.com:Iv23liAbCdEfGh":{"user":"bob","oauth_token":"gho_theirs"}}`,
			want:  "gho_theirs",
			found: true,
		},
		{
			name:  "host without an app id",
			body:  `{"github.com":{"user":"bob","oauth_token":"gho_plain"}}`,
			want:  "gho_plain",
			found: true,
		},
		{
			name: "our app id wins over another client's",
			body: `{"github.com:Iv23liAbCdEfGh":{"oauth_token":"gho_theirs"},
			        "github.com:Iv1.b507a08c87ecfe98":{"oauth_token":"gho_ours"}}`,
			want:  "gho_ours",
			found: true,
		},
		{
			name: "several foreign entries resolve to the first by key",
			body: `{"github.com:Iv50zzz":{"oauth_token":"gho_z"},
			        "github.com:Iv10aaa":{"oauth_token":"gho_a"}}`,
			want:  "gho_a",
			found: true,
		},
		{
			name:  "enterprise host is not used against the public API",
			body:  `{"tenant.ghe.com:Iv1.b507a08c87ecfe98":{"oauth_token":"gho_ghe"}}`,
			found: false,
		},
		{
			name:  "lookalike host is rejected",
			body:  `{"github.com.evil.example:Iv1.x":{"oauth_token":"gho_evil"}}`,
			found: false,
		},
		{
			name:  "entry without a token is skipped",
			body:  `{"github.com:Iv23liAbCdEfGh":{"user":"bob","oauth_token":""}}`,
			found: false,
		},
		{
			name:  "malformed file",
			body:  `not json`,
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeAppsJSON(t, tt.body)

			token, ok := RefreshTokenFromDisk()

			require.Equal(t, tt.found, ok)
			require.Equal(t, tt.want, token)
		})
	}
}

func TestRefreshTokenFromDiskIsStableAcrossCalls(t *testing.T) {
	writeAppsJSON(t, `{"github.com:Iv50zzz":{"oauth_token":"gho_z"},
	                   "github.com:Iv10aaa":{"oauth_token":"gho_a"},
	                   "github.com:Iv30mmm":{"oauth_token":"gho_m"}}`)

	first, ok := RefreshTokenFromDisk()
	require.True(t, ok)

	for range 20 {
		again, ok := RefreshTokenFromDisk()
		require.True(t, ok)
		require.Equal(t, first, again, "map iteration order must not leak into the choice")
	}
}

func TestRefreshTokenFromDiskWithoutFile(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", dir)
	} else {
		t.Setenv("HOME", dir)
	}

	token, ok := RefreshTokenFromDisk()

	require.False(t, ok)
	require.Empty(t, token)
}
