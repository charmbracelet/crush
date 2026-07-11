package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type recordedCall struct {
	method string
	body   map[string]any
}

func newFakeTelegram(t *testing.T, handler func(method string, body map[string]any) (any, error)) (*httptest.Server, *[]recordedCall) {
	t.Helper()
	var mu sync.Mutex
	calls := &[]recordedCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var body map[string]any
		if len(raw) > 0 {
			require.NoError(t, json.Unmarshal(raw, &body))
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// path: bot{token}/{method}
		require.GreaterOrEqual(t, len(parts), 2)
		method := parts[len(parts)-1]
		mu.Lock()
		*calls = append(*calls, recordedCall{method: method, body: body})
		mu.Unlock()

		result, err := handler(method, body)
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":          false,
				"description": err.Error(),
				"error_code":  400,
			})
			return
		}
		rawResult, err := json.Marshal(result)
		require.NoError(t, err)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": json.RawMessage(rawResult),
		})
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

func TestAPICallHappyPath(t *testing.T) {
	t.Parallel()
	srv, _ := newFakeTelegram(t, func(method string, _ map[string]any) (any, error) {
		require.Equal(t, "getMe", method)
		return User{ID: 42, Username: "crushbot"}, nil
	})
	a := newAPI("test-token", srv.URL)
	u, err := a.getMe(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(42), u.ID)
	require.Equal(t, "crushbot", u.Username)
}

func TestAPIErrorDoesNotContainToken(t *testing.T) {
	t.Parallel()
	token := "super-secret-token-xyz"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"description": "Unauthorized",
			"error_code":  401,
		})
	}))
	t.Cleanup(srv.Close)
	a := newAPI(token, srv.URL)
	err := a.call(context.Background(), "getMe", map[string]any{}, nil)
	require.Error(t, err)
	require.NotContains(t, err.Error(), token)
	require.Contains(t, err.Error(), "Unauthorized")
	require.Contains(t, err.Error(), "401")
}

func TestAPI429Retries(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		if n < 2 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":          false,
				"description": "Too Many Requests",
				"error_code":  429,
				"parameters": map[string]any{
					"retry_after": 1,
				},
			})
			return
		}
		raw, _ := json.Marshal(User{ID: 1})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"result": json.RawMessage(raw),
		})
	}))
	t.Cleanup(srv.Close)
	a := newAPI("tok", srv.URL)
	var slept []time.Duration
	a.sleep = func(d time.Duration) { slept = append(slept, d) }
	u, err := a.getMe(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), u.ID)
	require.Equal(t, int32(2), attempts.Load())
	require.Equal(t, []time.Duration{time.Second}, slept)
}

func TestGetUpdatesPayload(t *testing.T) {
	t.Parallel()
	srv, calls := newFakeTelegram(t, func(method string, _ map[string]any) (any, error) {
		require.Equal(t, "getUpdates", method)
		return []Update{}, nil
	})
	a := newAPI("tok", srv.URL)
	updates, err := a.getUpdates(context.Background(), 7, 50)
	require.NoError(t, err)
	require.Empty(t, updates)
	require.Len(t, *calls, 1)
	body := (*calls)[0].body
	require.EqualValues(t, 7, body["offset"])
	require.EqualValues(t, 50, body["timeout"])
	allowed, ok := body["allowed_updates"].([]any)
	require.True(t, ok)
	require.Equal(t, []any{"message", "callback_query"}, allowed)
}

func TestEditMessageTextNotModified(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"description": "Bad Request: message is not modified",
			"error_code":  400,
		})
	}))
	t.Cleanup(srv.Close)
	a := newAPI("tok", srv.URL)
	err := a.editMessageText(context.Background(), 1, 2, "same", false)
	require.NoError(t, err)
}

func TestSendMessageOpts(t *testing.T) {
	t.Parallel()
	srv, calls := newFakeTelegram(t, func(method string, _ map[string]any) (any, error) {
		require.Equal(t, "sendMessage", method)
		return Message{MessageID: 9, Text: "hi"}, nil
	})
	a := newAPI("tok", srv.URL)
	kb := &InlineKeyboardMarkup{
		InlineKeyboard: [][]InlineKeyboardButton{
			{{Text: "A", CallbackData: "a"}},
		},
	}
	msg, err := a.sendMessage(context.Background(), 123, "<b>hi</b>", &sendOpts{HTML: true, Keyboard: kb})
	require.NoError(t, err)
	require.Equal(t, int64(9), msg.MessageID)
	body := (*calls)[0].body
	require.Equal(t, "HTML", body["parse_mode"])
	require.NotNil(t, body["reply_markup"])
	require.NotNil(t, body["link_preview_options"])
}

// Transport errors wrap *url.Error, whose text contains the request URL
// including the token; redaction must keep it out of logs.
func TestTransportErrorRedactsToken(t *testing.T) {
	t.Parallel()
	const token = "123456:SECRET-abcdef"
	a := newAPI(token, "http://127.0.0.1:1")
	a.client.Timeout = 500 * time.Millisecond

	_, err := a.getMe(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), token)
	require.NotContains(t, err.Error(), "SECRET")
}
