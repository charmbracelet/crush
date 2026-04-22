package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/stretchr/testify/require"
)

type mockFetchPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
}

func (m *mockFetchPermissionService) Request(ctx context.Context, req permission.CreatePermissionRequest) (bool, error) {
	return true, nil
}

func (m *mockFetchPermissionService) Grant(req permission.PermissionRequest)           {}
func (m *mockFetchPermissionService) Deny(req permission.PermissionRequest)            {}
func (m *mockFetchPermissionService) GrantPersistent(req permission.PermissionRequest) {}
func (m *mockFetchPermissionService) AutoApproveSession(sessionID string)              {}
func (m *mockFetchPermissionService) SetSkipRequests(skip bool)                        {}
func (m *mockFetchPermissionService) SkipRequests() bool                               { return false }
func (m *mockFetchPermissionService) SubscribeNotifications(ctx context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

func newFetchToolForTest() fantasy.AgentTool {
	permissions := &mockFetchPermissionService{Broker: pubsub.NewBroker[permission.PermissionRequest]()}
	return NewFetchTool(permissions, "/tmp", http.DefaultClient)
}

func runFetchTool(t *testing.T, tool fantasy.AgentTool, params FetchParams) fantasy.ToolResponse {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "test-session")
	resp, err := tool.Run(ctx, fantasy.ToolCall{ID: "t", Name: FetchToolName, Input: string(input)})
	require.NoError(t, err)
	return resp
}

func TestApplyJQ(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		expr string
		want string
	}{
		{
			name: "length of array",
			body: `[1,2,3,4,5]`,
			expr: `length`,
			want: `5`,
		},
		{
			name: "extract field",
			body: `{"name":"crush","version":"1.0"}`,
			expr: `.name`,
			want: `"crush"`,
		},
		{
			name: "count objects in array",
			body: `[{"id":"a"},{"id":"b"},{"id":"c"}]`,
			expr: `length`,
			want: `3`,
		},
		{
			name: "sum nested array lengths",
			body: `[{"models":[1,2]},{"models":[3,4,5]},{"models":[6]}]`,
			expr: `[.[].models | length] | add`,
			want: `6`,
		},
		{
			name: "extract names",
			body: `[{"name":"a"},{"name":"b"}]`,
			expr: `[.[].name]`,
			want: "[\n  \"a\",\n  \"b\"\n]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := applyJQ(tt.body, tt.expr)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestApplyJQErrors(t *testing.T) {
	t.Parallel()

	_, err := applyJQ(`not json`, `.`)
	require.Error(t, err)

	_, err = applyJQ(`[1,2,3]`, `|||`)
	require.Error(t, err)

	_, err = applyJQ(``, `.`)
	require.Error(t, err)
}

// TestApplyJQShapeHint verifies that when a jq filter fails because it
// assumed the wrong top-level shape, the error message includes a
// describeShape() hint so the caller can self-correct.
func TestApplyJQShapeHint(t *testing.T) {
	t.Parallel()

	// Filter assumes an object but body is an array. This is the exact
	// failure mode observed with kimi-k2.5 in the eval harness:
	// `.providers[]` against a top-level array.
	_, err := applyJQ(`[{"id":"a"},{"id":"b"}]`, `.providers[]`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "input shape:")
	require.Contains(t, err.Error(), "array of 2 items")
	require.Contains(t, err.Error(), "object with keys: id")

	// Filter assumes an array index but body is an object.
	_, err = applyJQ(`{"data":{"x":1},"meta":{}}`, `.[0]`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "input shape:")
	require.Contains(t, err.Error(), "object with keys: data, meta")
}

func TestDescribeShape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		json string
		want string
	}{
		{"null", `null`, "null"},
		{"bool", `true`, "boolean"},
		{"number", `42`, "number"},
		{"string", `"hi"`, "string"},
		{"empty array", `[]`, "empty array"},
		{"empty object", `{}`, "empty object"},
		{"array of objects", `[{"a":1,"b":2},{"a":3}]`, "array of 2 items; first item is object with keys: a, b"},
		{"object with keys", `{"zebra":1,"apple":2,"mango":3}`, "object with keys: apple, mango, zebra"},
		{
			"object truncates keys",
			`{"a":1,"b":2,"c":3,"d":4,"e":5,"f":6,"g":7,"h":8,"i":9,"j":10}`,
			"object with keys: a, b, c, d, e, f, g, h, ...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var v any
			dec := json.NewDecoder(strings.NewReader(tt.json))
			dec.UseNumber()
			require.NoError(t, dec.Decode(&v))
			require.Equal(t, tt.want, describeShape(v))
		})
	}
}

func TestLooksLikeJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentType string
		body        string
		want        bool
	}{
		{"json content type", "application/json", "garbage", true},
		{"json content type uppercase", "Application/JSON; charset=utf-8", "garbage", true},
		{"array body", "text/plain", "[1,2,3]", true},
		{"object body", "text/plain", `{"a":1}`, true},
		{"leading whitespace", "text/plain", "\n  \t[1]", true},
		{"html body", "text/html", "<html></html>", false},
		{"plain text", "text/plain", "hello world", false},
		{"empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, looksLikeJSON(tt.contentType, []byte(tt.body)))
		})
	}
}

// TestFetchToolJQHintPlacement verifies that when fetch returns a large
// JSON body without a jq filter:
//
//  1. The hint banner is appended (not prepended).
//  2. The content up to the banner is the unmodified original JSON and
//     parses cleanly — this is the "agent pipes response to jq" path we
//     want to protect from regressions.
//  3. Small JSON bodies do NOT get the banner (no unnecessary noise).
//  4. Non-JSON bodies (even if large) do NOT get the banner.
//  5. When jq is set, there is no banner and format validation is skipped.
func TestFetchToolJQHintPlacement(t *testing.T) {
	t.Parallel()

	// Build a JSON body larger than jqHintThreshold.
	items := make([]map[string]any, 3000)
	for i := range items {
		items[i] = map[string]any{"id": i, "name": "item"}
	}
	largeJSON, err := json.Marshal(items)
	require.NoError(t, err)
	require.Greater(t, len(largeJSON), jqHintThreshold, "fixture must exceed threshold")

	smallJSON := []byte(`[{"id":1,"name":"item"},{"id":2,"name":"item"}]`)
	require.Less(t, len(smallJSON), jqHintThreshold)

	largeText := strings.Repeat("lorem ipsum dolor sit amet ", 3000)
	require.Greater(t, len(largeText), jqHintThreshold)

	tests := []struct {
		name        string
		contentType string
		body        []byte
		params      FetchParams
		wantBanner  bool
		wantErr     bool
	}{
		{
			name:        "large JSON without jq gets trailing banner",
			contentType: "application/json",
			body:        largeJSON,
			params:      FetchParams{Format: "text"},
			wantBanner:  true,
		},
		{
			name:        "large JSON with jq has no banner",
			contentType: "application/json",
			body:        largeJSON,
			params:      FetchParams{JQ: "length"},
			wantBanner:  false,
		},
		{
			name:        "large JSON with jq and no format still works",
			contentType: "application/json",
			body:        largeJSON,
			params:      FetchParams{JQ: "length"}, // note: Format unset
			wantBanner:  false,
		},
		{
			name:        "small JSON gets no banner",
			contentType: "application/json",
			body:        smallJSON,
			params:      FetchParams{Format: "text"},
			wantBanner:  false,
		},
		{
			name:        "large non-JSON text gets no banner",
			contentType: "text/plain",
			body:        []byte(largeText),
			params:      FetchParams{Format: "text"},
			wantBanner:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				w.Write(tt.body)
			}))
			t.Cleanup(srv.Close)

			tool := newFetchToolForTest()
			params := tt.params
			params.URL = srv.URL

			resp := runFetchTool(t, tool, params)
			require.False(t, resp.IsError, "unexpected error: %s", resp.Content)

			if tt.wantBanner {
				require.Contains(t, resp.Content, "[crush-hint:")
				// Banner must be trailing, not leading.
				require.False(t, strings.HasPrefix(resp.Content, "[crush-hint:"),
					"banner must be appended, not prepended")
				// Critical regression guard: the content BEFORE the banner
				// must be the original JSON body, still parseable.
				bannerIdx := strings.LastIndex(resp.Content, "\n\n[crush-hint:")
				require.Greater(t, bannerIdx, 0, "banner not found in expected form")
				jsonPortion := resp.Content[:bannerIdx]
				var parsed any
				require.NoError(t, json.Unmarshal([]byte(jsonPortion), &parsed),
					"content before banner must be valid JSON")
			} else {
				require.NotContains(t, resp.Content, "[crush-hint:")
			}
		})
	}
}

// TestFetchToolFormatOptionalWithJQ verifies that format is optional
// (defaults to text) when jq is set, so callers don't get bounced on
// format="json" or missing format when they're using jq.
func TestFetchToolFormatOptionalWithJQ(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":1},{"id":2},{"id":3}]`))
	}))
	t.Cleanup(srv.Close)

	tool := newFetchToolForTest()

	// No format at all.
	resp := runFetchTool(t, tool, FetchParams{URL: srv.URL, JQ: "length"})
	require.False(t, resp.IsError, "expected success, got: %s", resp.Content)
	require.Equal(t, "3", resp.Content)

	// format="json" — historically rejected, should now pass because jq is set.
	resp = runFetchTool(t, tool, FetchParams{URL: srv.URL, Format: "json", JQ: "length"})
	require.False(t, resp.IsError, "expected success with format=json + jq, got: %s", resp.Content)
	require.Equal(t, "3", resp.Content)

	// Sanity: invalid format WITHOUT jq still rejected.
	resp = runFetchTool(t, tool, FetchParams{URL: srv.URL, Format: "json"})
	require.True(t, resp.IsError, "invalid format without jq should still error")
	require.Contains(t, resp.Content, "jq")
}

// TestFetchToolShapeHintSurfaces verifies that a wrong-shape jq filter
// against a fetched body returns an error whose message includes the
// (input shape: ...) hint, so the LLM has enough info to self-correct.
func TestFetchToolShapeHintSurfaces(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"id":"a"},{"id":"b"}]`))
	}))
	t.Cleanup(srv.Close)

	tool := newFetchToolForTest()
	resp := runFetchTool(t, tool, FetchParams{
		URL: srv.URL,
		JQ:  ".providers[].id",
	})
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "jq:")
	require.Contains(t, resp.Content, "input shape:")
	require.Contains(t, resp.Content, "array of 2 items")
}
