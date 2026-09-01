package permission

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubjectScopedPersistentGrants(t *testing.T) {
	service := NewPermissionService("/tmp", false, []string{})

	req := CreatePermissionRequest{
		SessionID:   "subject-session",
		ToolCallID:  "call-build",
		ToolName:    "bash",
		Description: "Execute command: swift build",
		Action:      "execute",
		Path:        "/tmp",
		Subject:     "swift build",
	}

	events := service.Subscribe(t.Context())

	var granted bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		granted, _ = service.Request(t.Context(), req)
	}()

	event := <-events
	require.Equal(t, "swift build", event.Payload.Subject)
	require.True(t, service.GrantPersistent(event.Payload))
	<-done
	require.True(t, granted, "first request should be granted")

	// The same subject auto-approves even with different trailing args;
	// a matching grant publishes a notification but never a new prompt.
	same := req
	same.ToolCallID = "call-build-2"
	same.Description = "Execute command: swift build --explicit"
	ok, err := service.Request(t.Context(), same)
	require.NoError(t, err)
	assert.True(t, ok, "same subject should auto-approve")

	// A different subject must still prompt. With no respondent, the
	// request times out, which is how we prove no silent grant happened.
	other := CreatePermissionRequest{
		SessionID:   "subject-session",
		ToolCallID:  "call-rm",
		ToolName:    "bash",
		Description: "Execute command: rm -rf /tmp/x",
		Action:      "execute",
		Path:        "/tmp",
		Subject:     "rm -rf",
	}
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	prompted := make(chan bool, 1)
	go func() {
		g, err := service.Request(ctx, other)
		prompted <- g && err == nil
	}()

	select {
	case ev := <-events:
		assert.Equal(t, "rm -rf", ev.Payload.Subject, "prompt should carry the new subject")
		<-prompted
	case <-time.After(3 * time.Second):
		t.Fatal("different subject should have prompted")
	}
}

func TestEmptySubjectKeepsLegacyBehavior(t *testing.T) {
	service := NewPermissionService("/tmp", false, []string{})

	req := CreatePermissionRequest{
		SessionID:   "legacy-session",
		ToolCallID:  "call-legacy",
		ToolName:    "file_tool",
		Description: "Edit file",
		Action:      "write",
		Path:        "/tmp/notes.md",
	}

	events := service.Subscribe(t.Context())

	var granted bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		granted, _ = service.Request(t.Context(), req)
	}()

	event := <-events
	require.True(t, service.GrantPersistent(event.Payload))
	<-done
	require.True(t, granted)

	same := req
	same.ToolCallID = "call-legacy-2"
	ok, err := service.Request(t.Context(), same)
	require.NoError(t, err)
	assert.True(t, ok, "empty subject should behave like the old tool+action+path key")
}
