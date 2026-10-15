package db

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/crush/internal/lock"
	"github.com/stretchr/testify/require"
)

// Thông báo khi thư mục dữ liệu đang bị tiến trình khác giữ phải gọi sản phẩm là
// agent-cli (người dùng thấy câu này khi mở hai bản cùng một thư mục).
func TestErrDataDirLocked_MessageNamesAgentCLI(t *testing.T) {
	t.Cleanup(ResetPool)

	dataDir := t.TempDir()
	release, err := lock.TryFile(filepath.Join(dataDir, dataDirLockFile))
	require.NoError(t, err)
	t.Cleanup(release)

	_, err = Connect(context.Background(), dataDir, WithDataDirLock(true))
	require.ErrorIs(t, err, ErrDataDirLocked)
	require.Contains(t, err.Error(), "another agent-cli process")
	require.NotContains(t, err.Error(), "another crush process")
}
