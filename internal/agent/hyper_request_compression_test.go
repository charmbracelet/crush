package agent

// Hyper accepts gzip request bodies, and every turn re-uploads the whole
// conversation, so the Hyper provider compresses what it sends. Other
// openai-compat providers must not, because nothing tells us they can read it.

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/hyper"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// encodingRecorder answers chat completion requests and remembers how each
// arriving body was encoded.
type encodingRecorder struct {
	mu        sync.Mutex
	encodings []string
	bodies    []string
	server    *httptest.Server
}

func newEncodingRecorder(t *testing.T) *encodingRecorder {
	t.Helper()
	rec := &encodingRecorder{}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		encoding := r.Header.Get("Content-Encoding")
		body := string(raw)
		if encoding == "gzip" {
			zr, gzErr := gzip.NewReader(bytes.NewReader(raw))
			if gzErr != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			inflated, gzErr := io.ReadAll(zr)
			require.NoError(t, gzErr)
			body = string(inflated)
		}

		rec.mu.Lock()
		rec.encodings = append(rec.encodings, encoding)
		rec.bodies = append(rec.bodies, body)
		rec.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"z","created":1,"model":"test-model","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":2,"total_tokens":4}}`))
	}))
	t.Cleanup(rec.server.Close)
	return rec
}

func (r *encodingRecorder) received() ([]string, []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.encodings...), append([]string(nil), r.bodies...)
}

// longPrompt clears the compression floor, which is what any session with
// history in it does.
func longPrompt() fantasy.Prompt {
	return fantasy.Prompt{
		{
			Role:    fantasy.MessageRoleSystem,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: strings.Repeat("You are Crush. ", 200)}},
		},
		{
			Role:    fantasy.MessageRoleUser,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: "hello"}},
		},
	}
}

func testCoordinator(t *testing.T) *coordinator {
	t.Helper()
	cfg, err := config.Init(t.Context(), t.TempDir(), "", false)
	require.NoError(t, err)
	return &coordinator{cfg: cfg}
}

func generateAgainst(t *testing.T, coord *coordinator, baseURL, providerID string, prompt fantasy.Prompt) error {
	t.Helper()
	provider, err := coord.buildOpenaiCompatProvider(baseURL, "test-key", nil, nil, providerID, false)
	require.NoError(t, err)
	lm, err := provider.LanguageModel(t.Context(), "test-model")
	require.NoError(t, err)
	_, err = lm.Generate(t.Context(), fantasy.Call{Prompt: prompt})
	return err
}

func TestHyperProviderCompressesRequests(t *testing.T) {
	t.Parallel()

	t.Run("hyper uploads a compressed body", func(t *testing.T) {
		t.Parallel()
		rec := newEncodingRecorder(t)

		require.NoError(t, generateAgainst(t, testCoordinator(t), rec.server.URL+"/v1", hyper.Name, longPrompt()))

		encodings, bodies := rec.received()
		require.Equal(t, []string{"gzip"}, encodings, "one compressed request, no retry")
		require.Contains(t, bodies[0], `"model":"test-model"`,
			"the server must be able to inflate what Hyper was handed")
	})

	// A compressed body is a contract with the server. An endpoint that was
	// never told about it should not have to guess.
	t.Run("other openai-compat providers upload plain JSON", func(t *testing.T) {
		t.Parallel()
		rec := newEncodingRecorder(t)

		require.NoError(t, generateAgainst(t, testCoordinator(t), rec.server.URL+"/v1", "some-other-provider", longPrompt()))

		encodings, _ := rec.received()
		require.Equal(t, []string{""}, encodings)
	})

	t.Run("short hyper prompts are not worth compressing", func(t *testing.T) {
		t.Parallel()
		rec := newEncodingRecorder(t)

		prompt := fantasy.Prompt{
			{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{fantasy.TextPart{Text: "hi"}}},
		}
		require.NoError(t, generateAgainst(t, testCoordinator(t), rec.server.URL+"/v1", hyper.Name, prompt))

		encodings, _ := rec.received()
		require.Equal(t, []string{""}, encodings)
	})
}
