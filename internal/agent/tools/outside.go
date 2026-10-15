package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
)

// resolvePath trả đường dẫn tuyệt đối của p sau khi giải mọi symlink. Path
// chưa tồn tại thì giải phần cha gần nhất đang tồn tại rồi ghép phần còn lại,
// để một symlink ở giữa đường vẫn bị nhận ra.
func resolvePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	rest := ""
	cur := abs
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}

// outsideWorkingDir trả đường dẫn đã giải symlink của path và cho biết nó có
// nằm ngoài workingDir không. Symlink được giải ở cả hai phía: symlink trong
// dự án trỏ ra ngoài (vd docs -> ~/.ssh) vẫn là "ngoài".
func outsideWorkingDir(workingDir, path string) (string, bool) {
	resolvedDir := resolvePath(workingDir)
	resolved := resolvePath(path)
	rel, err := filepath.Rel(resolvedDir, resolved)
	if err != nil {
		return resolved, true
	}
	return resolved, rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// requestOutsideAccess xin quyền khi một tool đọc chạm tới path ngoài thư mục
// làm việc. Trả (nil, nil) khi path nằm trong dự án hoặc đã được cho phép; trả
// response từ chối khi người dùng không cho.
func requestOutsideAccess(ctx context.Context, permissions permission.Service, call fantasy.ToolCall, workingDir, path, toolName, action, what string, params any) (*fantasy.ToolResponse, error) {
	resolved, outside := outsideWorkingDir(workingDir, path)
	if !outside {
		return nil, nil
	}
	if permissions == nil {
		resp := fantasy.NewTextErrorResponse(fmt.Sprintf("%s outside the working directory is not allowed: %s", what, resolved))
		return &resp, nil
	}
	sessionID := GetSessionFromContext(ctx)
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required for accessing paths outside working directory")
	}
	granted, err := permissions.Request(ctx, permission.CreatePermissionRequest{
		SessionID:   sessionID,
		Path:        resolved,
		ToolCallID:  call.ID,
		ToolName:    toolName,
		Action:      action,
		Description: fmt.Sprintf("%s outside working directory: %s", what, resolved),
		Params:      params,
	})
	if err != nil {
		return nil, err
	}
	if !granted {
		resp := NewPermissionDeniedResponse()
		return &resp, nil
	}
	return nil, nil
}
