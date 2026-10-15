package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

// recordingPermissions ghi lại mọi yêu cầu quyền và trả lời theo grant.
type recordingPermissions struct {
	mockPermissionService
	grant bool

	mu       sync.Mutex
	requests []permission.CreatePermissionRequest
}

func newRecordingPermissions(grant bool) *recordingPermissions {
	return &recordingPermissions{
		mockPermissionService: mockPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()},
		grant:                 grant,
	}
}

func (r *recordingPermissions) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
	return r.grant, nil
}

func (r *recordingPermissions) seen() []permission.CreatePermissionRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]permission.CreatePermissionRequest(nil), r.requests...)
}

// outsideFixture dựng một dự án và một thư mục "nhà" ngoài dự án chứa tệp
// giống tệp env có key.
func outsideFixture(t *testing.T) (project, outside string) {
	t.Helper()
	root := t.TempDir()
	project = filepath.Join(root, "project")
	outside = filepath.Join(root, "home", ".config", "agent-cli")
	require.NoError(t, os.MkdirAll(project, 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.go"), []byte("package main // API_KEY not here\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "env"), []byte("AGENT_CLI_API_KEY=secret-value\n"), 0o600))
	return project, outside
}

func runTool(t *testing.T, tool fantasy.AgentTool, name string, params any) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	ctx := context.WithValue(t.Context(), SessionIDContextKey, "session-1")
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "call-1", Name: name, Input: string(input)})
	require.NoError(t, err)
	return resp
}

func TestOutsideWorkingDir(t *testing.T) {
	t.Parallel()
	project, outside := outsideFixture(t)
	require.NoError(t, os.Symlink(outside, filepath.Join(project, "docs")))
	link := filepath.Join(filepath.Dir(project), "project-link")
	require.NoError(t, os.Symlink(project, link))

	cases := []struct {
		name    string
		path    string
		outside bool
	}{
		{"file in project", filepath.Join(project, "main.go"), false},
		{"missing file in project", filepath.Join(project, "new", "file.go"), false},
		{"project root", project, false},
		{"absolute outside", filepath.Join(outside, "env"), true},
		{"dot-dot traversal", filepath.Join(project, "..", "home"), true},
		{"symlink in project pointing outside", filepath.Join(project, "docs", "env"), true},
		{"file named with leading dots stays inside", filepath.Join(project, "..hidden"), false},
		{"project reached through a symlink", filepath.Join(link, "main.go"), false},
	}
	for _, tc := range cases {
		_, got := outsideWorkingDir(project, tc.path)
		require.Equalf(t, tc.outside, got, "%s: %s", tc.name, tc.path)
	}
	// Thư mục làm việc khai bằng symlink vẫn nhận ra tệp thật bên trong.
	_, got := outsideWorkingDir(link, filepath.Join(project, "main.go"))
	require.False(t, got)
}

// grep đọc nội dung tệp: tìm trong thư mục ngoài dự án (nơi để tệp env chứa
// key) phải hỏi quyền; từ chối thì không trả nội dung nào.
func TestGrepAsksBeforeSearchingOutsideWorkingDir(t *testing.T) {
	t.Parallel()
	project, outside := outsideFixture(t)

	denied := newRecordingPermissions(false)
	tool := NewGrepTool(denied, project, config.ToolGrep{})
	resp := runTool(t, tool, GrepToolName, GrepParams{Pattern: "API_KEY", Path: outside})
	require.True(t, resp.IsError)
	require.NotContains(t, resp.Content, "secret-value")
	reqs := denied.seen()
	require.Len(t, reqs, 1)
	require.Equal(t, GrepToolName, reqs[0].ToolName)
	require.Equal(t, resolvePath(outside), reqs[0].Path)

	granted := newRecordingPermissions(true)
	tool = NewGrepTool(granted, project, config.ToolGrep{})
	resp = runTool(t, tool, GrepToolName, GrepParams{Pattern: "API_KEY", Path: outside})
	require.False(t, resp.IsError, resp.Content)
	require.Contains(t, resp.Content, "secret-value")

	// Trong dự án thì không hỏi.
	inside := newRecordingPermissions(false)
	tool = NewGrepTool(inside, project, config.ToolGrep{})
	resp = runTool(t, tool, GrepToolName, GrepParams{Pattern: "API_KEY"})
	require.False(t, resp.IsError, resp.Content)
	require.Empty(t, inside.seen())
}

// glob liệt kê tên tệp: ngoài dự án phải hỏi như ls; symlink trong dự án trỏ
// ra ngoài cũng tính là ngoài.
func TestGlobAsksBeforeListingOutsideWorkingDir(t *testing.T) {
	t.Parallel()
	project, outside := outsideFixture(t)
	require.NoError(t, os.Symlink(outside, filepath.Join(project, "docs")))

	for _, path := range []string{outside, "docs"} {
		denied := newRecordingPermissions(false)
		tool := NewGlobTool(denied, project, config.ToolGlob{})
		resp := runTool(t, tool, GlobToolName, GlobParams{Pattern: "*", Path: path})
		require.Truef(t, resp.IsError, "path %q", path)
		require.NotContains(t, resp.Content, "env")
		require.Lenf(t, denied.seen(), 1, "path %q", path)
		require.Equal(t, GlobToolName, denied.seen()[0].ToolName)
	}

	inside := newRecordingPermissions(false)
	tool := NewGlobTool(inside, project, config.ToolGlob{})
	resp := runTool(t, tool, GlobToolName, GlobParams{Pattern: "*.go"})
	require.False(t, resp.IsError, resp.Content)
	require.Contains(t, resp.Content, "main.go")
	require.Empty(t, inside.seen())
}

// view và ls trước đây so đường dẫn mà không giải symlink: tệp trong dự án
// là symlink trỏ ra ngoài được đọc không hỏi.
func TestViewAndLsAskForSymlinkPointingOutside(t *testing.T) {
	t.Parallel()
	project, outside := outsideFixture(t)
	require.NoError(t, os.Symlink(outside, filepath.Join(project, "docs")))

	viewPerms := newRecordingPermissions(false)
	view := NewViewTool(nil, viewPerms, mockFileTracker{}, nil, project)
	resp := runTool(t, view, ViewToolName, ViewParams{FilePath: filepath.Join("docs", "env")})
	require.True(t, resp.IsError)
	require.NotContains(t, resp.Content, "secret-value")
	require.Len(t, viewPerms.seen(), 1)
	require.True(t, strings.HasSuffix(viewPerms.seen()[0].Path, filepath.Join("agent-cli", "env")),
		"the dialog must show the real target, got %q", viewPerms.seen()[0].Path)

	lsPerms := newRecordingPermissions(false)
	ls := NewLsTool(lsPerms, project, config.ToolLs{})
	resp = runTool(t, ls, LSToolName, LSParams{Path: "docs"})
	require.True(t, resp.IsError)
	require.Len(t, lsPerms.seen(), 1)
}

// Pattern của grep do model viết không bao giờ thành cờ của rg: "--pre=<x>"
// từng khiến rg chạy <x> trên mọi tệp mà không hỏi quyền.
func TestGrepPatternIsNeverARipgrepFlag(t *testing.T) {
	// Không parallel: useRealRg đổi biến cấp gói (getRg trả rỗng khi test).
	useRealRg(t)
	project, _ := outsideFixture(t)
	marker := filepath.Join(t.TempDir(), "pwned")
	script := filepath.Join(project, "x.sh")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\ntouch '"+marker+"'\ncat \"$1\"\n"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, "flags.txt"), []byte("a line with --pre=x and -v inside\n"), 0o644))

	for _, pattern := range []string{"--pre=" + script, "--pre=./x.sh", "-v"} {
		perms := newRecordingPermissions(false)
		resp := runTool(t, NewGrepTool(perms, project, config.ToolGrep{}), GrepToolName, GrepParams{Pattern: pattern})
		require.False(t, resp.IsError, resp.Content)
		_, err := os.Stat(marker)
		require.True(t, os.IsNotExist(err), "pattern %q ran a program", pattern)
		require.Empty(t, perms.seen())
	}
	resp := runTool(t, NewGrepTool(newRecordingPermissions(false), project, config.ToolGrep{}), GrepToolName, GrepParams{Pattern: "-v", LiteralText: true})
	require.Contains(t, resp.Content, "flags.txt", "a dash pattern is searched as text")
}

// Phần chữ đầu của pattern dời gốc duyệt của glob: "../x/*" hoặc "docs/*"
// (docs là symlink ra ngoài) phải hỏi quyền như khi path nằm ngoài.
func TestGlobPatternPrefixOutsideAsksPermission(t *testing.T) {
	t.Parallel()
	project, outside := outsideFixture(t)
	require.NoError(t, os.Symlink(outside, filepath.Join(project, "docs")))
	rel, err := filepath.Rel(project, outside)
	require.NoError(t, err)

	for _, pattern := range []string{filepath.ToSlash(rel) + "/*", "docs/*"} {
		denied := newRecordingPermissions(false)
		resp := runTool(t, NewGlobTool(denied, project, config.ToolGlob{}), GlobToolName, GlobParams{Pattern: pattern})
		require.Truef(t, resp.IsError, "pattern %q", pattern)
		require.NotContains(t, resp.Content, "env")
		require.Lenf(t, denied.seen(), 1, "pattern %q", pattern)
	}
	inside := newRecordingPermissions(false)
	resp := runTool(t, NewGlobTool(inside, project, config.ToolGlob{}), GlobToolName, GlobParams{Pattern: "./*.go"})
	require.Contains(t, resp.Content, "main.go")
	require.Empty(t, inside.seen())
}

// Nhánh dự phòng của grep (khi rg lỗi hoặc không có) không theo symlink tệp
// ra ngoài dự án.
func TestGrepFallbackSkipsSymlinks(t *testing.T) {
	t.Parallel()
	project, outside := outsideFixture(t)
	require.NoError(t, os.Symlink(filepath.Join(outside, "env"), filepath.Join(project, "notes.txt")))
	matches, err := searchFilesWithRegex("API_KEY", project, "")
	require.NoError(t, err)
	for _, m := range matches {
		require.NotContains(t, m.lineText, "secret-value", m.path)
	}
	require.NotEmpty(t, matches, "the real file in the project still matches")
}
