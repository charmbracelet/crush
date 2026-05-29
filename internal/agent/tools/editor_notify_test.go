package tools

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/taigrr/crush/internal/editor"
)

// recordingBridge captures every call so tests can assert on them.
type recordingBridge struct {
	mu          sync.Mutex
	available   bool
	flashCalls  []flashCall
	notifyCalls []string
	flashErr    error
	notifyErr   error
}

type flashCall struct {
	path  string
	start int
	end   int
}

func (r *recordingBridge) Available() bool { return r.available }
func (r *recordingBridge) Context(context.Context) (editor.EditorContext, error) {
	return editor.EditorContext{}, editor.ErrUnavailable
}

func (r *recordingBridge) ShowLocations(context.Context, string, []editor.Location) error {
	return editor.ErrUnavailable
}

func (r *recordingBridge) FlashEdit(_ context.Context, path string, start, end int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flashCalls = append(r.flashCalls, flashCall{path, start, end})
	return r.flashErr
}

func (r *recordingBridge) NotifyFileChanged(_ context.Context, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifyCalls = append(r.notifyCalls, path)
	return r.notifyErr
}
func (r *recordingBridge) Close() error { return nil }

func TestNotifyEditor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bridge     func() *recordingBridge
		path       string
		oldContent string
		newContent string
		wantFlash  []flashCall
		wantNotify []string
	}{
		{
			name:       "available bridge gets flash + notify",
			bridge:     func() *recordingBridge { return &recordingBridge{available: true} },
			path:       "/tmp/x.go",
			oldContent: "a\nb\nc\n",
			newContent: "a\nB\nc\n",
			wantFlash:  []flashCall{{path: "/tmp/x.go", start: 1, end: 2}},
			wantNotify: []string{"/tmp/x.go"},
		},
		{
			name:       "no-op when contents identical",
			bridge:     func() *recordingBridge { return &recordingBridge{available: true} },
			path:       "/tmp/x.go",
			oldContent: "same",
			newContent: "same",
			wantFlash:  nil,
			wantNotify: []string{"/tmp/x.go"},
		},
		{
			name:       "unavailable bridge skips both calls",
			bridge:     func() *recordingBridge { return &recordingBridge{available: false} },
			path:       "/tmp/x.go",
			oldContent: "a\n",
			newContent: "b\n",
			wantFlash:  nil,
			wantNotify: nil,
		},
		{
			name: "errors from bridge are swallowed",
			bridge: func() *recordingBridge {
				return &recordingBridge{
					available: true,
					flashErr:  errors.New("flash boom"),
					notifyErr: errors.New("notify boom"),
				}
			},
			path:       "/tmp/x.go",
			oldContent: "",
			newContent: "new\n",
			wantFlash:  []flashCall{{path: "/tmp/x.go", start: 0, end: 1}},
			wantNotify: []string{"/tmp/x.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := tt.bridge()
			notifyEditor(t.Context(), b, tt.path, tt.oldContent, tt.newContent)
			require.Equal(t, tt.wantFlash, b.flashCalls)
			require.Equal(t, tt.wantNotify, b.notifyCalls)
		})
	}
}

// nil bridge must not panic (covers the early-return guard).
func TestNotifyEditor_NilBridge(t *testing.T) {
	t.Parallel()
	require.NotPanics(t, func() {
		notifyEditor(t.Context(), nil, "/tmp/x.go", "old", "new")
	})
}
