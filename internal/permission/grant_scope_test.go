package permission

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// grantFirst answers the first prompt the service raises, and reports the
// request it was answering.
func grantFirst(t *testing.T, svc Service, persistent bool) PermissionRequest {
	t.Helper()
	events := svc.Subscribe(t.Context())
	select {
	case ev := <-events:
		if persistent {
			svc.GrantPersistent(ev.Payload)
		} else {
			svc.Grant(ev.Payload)
		}
		return ev.Payload
	case <-time.After(5 * time.Second):
		t.Fatal("no permission request was published")
		return PermissionRequest{}
	}
}

// A session grant taken on one command must not cover a different one. The
// working directory is the same for every shell command in a session, so
// without the command in the key, approving `ls` once would stand in for
// approving everything afterwards.
func TestSessionGrantDoesNotCoverADifferentCommand(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService("/work", nil)
	events := svc.Subscribe(t.Context())

	benign := make(chan bool, 1)
	go func() {
		ok, _ := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID: "s", ToolCallID: "c1", ToolName: "bash",
			Action: "execute", Description: "ls", Path: "/work",
			GrantKey: "ls",
		})
		benign <- ok
	}()

	select {
	case ev := <-events:
		require.True(t, svc.GrantPersistent(ev.Payload))
	case <-time.After(5 * time.Second):
		t.Fatal("no prompt for the benign command")
	}
	require.True(t, <-benign)

	// The same command again rides the grant and must not prompt.
	repeat, err := svc.Request(t.Context(), CreatePermissionRequest{
		SessionID: "s", ToolCallID: "c2", ToolName: "bash",
		Action: "execute", Description: "ls", Path: "/work",
		GrantKey: "ls",
	})
	require.NoError(t, err)
	require.True(t, repeat, "the approved command should not prompt again")

	// A different command in the same session and directory must prompt.
	dangerous := make(chan bool, 1)
	go func() {
		ok, _ := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID: "s", ToolCallID: "c3", ToolName: "bash",
			Action: "execute", Description: "sudo rm -rf /", Path: "/work",
			GrantKey: "sudo rm -rf /", Danger: "sudo",
		})
		dangerous <- ok
	}()

	select {
	case ev := <-events:
		require.Equal(t, "sudo", ev.Payload.Danger)
		svc.Deny(ev.Payload)
	case granted := <-dangerous:
		t.Fatalf("a different command rode the earlier grant without prompting (granted=%v)", granted)
	case <-time.After(5 * time.Second):
		t.Fatal("neither prompted nor returned")
	}
}

// A resolution is recorded against the request the service issued, not
// against whatever the answering client claims it was answering.
func TestGrantIgnoresFieldsSuppliedByTheAnswer(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService("/work", nil)
	events := svc.Subscribe(t.Context())

	done := make(chan bool, 1)
	go func() {
		ok, _ := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID: "s", ToolCallID: "c1", ToolName: "view",
			Action: "read", Description: "read notes.txt", Path: "/work/notes.txt",
		})
		done <- ok
	}()

	var issued PermissionRequest
	select {
	case ev := <-events:
		issued = ev.Payload
	case <-time.After(5 * time.Second):
		t.Fatal("no prompt")
	}

	// Answer with the right ID but a wholly different, far broader claim
	// about what was being asked.
	forged := issued
	forged.ToolName = "bash"
	forged.Action = "execute"
	forged.Path = "/work"
	forged.GrantKey = ""
	require.True(t, svc.GrantPersistent(forged))
	require.True(t, <-done)

	// The forged shape must not have been recorded, so a bash command in
	// that directory still prompts.
	bash := make(chan bool, 1)
	go func() {
		ok, _ := svc.Request(t.Context(), CreatePermissionRequest{
			SessionID: "s", ToolCallID: "c2", ToolName: "bash",
			Action: "execute", Description: "sudo rm -rf /", Path: "/work",
			GrantKey: "sudo rm -rf /",
		})
		bash <- ok
	}()

	select {
	case ev := <-events:
		svc.Deny(ev.Payload)
	case granted := <-bash:
		t.Fatalf("a grant was recorded from client-supplied fields (granted=%v)", granted)
	case <-time.After(5 * time.Second):
		t.Fatal("neither prompted nor returned")
	}
}

// The outcome notification names the tool call the service asked about, so a
// mis-stated answer cannot dismiss a different call in another client's UI.
func TestNotificationUsesTheStoredToolCallID(t *testing.T) {
	t.Parallel()

	svc := NewPermissionService("/work", nil)
	events := svc.Subscribe(t.Context())
	notes := svc.SubscribeNotifications(t.Context())

	go func() {
		svc.Request(t.Context(), CreatePermissionRequest{
			SessionID: "s", ToolCallID: "real-call", ToolName: "view",
			Action: "read", Description: "read x", Path: "/work/x",
		})
	}()

	var issued PermissionRequest
	select {
	case ev := <-events:
		issued = ev.Payload
	case <-time.After(5 * time.Second):
		t.Fatal("no prompt")
	}
	// Drain the request notification published alongside the prompt.
	select {
	case <-notes:
	case <-time.After(5 * time.Second):
		t.Fatal("no request notification")
	}

	forged := issued
	forged.ToolCallID = "someone-elses-call"
	require.True(t, svc.Grant(forged))

	select {
	case n := <-notes:
		require.Equal(t, "real-call", n.Payload.ToolCallID,
			"the notification must name the call the service asked about")
	case <-time.After(5 * time.Second):
		t.Fatal("no resolution notification")
	}
}
