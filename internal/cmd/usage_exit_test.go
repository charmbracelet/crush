package cmd

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// Mọi lệnh, không chỉ run/send, trả lỗi cú pháp dạng usageError (mã thoát 2):
// cờ lạ, sai số tham số, lệnh con sai tên, vi phạm nhóm cờ.
func TestUsageErrorsForEveryCommand(t *testing.T) {
	t.Parallel()
	newTree := func() *cobra.Command {
		root := &cobra.Command{Use: "root", RunE: func(*cobra.Command, []string) error { return nil }}
		root.SilenceErrors, root.SilenceUsage = true, true
		ps := &cobra.Command{Use: "ps", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return nil }}
		ps.Flags().Bool("all", false, "")
		group := &cobra.Command{Use: "session"}
		group.AddCommand(&cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }})
		excl := &cobra.Command{Use: "pick", RunE: func(*cobra.Command, []string) error { return nil }}
		excl.Flags().Bool("a", false, "")
		excl.Flags().Bool("b", false, "")
		excl.MarkFlagsMutuallyExclusive("a", "b")
		root.AddCommand(ps, group, excl)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		markUsageErrors(root)
		return root
	}
	for _, args := range [][]string{
		{"ps", "--bogus"},
		{"ps", "extra-arg"},
		{"session", "nope"},
		{"pick", "--a", "--b"},
	} {
		root := newTree()
		root.SetArgs(args)
		err := root.Execute()
		require.Errorf(t, err, "%v", args)
		require.Truef(t, isUsageError(err), "%v: %v", args, err)
	}
	// Lệnh chạy được thì không phải lỗi cú pháp.
	root := newTree()
	root.SetArgs([]string{"ps", "--all"})
	require.NoError(t, root.Execute())
	require.False(t, isUsageError(io.ErrUnexpectedEOF))
}

// Workspace tái dùng mang YOLO khác ý người gọi thì phải có cảnh báo, theo
// cả hai chiều; khớp thì im lặng.
func TestYoloMismatchWarning(t *testing.T) {
	t.Parallel()
	require.Empty(t, yoloMismatchWarning(false, false))
	require.Empty(t, yoloMismatchWarning(true, true))
	require.Contains(t, yoloMismatchWarning(false, true), "already in YOLO mode")
	require.Contains(t, yoloMismatchWarning(true, false), "--yolo was ignored")
}
