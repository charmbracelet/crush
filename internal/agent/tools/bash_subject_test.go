package tools

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPermissionSubjectScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{"simple build", "swift build", "swift build"},
		{"pipeline keeps primary", "swift build 2>&1 | tail -20", "swift build"},
		{"env prefix skipped", "FOO=1 swift test", "swift test"},
		{"flags skipped", "swift test --verbose --parallel", "swift test"},
		{"absolute path basenamed", "/usr/bin/grep pattern file.txt", "grep pattern"},
		{"relative script", "./deploy.sh production", "deploy.sh production"},
		{"chained sorted deduped", "git commit && git status && git commit", "git commit+git status"},
		{"compound keeps both", "swift build && ./run_tests.sh", "run_tests.sh+swift build"},
		{"redirect only args", "make 2>/dev/null", "make"},
		{"empty falls back", "", "bash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, permissionSubjectScope(tt.cmd))
		})
	}
}
