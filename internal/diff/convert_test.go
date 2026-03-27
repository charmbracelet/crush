package diff

import (
	"strings"
	"testing"

	"github.com/aymanbagabas/go-udiff"
	"github.com/stretchr/testify/require"
)

func TestConvertMyersToUnifiedDiff(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		before       string
		after        string
		contextLines int
		checkFunc    func(t *testing.T, unified udiff.UnifiedDiff)
	}{
		{
			name:         "simple line insertion",
			before:       "line 1\nline 2\nline 3\nline 4\nline 5\n",
			after:        "line 1\nline 2\nNEW LINE INSERTED HERE\nline 3\nline 4\nline 5\n",
			contextLines: 3,
			checkFunc: func(t *testing.T, unified udiff.UnifiedDiff) {
				t.Helper()
				require.Len(t, unified.Hunks, 1, "should have exactly one hunk")

				hunk := unified.Hunks[0]
				require.GreaterOrEqual(t, len(hunk.Lines), 3, "should have at least 3 lines")

				insertFound := false
				line2Changed := false

				for _, line := range hunk.Lines {
					if strings.Contains(line.Content, "NEW LINE INSERTED HERE") {
						require.Equal(t, udiff.Insert, line.Kind, "new line should be marked as insert")
						insertFound = true
					}
					if strings.Contains(line.Content, "line 2") {
						if line.Kind == udiff.Delete {
							line2Changed = true
						}
					}
				}

				require.True(t, insertFound, "should find the inserted line")
				require.False(t, line2Changed, "line 2 should not be marked as changed")
			},
		},
		{
			name:         "line deletion",
			before:       "line 1\nline 2\nline 3\nline 4\nline 5\n",
			after:        "line 1\nline 2\nline 4\nline 5\n",
			contextLines: 3,
			checkFunc: func(t *testing.T, unified udiff.UnifiedDiff) {
				t.Helper()
				require.Len(t, unified.Hunks, 1, "should have exactly one hunk")

				hunk := unified.Hunks[0]
				deleteFound := false

				for _, line := range hunk.Lines {
					if strings.Contains(line.Content, "line 3") {
						require.Equal(t, udiff.Delete, line.Kind, "deleted line should be marked as delete")
						deleteFound = true
					}
				}

				require.True(t, deleteFound, "should find the deleted line")
			},
		},
		{
			name:         "line replacement",
			before:       "line 1\nline 2\nline 3\nline 4\nline 5\n",
			after:        "line 1\nline 2\nREPLACED LINE\nline 4\nline 5\n",
			contextLines: 3,
			checkFunc: func(t *testing.T, unified udiff.UnifiedDiff) {
				t.Helper()
				require.Len(t, unified.Hunks, 1, "should have exactly one hunk")

				hunk := unified.Hunks[0]
				deleteFound := false
				insertFound := false

				for _, line := range hunk.Lines {
					if strings.Contains(line.Content, "line 3") {
						require.Equal(t, udiff.Delete, line.Kind, "old line should be marked as delete")
						deleteFound = true
					}
					if strings.Contains(line.Content, "REPLACED LINE") {
						require.Equal(t, udiff.Insert, line.Kind, "new line should be marked as insert")
						insertFound = true
					}
				}

				require.True(t, deleteFound, "should find the deleted line")
				require.True(t, insertFound, "should find the inserted line")
			},
		},
		{
			name:         "empty diff",
			before:       "line 1\nline 2\nline 3\n",
			after:        "line 1\nline 2\nline 3\n",
			contextLines: 3,
			checkFunc: func(t *testing.T, unified udiff.UnifiedDiff) {
				t.Helper()
				require.Empty(t, unified.Hunks, "identical files should have no hunks")
			},
		},
		{
			name:         "multiple changes in same hunk",
			before:       "line 1\nline 2\nline 3\nline 4\nline 5\n",
			after:        "line 1\nINSERT 1\nline 2\nline 3\nINSERT 2\nline 4\nline 5\n",
			contextLines: 3,
			checkFunc: func(t *testing.T, unified udiff.UnifiedDiff) {
				t.Helper()
				require.Len(t, unified.Hunks, 1, "should have one hunk for nearby changes")

				hunk := unified.Hunks[0]
				insertCount := 0

				for _, line := range hunk.Lines {
					if line.Kind == udiff.Insert {
						insertCount++
					}
				}

				require.Equal(t, 2, insertCount, "should have 2 insertions")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diffs := generateMyersDiff(tt.before, tt.after)
			unified := convertMyersToUnifiedDiff(diffs, "test.txt", "test.txt", tt.contextLines)

			tt.checkFunc(t, unified)
		})
	}
}

func TestGenerateDiff(t *testing.T) {
	t.Parallel()

	before := "line 1\nline 2\nline 3\nline 4\nline 5\n"
	after := "line 1\nline 2\nNEW LINE\nline 3\nline 4\nline 5\n"

	diffStr, additions, removals := GenerateDiff(before, after, "test.txt")

	require.Contains(t, diffStr, "--- a/test.txt")
	require.Contains(t, diffStr, "+++ b/test.txt")
	require.Contains(t, diffStr, "+NEW LINE")
	require.NotContains(t, diffStr, "-line 2\n+line 2", "line 2 should not show as changed")

	require.Equal(t, 1, additions, "should have 1 addition")
	require.Equal(t, 0, removals, "should have 0 removals")
}
