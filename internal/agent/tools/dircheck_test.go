package tools

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirRestrictions_DenyIfRestricted(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "project")
	extraDir := filepath.Join(tmpDir, "extra")
	outsideDir := filepath.Join(tmpDir, "outside")

	require.NoError(t, os.MkdirAll(filepath.Join(workDir, "src"), 0o755))
	require.NoError(t, os.MkdirAll(extraDir, 0o755))
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))
	// Create test files so EvalSymlinks works
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "src", "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(extraDir, "lib.go"), []byte("package lib"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "file.go"), []byte("package out"), 0o644))

	tests := []struct {
		name        string
		restrictions DirRestrictions
		absPath     string
		wantDenied  bool
	}{
		{
			name: "restricted mode off allows everything",
			restrictions: DirRestrictions{
				WorkingDir:        workDir,
				RestrictToProject: false,
			},
			absPath:    filepath.Join(outsideDir, "file.go"),
			wantDenied: false,
		},
		{
			name: "restricted mode denies outside working dir",
			restrictions: DirRestrictions{
				WorkingDir:        workDir,
				RestrictToProject: true,
			},
			absPath:    filepath.Join(outsideDir, "file.go"),
			wantDenied: true,
		},
		{
			name: "restricted mode allows working dir",
			restrictions: DirRestrictions{
				WorkingDir:        workDir,
				RestrictToProject: true,
			},
			absPath:    filepath.Join(workDir, "src", "main.go"),
			wantDenied: false,
		},
		{
			name: "restricted mode allows additional dirs",
			restrictions: DirRestrictions{
				WorkingDir:        workDir,
				AdditionalDirs:    []string{extraDir},
				RestrictToProject: true,
			},
			absPath:    filepath.Join(extraDir, "lib.go"),
			wantDenied: false,
		},
		{
			name: "restricted mode denies path not in additional dirs",
			restrictions: DirRestrictions{
				WorkingDir:        workDir,
				AdditionalDirs:    []string{extraDir},
				RestrictToProject: true,
			},
			absPath:    filepath.Join(outsideDir, "file.go"),
			wantDenied: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.restrictions.DenyIfRestricted(tt.absPath, "test")
			if tt.wantDenied {
				require.NotNil(t, result, "expected denial but got nil")
				require.True(t, result.IsError)
			} else {
				require.Nil(t, result, "expected nil but got denial")
			}
		})
	}
}
