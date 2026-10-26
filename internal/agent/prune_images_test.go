package agent

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func geminiModel() Model {
	return Model{
		CatwalkCfg: catwalk.Model{SupportsImages: true},
		ModelCfg:   config.SelectedModel{Provider: string(catwalk.InferenceProviderGemini)},
	}
}

func anthropicModel() Model {
	return Model{
		CatwalkCfg: catwalk.Model{SupportsImages: true},
		ModelCfg:   config.SelectedModel{Provider: string(catwalk.InferenceProviderAnthropic)},
	}
}

func vertexModel() Model {
	return Model{
		CatwalkCfg: catwalk.Model{SupportsImages: true},
		ModelCfg:   config.SelectedModel{Provider: string(catwalk.InferenceProviderVertexAI)},
	}
}

func imagePart(name string) fantasy.FilePart {
	return fantasy.FilePart{
		Filename:  name,
		Data:      []byte("fake-image-data"),
		MediaType: "image/png",
	}
}

func textFilePart(name string) fantasy.FilePart {
	return fantasy.FilePart{
		Filename:  name,
		Data:      []byte("text content"),
		MediaType: "text/plain",
	}
}

func userMsgWithImages(text string, images ...fantasy.FilePart) fantasy.Message {
	parts := []fantasy.MessagePart{fantasy.TextPart{Text: text}}
	for _, img := range images {
		parts = append(parts, img)
	}
	return fantasy.Message{Role: fantasy.MessageRoleUser, Content: parts}
}

func assistantMsg(text string) fantasy.Message {
	return fantasy.Message{
		Role:    fantasy.MessageRoleAssistant,
		Content: []fantasy.MessagePart{fantasy.TextPart{Text: text}},
	}
}

// --- maxImagesForModel ---

func TestMaxImagesForModel_Gemini(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 10, maxImagesForModel(geminiModel()))
}

func TestMaxImagesForModel_VertexAI(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 10, maxImagesForModel(vertexModel()))
}

func TestMaxImagesForModel_Anthropic(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, maxImagesForModel(anthropicModel()))
}

func TestMaxImagesForModel_OpenAI(t *testing.T) {
	t.Parallel()
	m := Model{
		ModelCfg: config.SelectedModel{Provider: string(catwalk.InferenceProviderOpenAI)},
	}
	assert.Equal(t, 0, maxImagesForModel(m))
}

// --- isImageFilePart ---

func TestIsImageFilePart_ImagePNG(t *testing.T) {
	t.Parallel()
	fp := imagePart("test.png")
	_, ok := isImageFilePart(fp)
	assert.True(t, ok)
}

func TestIsImageFilePart_TextFile(t *testing.T) {
	t.Parallel()
	fp := textFilePart("readme.txt")
	_, ok := isImageFilePart(fp)
	assert.False(t, ok)
}

func TestIsImageFilePart_TextPart(t *testing.T) {
	t.Parallel()
	tp := fantasy.TextPart{Text: "hello"}
	_, ok := isImageFilePart(tp)
	assert.False(t, ok)
}

// --- countImagesInMessages ---

func TestCountImagesInMessages_Empty(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 0, countImagesInMessages(nil))
}

func TestCountImagesInMessages_NoImages(t *testing.T) {
	t.Parallel()
	msgs := []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{
			fantasy.TextPart{Text: "hello"},
		}},
	}
	assert.Equal(t, 0, countImagesInMessages(msgs))
}

func TestCountImagesInMessages_MixedContent(t *testing.T) {
	t.Parallel()
	msgs := []fantasy.Message{
		userMsgWithImages("first", imagePart("a.png"), imagePart("b.png")),
		assistantMsg("ok"),
		userMsgWithImages("second", imagePart("c.png")),
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{
			fantasy.TextPart{Text: "text file"}, textFilePart("readme.txt"),
		}},
	}
	assert.Equal(t, 3, countImagesInMessages(msgs))
}

// --- pruneExcessImages ---

func TestPruneExcessImages_NoLimitProvider(t *testing.T) {
	t.Parallel()
	msgs := []fantasy.Message{
		userMsgWithImages("img1", imagePart("1.png")),
		userMsgWithImages("img2", imagePart("2.png")),
	}
	result := pruneExcessImages(msgs, anthropicModel())
	assert.Equal(t, msgs, result)
}

func TestPruneExcessImages_UnderLimit(t *testing.T) {
	t.Parallel()
	msgs := []fantasy.Message{
		userMsgWithImages("msg1", imagePart("1.png"), imagePart("2.png")),
		assistantMsg("response"),
		userMsgWithImages("msg2", imagePart("3.png")),
	}
	result := pruneExcessImages(msgs, geminiModel())
	// 3 images, limit is 10, should not prune
	assert.Equal(t, 3, countImagesInMessages(result))
}

func TestPruneExcessImages_AtExactLimit(t *testing.T) {
	t.Parallel()
	var msgs []fantasy.Message
	for i := range 10 {
		msgs = append(msgs, userMsgWithImages("msg", imagePart("img.png")))
		if i < 9 {
			msgs = append(msgs, assistantMsg("ok"))
		}
	}
	result := pruneExcessImages(msgs, geminiModel())
	assert.Equal(t, 10, countImagesInMessages(result))
}

func TestPruneExcessImages_OverLimit_PrunesOldest(t *testing.T) {
	t.Parallel()

	// Build 12 images across multiple messages
	msgs := []fantasy.Message{
		userMsgWithImages("old1", imagePart("old-1.png"), imagePart("old-2.png")),
		assistantMsg("noted"),
		userMsgWithImages("old2", imagePart("old-3.png"), imagePart("old-4.png")),
		assistantMsg("ok"),
		userMsgWithImages("mid", imagePart("mid-5.png"), imagePart("mid-6.png")),
		assistantMsg("got it"),
		userMsgWithImages("mid2", imagePart("mid-7.png"), imagePart("mid-8.png")),
		assistantMsg("sure"),
		userMsgWithImages("new1", imagePart("new-9.png"), imagePart("new-10.png")),
		assistantMsg("yes"),
		userMsgWithImages("new2", imagePart("new-11.png"), imagePart("new-12.png")),
	}

	result := pruneExcessImages(msgs, geminiModel())

	// Should now have exactly 10 images (12 - 2 = 10)
	assert.Equal(t, 10, countImagesInMessages(result))

	// The oldest 2 images should be replaced with text placeholders
	// that include the original filename.
	firstUser := result[0]
	require.Len(t, firstUser.Content, 3) // text + 2 placeholders
	tp, ok := fantasy.AsMessagePart[fantasy.TextPart](firstUser.Content[1])
	require.True(t, ok)
	assert.Contains(t, tp.Text, "old-1.png")
	assert.Contains(t, tp.Text, "removed to stay within model limits")

	tp2, ok := fantasy.AsMessagePart[fantasy.TextPart](firstUser.Content[2])
	require.True(t, ok)
	assert.Contains(t, tp2.Text, "old-2.png")
	assert.Contains(t, tp2.Text, "removed to stay within model limits")

	// The newest images should still be present
	lastUser := result[len(result)-1]
	for _, part := range lastUser.Content {
		if fp, ok := fantasy.AsMessagePart[fantasy.FilePart](part); ok {
			assert.Equal(t, "image/png", fp.MediaType)
		}
	}
}

func TestPruneExcessImages_PreservesTextFiles(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{
			fantasy.TextPart{Text: "files"},
			textFilePart("readme.txt"),
			imagePart("old-1.png"),
		}},
		assistantMsg("ok"),
		userMsgWithImages("more images",
			imagePart("2.png"), imagePart("3.png"), imagePart("4.png"),
			imagePart("5.png"), imagePart("6.png"), imagePart("7.png"),
			imagePart("8.png"), imagePart("9.png"), imagePart("10.png"),
			imagePart("11.png"),
		),
	}

	result := pruneExcessImages(msgs, geminiModel())
	assert.Equal(t, 10, countImagesInMessages(result))

	// Text file should still be present in first message
	firstMsg := result[0]
	hasTextFile := false
	for _, part := range firstMsg.Content {
		if fp, ok := fantasy.AsMessagePart[fantasy.FilePart](part); ok {
			if fp.MediaType == "text/plain" {
				hasTextFile = true
			}
		}
	}
	assert.True(t, hasTextFile, "text file attachment should not be pruned")
}

func TestPruneExcessImages_PreservesMessageRolesAndStructure(t *testing.T) {
	t.Parallel()

	msgs := []fantasy.Message{
		userMsgWithImages("q1", imagePart("1.png")),
		assistantMsg("a1"),
		userMsgWithImages("q2", imagePart("2.png")),
		assistantMsg("a2"),
	}

	// Under limit — no changes
	result := pruneExcessImages(msgs, geminiModel())
	require.Len(t, result, 4)
	assert.Equal(t, fantasy.MessageRoleUser, result[0].Role)
	assert.Equal(t, fantasy.MessageRoleAssistant, result[1].Role)
	assert.Equal(t, fantasy.MessageRoleUser, result[2].Role)
	assert.Equal(t, fantasy.MessageRoleAssistant, result[3].Role)
}

func TestPruneExcessImages_VertexAI(t *testing.T) {
	t.Parallel()

	var msgs []fantasy.Message
	for range 12 {
		msgs = append(msgs, userMsgWithImages("img", imagePart("x.png")))
		msgs = append(msgs, assistantMsg("ok"))
	}

	result := pruneExcessImages(msgs, vertexModel())
	assert.Equal(t, 10, countImagesInMessages(result))
}

func TestPruneExcessImages_PreservesProviderOptions(t *testing.T) {
	t.Parallel()

	// Build a message with ProviderOptions set to nil (the zero value)
	// and verify that after pruning the options field is preserved as-is.
	msgs := []fantasy.Message{
		{
			Role:    fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: "hello"}, imagePart("1.png")},
		},
	}

	// Under limit — message should be returned unchanged
	result := pruneExcessImages(msgs, geminiModel())
	assert.Nil(t, result[0].ProviderOptions)
	assert.Len(t, result[0].Content, 2)
}

func TestPruneExcessImages_AllImagesPrunable(t *testing.T) {
	t.Parallel()

	// 15 images, limit 10 → remove 5 oldest
	var msgs []fantasy.Message
	for i := range 15 {
		msgs = append(msgs, userMsgWithImages("img", imagePart("x.png")))
		if i < 14 {
			msgs = append(msgs, assistantMsg("ok"))
		}
	}

	result := pruneExcessImages(msgs, geminiModel())
	assert.Equal(t, 10, countImagesInMessages(result))

	// First 5 user messages should have their image replaced
	pruned := 0
	for _, msg := range result {
		for _, part := range msg.Content {
			if tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				if strings.Contains(tp.Text, "removed to stay within model limits") {
					pruned++
				}
			}
		}
	}
	assert.Equal(t, 5, pruned)
}

// --- Integration tests: DB → preparePrompt → pruneExcessImages pipeline ---

// TestPruneExcessImages_DBPipeline_ExceedsLimit simulates the real bug
// scenario: a session accumulates more images than the provider allows across
// multiple messages stored in the database. We verify that the full pipeline
// (DB messages → preparePrompt → pruneExcessImages) produces a history that
// stays within the provider's image limit.
func TestPruneExcessImages_DBPipeline_ExceedsLimit(t *testing.T) {
	env := testEnv(t)
	agent := &sessionAgent{
		messages: env.messages,
	}

	sess, err := env.sessions.Create(t.Context(), "image-limit-session")
	require.NoError(t, err)

	// Simulate 12 rounds of user-sends-image / assistant-replies,
	// building up more images than Gemini's 10-image limit.
	for i := range 12 {
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: fmt.Sprintf("Here is screenshot %d", i+1)},
				message.BinaryContent{
					Path:     fmt.Sprintf("screenshot-%d.png", i+1),
					MIMEType: "image/png",
					Data:     []byte(fmt.Sprintf("fake-png-data-%d", i+1)),
				},
			},
		})
		require.NoError(t, err)

		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: fmt.Sprintf("I see screenshot %d", i+1)},
			},
		})
		require.NoError(t, err)
	}

	// Load messages from DB (same as sessionAgent.Run does).
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 24) // 12 user + 12 assistant

	// Convert through preparePrompt (DB messages → fantasy messages).
	history, _ := agent.preparePrompt(msgs)

	// Verify: without pruning, history has 12 images (over limit).
	require.Equal(t, 12, countImagesInMessages(history))

	// Apply pruneExcessImages with Gemini model (limit = 10).
	pruned := pruneExcessImages(history, geminiModel())
	assert.Equal(t, 10, countImagesInMessages(pruned))

	// The 10 newest images should be preserved, 2 oldest replaced.
	placeholders := 0
	for _, msg := range pruned {
		for _, part := range msg.Content {
			if tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				if strings.Contains(tp.Text, "removed to stay within model limits") {
					placeholders++
				}
			}
		}
	}
	assert.Equal(t, 2, placeholders)

	// All message roles should still alternate user/assistant correctly.
	// (Skip the first system_reminder injected by preparePrompt.)
	for i := 1; i < len(pruned); i++ {
		if i%2 == 1 { // odd index = user
			assert.Equal(t, fantasy.MessageRoleUser, pruned[i].Role, "index %d should be user", i)
		} else { // even index = assistant
			assert.Equal(t, fantasy.MessageRoleAssistant, pruned[i].Role, "index %d should be assistant", i)
		}
	}
}

// TestPruneExcessImages_DBPipeline_UnderLimit verifies that when images are
// within the limit, nothing is modified.
func TestPruneExcessImages_DBPipeline_UnderLimit(t *testing.T) {
	env := testEnv(t)
	agent := &sessionAgent{
		messages: env.messages,
	}

	sess, err := env.sessions.Create(t.Context(), "under-limit-session")
	require.NoError(t, err)

	// Create 5 messages with images (under Gemini's 10-image limit).
	for i := range 5 {
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: fmt.Sprintf("image %d", i+1)},
				message.BinaryContent{
					Path:     fmt.Sprintf("img-%d.png", i+1),
					MIMEType: "image/png",
					Data:     []byte("fake"),
				},
			},
		})
		require.NoError(t, err)

		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "ok"},
			},
		})
		require.NoError(t, err)
	}

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs)
	require.Equal(t, 5, countImagesInMessages(history))

	pruned := pruneExcessImages(history, geminiModel())
	// Nothing should be removed.
	assert.Equal(t, 5, countImagesInMessages(pruned))
}

// TestPruneExcessImages_DBPipeline_TextOnlyAfterLimit verifies the key
// fix: even after images exceed the limit, a text-only follow-up message
// can still be sent because pruning brings the count back within bounds.
func TestPruneExcessImages_DBPipeline_TextOnlyAfterLimit(t *testing.T) {
	env := testEnv(t)
	agent := &sessionAgent{
		messages: env.messages,
	}

	sess, err := env.sessions.Create(t.Context(), "text-after-limit")
	require.NoError(t, err)

	// Create 11 messages with images (over Gemini's 10-image limit).
	for i := range 11 {
		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role: message.User,
			Parts: []message.ContentPart{
				message.TextContent{Text: fmt.Sprintf("image %d", i+1)},
				message.BinaryContent{
					Path:     fmt.Sprintf("img-%d.png", i+1),
					MIMEType: "image/png",
					Data:     []byte("fake"),
				},
			},
		})
		require.NoError(t, err)

		_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
			Role: message.Assistant,
			Parts: []message.ContentPart{
				message.TextContent{Text: "ok"},
			},
		})
		require.NoError(t, err)
	}

	// Now simulate a user sending a text-only message.
	// Before the fix, this would fail because 11 historical images are
	// always re-sent, exceeding the limit.
	_, err = env.messages.Create(t.Context(), sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Now summarize everything we discussed"},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs)
	require.Equal(t, 11, countImagesInMessages(history), "history should have 11 images before pruning")

	pruned := pruneExcessImages(history, geminiModel())
	assert.Equal(t, 10, countImagesInMessages(pruned), "after pruning, should have ≤ 10 images")

	// The text-only message should still be at the end of history.
	lastMsg := pruned[len(pruned)-1]
	assert.Equal(t, fantasy.MessageRoleUser, lastMsg.Role)
	tp, ok := fantasy.AsMessagePart[fantasy.TextPart](lastMsg.Content[0])
	require.True(t, ok)
	assert.Contains(t, tp.Text, "summarize everything")
}

// --- E2E test: mock LanguageModel that enforces image limits ---

// geminiMockModel implements fantasy.LanguageModel. It simulates Gemini's
// behaviour: if the prompt contains more than maxImages image parts, Stream
// returns the same "too many images" error that the real API would.
type geminiMockModel struct {
	maxImages     int
	imagesSeen    atomic.Int32 // images received on the last call
	callCount     atomic.Int32
	lastCallError atomic.Value // stores the last error (if any)
}

func (m *geminiMockModel) Provider() string { return "gemini" }
func (m *geminiMockModel) Model() string    { return "gemini-3.1-pro-preview" }

func (m *geminiMockModel) Generate(_ context.Context, _ fantasy.Call) (*fantasy.Response, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *geminiMockModel) GenerateObject(_ context.Context, _ fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *geminiMockModel) StreamObject(_ context.Context, _ fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *geminiMockModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.callCount.Add(1)

	// Count images in the prompt (exactly what the real Gemini API does).
	images := 0
	for _, msg := range call.Prompt {
		for _, part := range msg.Content {
			if fp, ok := fantasy.AsMessagePart[fantasy.FilePart](part); ok {
				if strings.HasPrefix(fp.MediaType, "image/") {
					images++
				}
			}
		}
	}
	m.imagesSeen.Store(int32(images))

	if images > m.maxImages {
		err := &fantasy.ProviderError{
			Title:   "bad request",
			Message: fmt.Sprintf("too many images: maximum allowed for model gemini-3.1-pro-preview is %d, got %d", m.maxImages, images),
		}
		m.lastCallError.Store(err)
		return nil, err
	}

	m.lastCallError.Store((*fantasy.ProviderError)(nil))

	// Return a simple successful stream.
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, Delta: "I see your images."})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop, Usage: fantasy.Usage{InputTokens: 10, OutputTokens: 5}})
	}, nil
}

// TestE2E_SessionAgentRun_PrunesImagesBeforeProvider is the most comprehensive
// test: it runs the real sessionAgent.Run() code path with a mock Gemini model
// that enforces a 10-image limit. It proves that:
//
//  1. With 12 images in history, the model only receives ≤ 10 (pruning works)
//  2. The agent call succeeds instead of returning "too many images"
//  3. After exceeding the limit, the session is still usable for new messages
func TestE2E_SessionAgentRun_PrunesImagesBeforeProvider(t *testing.T) {
	env := testEnv(t)
	mock := &geminiMockModel{maxImages: 10}

	largeModel := Model{
		Model:      mock,
		CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000, SupportsImages: true},
		ModelCfg:   config.SelectedModel{Provider: string(catwalk.InferenceProviderGemini)},
	}
	smallModel := Model{
		Model:      mock,
		CatwalkCfg: catwalk.Model{ContextWindow: 200000, DefaultMaxTokens: 10000},
		ModelCfg:   config.SelectedModel{Provider: string(catwalk.InferenceProviderGemini)},
	}

	agent := NewSessionAgent(SessionAgentOptions{
		LargeModel:   largeModel,
		SmallModel:   smallModel,
		SystemPrompt: "You are a helpful assistant.",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
	})

	sess, err := env.sessions.Create(t.Context(), "e2e-image-limit")
	require.NoError(t, err)

	// --- Phase 1: Send 12 messages with images (exceeds Gemini's 10 limit) ---
	for i := range 12 {
		_, err = agent.Run(t.Context(), SessionAgentCall{
			SessionID:       sess.ID,
			Prompt:          fmt.Sprintf("Here is screenshot %d", i+1),
			MaxOutputTokens: 10000,
			Attachments: []message.Attachment{{
				FileName: fmt.Sprintf("screenshot-%d.png", i+1),
				MimeType: "image/png",
				Content:  []byte(fmt.Sprintf("fake-png-data-%d", i+1)),
			}},
		})
		require.NoError(t, err, "call %d should succeed thanks to pruning", i+1)
	}

	// The mock model should never have seen more than 10 images in a single call.
	assert.LessOrEqual(t, int(mock.imagesSeen.Load()), 10,
		"model should never receive more than 10 images after pruning")

	// --- Phase 2: Send a text-only message after the limit was exceeded ---
	// Before the fix, this would fail because 12 historical images would be
	// re-sent. After the fix, pruning removes the oldest 2 images.
	_, err = agent.Run(t.Context(), SessionAgentCall{
		SessionID:       sess.ID,
		Prompt:          "Now summarize everything",
		MaxOutputTokens: 10000,
	})
	require.NoError(t, err, "text-only message should succeed after image limit was previously exceeded")

	assert.LessOrEqual(t, int(mock.imagesSeen.Load()), 10,
		"model should still receive ≤ 10 images on the follow-up call")

	// Verify the session accumulated all messages (not stuck/corrupted).
	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	// 12 rounds × (1 user + 1 assistant) + 1 text-only user + 1 assistant = 26
	assert.Equal(t, 26, len(msgs), "all messages should be persisted in the database")
}
