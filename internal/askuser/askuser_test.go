package askuser

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	t.Parallel()
	svc := NewService()
	require.NotNil(t, svc)
}

func TestRequestResponseFlow(t *testing.T) {
	t.Parallel()
	svc := NewService()

	questions := []Question{
		{
			Question: "Which framework?",
			Header:   "Framework",
			Options: []QuestionOption{
				{Label: "React", Description: "A JavaScript library"},
				{Label: "Vue", Description: "Progressive framework"},
			},
			MultiSelect: false,
		},
	}

	// Subscribe BEFORE making the request to ensure we catch the event
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	subCh := svc.Subscribe(ctx)

	var wg sync.WaitGroup
	var response *AskUserResponse
	var reqErr error

	// Make the request in a goroutine since it blocks
	wg.Add(1)
	go func() {
		defer wg.Done()
		response, reqErr = svc.Request(CreateAskUserRequest{
			SessionID:  "test-session",
			ToolCallID: "test-tool-call",
			Questions:  questions,
		})
	}()

	// Wait for the event and respond
	select {
	case event := <-subCh:
		svc.Respond(event.Payload.ID, AskUserResponse{
			RequestID: event.Payload.ID,
			Answers: []Answer{
				{
					QuestionIndex:   0,
					SelectedIndex:   1,
					SelectedIndices: []int{1},
				},
			},
		})
	case <-ctx.Done():
		t.Fatal("timeout waiting for request event")
	}

	wg.Wait()

	require.NoError(t, reqErr)
	require.NotNil(t, response)
	assert.False(t, response.Cancelled)
	require.Len(t, response.Answers, 1)
	assert.Equal(t, 1, response.Answers[0].SelectedIndex)
}

func TestCancel(t *testing.T) {
	t.Parallel()
	svc := NewService().(*askUserService)

	// Create a pending request channel
	respCh := make(chan AskUserResponse, 1)
	svc.pendingRequests.Set("test-id", respCh)

	// Cancel it
	svc.Cancel("test-id")

	select {
	case resp := <-respCh:
		assert.True(t, resp.Cancelled)
		assert.Equal(t, "test-id", resp.RequestID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected response on channel")
	}
}

func TestCancelNonExistent(t *testing.T) {
	t.Parallel()
	svc := NewService()

	// Should not panic when cancelling non-existent request
	svc.Cancel("non-existent-id")
}

func TestRespondNonExistent(t *testing.T) {
	t.Parallel()
	svc := NewService()

	// Should not panic when responding to non-existent request
	svc.Respond("non-existent-id", AskUserResponse{
		Answers: []Answer{{SelectedIndex: 0}},
	})
}

func TestMultiSelectResponse(t *testing.T) {
	t.Parallel()
	svc := NewService()

	questions := []Question{
		{
			Question: "Select features",
			Header:   "Features",
			Options: []QuestionOption{
				{Label: "TypeScript"},
				{Label: "ESLint"},
				{Label: "Prettier"},
			},
			MultiSelect: true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	subCh := svc.Subscribe(ctx)

	var wg sync.WaitGroup
	var response *AskUserResponse
	var reqErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		response, reqErr = svc.Request(CreateAskUserRequest{
			SessionID:  "test-session",
			ToolCallID: "test-tool-call",
			Questions:  questions,
		})
	}()

	select {
	case event := <-subCh:
		svc.Respond(event.Payload.ID, AskUserResponse{
			RequestID: event.Payload.ID,
			Answers: []Answer{
				{
					QuestionIndex:   0,
					SelectedIndices: []int{0, 2}, // TypeScript and Prettier
				},
			},
		})
	case <-ctx.Done():
		t.Fatal("timeout")
	}

	wg.Wait()

	require.NoError(t, reqErr)
	require.Len(t, response.Answers, 1)
	assert.Equal(t, []int{0, 2}, response.Answers[0].SelectedIndices)
}

func TestOtherResponse(t *testing.T) {
	t.Parallel()
	svc := NewService()

	questions := []Question{
		{
			Question: "Which editor?",
			Header:   "Editor",
			Options: []QuestionOption{
				{Label: "VS Code"},
				{Label: "Vim"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	subCh := svc.Subscribe(ctx)

	var wg sync.WaitGroup
	var response *AskUserResponse
	var reqErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		response, reqErr = svc.Request(CreateAskUserRequest{
			SessionID:  "test-session",
			ToolCallID: "test-tool-call",
			Questions:  questions,
		})
	}()

	select {
	case event := <-subCh:
		svc.Respond(event.Payload.ID, AskUserResponse{
			RequestID: event.Payload.ID,
			Answers: []Answer{
				{
					QuestionIndex: 0,
					IsOther:       true,
					OtherText:     "Emacs",
				},
			},
		})
	case <-ctx.Done():
		t.Fatal("timeout")
	}

	wg.Wait()

	require.NoError(t, reqErr)
	require.Len(t, response.Answers, 1)
	assert.True(t, response.Answers[0].IsOther)
	assert.Equal(t, "Emacs", response.Answers[0].OtherText)
}

func TestMultipleQuestions(t *testing.T) {
	t.Parallel()
	svc := NewService()

	questions := []Question{
		{
			Question: "Framework?",
			Header:   "Framework",
			Options: []QuestionOption{
				{Label: "React"},
				{Label: "Vue"},
			},
		},
		{
			Question: "Language?",
			Header:   "Language",
			Options: []QuestionOption{
				{Label: "TypeScript"},
				{Label: "JavaScript"},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	subCh := svc.Subscribe(ctx)

	var wg sync.WaitGroup
	var response *AskUserResponse
	var reqErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		response, reqErr = svc.Request(CreateAskUserRequest{
			SessionID:  "test-session",
			ToolCallID: "test-tool-call",
			Questions:  questions,
		})
	}()

	select {
	case event := <-subCh:
		assert.Len(t, event.Payload.Questions, 2)
		svc.Respond(event.Payload.ID, AskUserResponse{
			RequestID: event.Payload.ID,
			Answers: []Answer{
				{QuestionIndex: 0, SelectedIndex: 0, SelectedIndices: []int{0}},
				{QuestionIndex: 1, SelectedIndex: 0, SelectedIndices: []int{0}},
			},
		})
	case <-ctx.Done():
		t.Fatal("timeout")
	}

	wg.Wait()

	require.NoError(t, reqErr)
	require.Len(t, response.Answers, 2)
	assert.Equal(t, 0, response.Answers[0].QuestionIndex)
	assert.Equal(t, 1, response.Answers[1].QuestionIndex)
}
