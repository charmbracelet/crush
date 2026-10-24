package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/proto"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// postThemeBody calls the POST theme handler directly with a raw body.
func postThemeBody(t *testing.T, c *controllerV1, wsID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost,
		"/v1/workspaces/"+wsID+"/theme", strings.NewReader(body))
	req.SetPathValue("id", wsID)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.handlePostWorkspaceTheme(rec, req)
	return rec
}

// getTheme calls the GET theme handler directly.
func getTheme(t *testing.T, c *controllerV1, wsID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/v1/workspaces/"+wsID+"/theme", nil)
	req.SetPathValue("id", wsID)
	rec := httptest.NewRecorder()
	c.handleGetWorkspaceTheme(rec, req)
	return rec
}

func decodeThemeState(t *testing.T, rec *httptest.ResponseRecorder) proto.HostThemeState {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var st proto.HostThemeState
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
	return st
}

func TestHostTheme_UnknownWorkspace(t *testing.T) {
	t.Parallel()
	c := newTestController()

	require.Equal(t, http.StatusNotFound, getTheme(t, c, "nope").Code)
	require.Equal(t, http.StatusNotFound,
		postThemeBody(t, c, "nope", `{"theme":{"palette":{"primary":"#6B50FF"}}}`).Code)
}

func TestHostTheme_SetGetClear(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c) // No App: exercises the publish guard.

	st := decodeThemeState(t, getTheme(t, c, ws.ID))
	require.Equal(t, ws.ID, st.WorkspaceID)
	require.Nil(t, st.Theme)
	require.JSONEq(t, `{"workspace_id":"`+ws.ID+`","theme":null}`, getTheme(t, c, ws.ID).Body.String())

	body := `{"theme":{"name":"charmtone-dark","source":"superjoy","base":"",` +
		`"palette":{"primary":"#6B50FF","bg_base":"#201F26","fg_base":"Steep","ansi_red":"9"},` +
		`"syntax":{"keyword":"#00A4FF","literal_string":"#BF976F"},"transparent":true}}`
	st = decodeThemeState(t, postThemeBody(t, c, ws.ID, body))
	require.Equal(t, ws.ID, st.WorkspaceID)
	require.NotNil(t, st.Theme)
	require.Equal(t, "charmtone-dark", st.Theme.Name)
	require.Equal(t, "superjoy", st.Theme.Source)
	require.Equal(t, "#6B50FF", st.Theme.Palette["primary"])
	require.Equal(t, "#BF976F", st.Theme.Syntax["literal_string"])
	require.NotNil(t, st.Theme.Transparent)
	require.True(t, *st.Theme.Transparent)

	got := decodeThemeState(t, getTheme(t, c, ws.ID))
	require.True(t, st.Theme.Equal(got.Theme))

	st = decodeThemeState(t, postThemeBody(t, c, ws.ID, `{"theme":null}`))
	require.Nil(t, st.Theme)
	require.Nil(t, decodeThemeState(t, getTheme(t, c, ws.ID)).Theme)
}

func TestHostTheme_RejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"bad json", `{"theme":`, "decode"},
		{"theme not an object", `{"theme":5}`, "decode"},
		{"empty body", ``, "decode"},
		{"not a color", `{"theme":{"palette":{"primary":"purple-ish"}}}`, "palette.primary"},
		{"short hex", `{"theme":{"palette":{"bg_base":"#12345"}}}`, "palette.bg_base"},
		{"hex with alpha", `{"theme":{"palette":{"primary":"#6b50ffff"}}}`, "palette.primary"},
		{"short hex with alpha", `{"theme":{"palette":{"primary":"#fffa"}}}`, "palette.primary"},
		{"ansi out of range", `{"theme":{"palette":{"ansi_red":"256"}}}`, "palette.ansi_red"},
		{"bad syntax color", `{"theme":{"syntax":{"keyword":"#zzzzzz"}}}`, "syntax.keyword"},
		{"unknown base", `{"theme":{"base":"definitely-not-a-theme-3f9a"}}`, "base theme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := postThemeBody(t, c, ws.ID, tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var perr proto.Error
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perr))
			require.Contains(t, perr.Message, tc.want)
		})
	}

	// Nothing was stored by any of the rejected requests.
	require.Nil(t, decodeThemeState(t, getTheme(t, c, ws.ID)).Theme)
}

// TestHostTheme_MissingThemeKeepsTheme covers a body without the "theme"
// key, such as a theme sent without its wrapper: it is rejected rather than
// read as a clear, so a host with a wire bug cannot wipe the theme.
func TestHostTheme_MissingThemeKeepsTheme(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	set := `{"theme":{"name":"kept","palette":{"primary":"#6B50FF"}}}`
	require.NotNil(t, decodeThemeState(t, postThemeBody(t, c, ws.ID, set)).Theme)

	for _, body := range []string{`{}`, `{"name":"x","palette":{"primary":"#FF0000"}}`} {
		rec := postThemeBody(t, c, ws.ID, body)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
		var perr proto.Error
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &perr))
		require.Contains(t, perr.Message, `"theme": null`)
	}
	st := decodeThemeState(t, getTheme(t, c, ws.ID))
	require.NotNil(t, st.Theme)
	require.Equal(t, "kept", st.Theme.Name)
}

func TestHostTheme_IgnoresUnknownKeys(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	body := `{"theme":{"name":"future","palette":{"primary":"#6B50FF","hologram":"#ffffff"},` +
		`"syntax":{"keyword":"#00A4FF","quantum_keyword":"not even a color"}},"future_field":1}`
	st := decodeThemeState(t, postThemeBody(t, c, ws.ID, body))
	require.NotNil(t, st.Theme)
	require.Equal(t, "#6B50FF", st.Theme.Palette["primary"])
}

func TestHostTheme_RejectsOversizedBody(t *testing.T) {
	t.Parallel()
	c := newTestController()
	ws := installSyntheticWorkspace(t, c)

	body := `{"theme":{"name":"` + strings.Repeat("x", maxHostThemeBody) + `"}}`
	rec := postThemeBody(t, c, ws.ID, body)
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	require.Nil(t, decodeThemeState(t, getTheme(t, c, ws.ID)).Theme)
}

// postHostTheme drives the real POST theme endpoint and returns the state
// it responds with.
func (h *e2eHarness) postHostTheme(t *testing.T, ctx context.Context, workspaceID string, theme *proto.HostTheme) proto.HostThemeState {
	t.Helper()
	body, err := json.Marshal(proto.HostThemeRequest{Theme: theme})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		h.httpSrv.URL+"/v1/workspaces/"+workspaceID+"/theme?client_id="+uuid.New().String(),
		bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpSrv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var st proto.HostThemeState
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&st))
	return st
}

// nextHostTheme returns the next host theme event on evc.
func nextHostTheme(t *testing.T, ctx context.Context, evc <-chan any) pubsub.Event[proto.HostThemeState] {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ev, ok := drainUntil[pubsub.Event[proto.HostThemeState]](ctx, evc, nil)
	require.True(t, ok, "expected a host_theme event")
	return ev
}

// TestE2E_HostThemeBroadcast verifies that a host theme reaches every
// client of the workspace, that re-sending the current theme publishes
// nothing, and that a clear arrives as a deleted event.
func TestE2E_HostThemeBroadcast(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx := t.Context()

	evcA, cancelA := h.subscribeSSE(t, ctx, h.workspace.ID, uuid.New().String())
	t.Cleanup(cancelA)
	evcB, cancelB := h.subscribeSSE(t, ctx, h.workspace.ID, uuid.New().String())
	t.Cleanup(cancelB)
	h.waitForAttached(t, 2)

	first := &proto.HostTheme{Name: "first", Palette: map[string]string{"primary": "#6B50FF"}}
	st := h.postHostTheme(t, ctx, h.workspace.ID, first)
	require.True(t, first.Equal(st.Theme))
	for _, evc := range []<-chan any{evcA, evcB} {
		ev := nextHostTheme(t, ctx, evc)
		require.Equal(t, pubsub.UpdatedEvent, ev.Type)
		require.Equal(t, h.workspace.ID, ev.Payload.WorkspaceID)
		require.True(t, first.Equal(ev.Payload.Theme))
	}

	// The same theme again is a no-op: the next event either client sees
	// is the second theme, not a duplicate of the first.
	h.postHostTheme(t, ctx, h.workspace.ID, first.Clone())
	second := &proto.HostTheme{Name: "second", Palette: map[string]string{"primary": "#FF60FF"}}
	h.postHostTheme(t, ctx, h.workspace.ID, second)
	for _, evc := range []<-chan any{evcA, evcB} {
		ev := nextHostTheme(t, ctx, evc)
		require.Equal(t, "second", ev.Payload.Theme.Name)
	}

	st = h.postHostTheme(t, ctx, h.workspace.ID, nil)
	require.Nil(t, st.Theme)
	for _, evc := range []<-chan any{evcA, evcB} {
		ev := nextHostTheme(t, ctx, evc)
		require.Equal(t, pubsub.DeletedEvent, ev.Type)
		require.Nil(t, ev.Payload.Theme)
	}
}

// TestE2E_HostThemeReplayedOnAttach verifies that a client attaching after
// the host set its theme receives it first, and that nothing is replayed
// once the theme is cleared.
func TestE2E_HostThemeReplayedOnAttach(t *testing.T) {
	t.Parallel()
	h := newE2EHarness(t)
	ctx := t.Context()

	theme := &proto.HostTheme{Name: "early", Syntax: map[string]string{"keyword": "#00A4FF"}}
	h.postHostTheme(t, ctx, h.workspace.ID, theme)

	evc, cancel := h.subscribeSSE(t, ctx, h.workspace.ID, uuid.New().String())
	t.Cleanup(cancel)
	ev := nextHostTheme(t, ctx, evc)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.True(t, theme.Equal(ev.Payload.Theme))

	h.postHostTheme(t, ctx, h.workspace.ID, nil)
	require.Equal(t, pubsub.DeletedEvent, nextHostTheme(t, ctx, evc).Type)

	// A late client after the clear gets no replay: the first host theme
	// event it sees is the next one the host sets.
	late, cancelLate := h.subscribeSSE(t, ctx, h.workspace.ID, uuid.New().String())
	t.Cleanup(cancelLate)
	h.waitForAttached(t, 2)
	next := &proto.HostTheme{Name: "next"}
	h.postHostTheme(t, ctx, h.workspace.ID, next)
	ev = nextHostTheme(t, ctx, late)
	require.Equal(t, pubsub.UpdatedEvent, ev.Type)
	require.Equal(t, "next", ev.Payload.Theme.Name)
}
