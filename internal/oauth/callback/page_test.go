package callback

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWrite_Success(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	require.NoError(t, Write(&b, Result{Subject: "linear"}))
	page := b.String()

	require.Contains(t, page, `class="card ok"`)
	require.Contains(t, page, "You’re all set")
	require.Contains(t, page, "linear")
	// A successful page counts itself down and closes.
	require.Contains(t, page, `data-delay="5"`)
	// Everything needed to render must be inlined, so the page still
	// works with no network beyond the optional web font.
	require.Contains(t, page, "<svg")
	require.Contains(t, page, "data:image/svg")
}

// TestWrite_FailureDoesNotAutoClose guards the choice not to yank an error
// message away from whoever is reading it.
func TestWrite_FailureDoesNotAutoClose(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	require.NoError(t, Write(&b, Result{
		Subject:          "sentry",
		ErrorCode:        "access_denied",
		ErrorDescription: "The user declined the request.",
	}))
	page := b.String()

	require.Contains(t, page, `class="card failed"`)
	require.Contains(t, page, "access_denied")
	require.Contains(t, page, "The user declined the request.")
	// A failed page has no countdown rail, so nothing takes the reason away.
	require.NotContains(t, page, `id="rail"`)
	require.NotContains(t, page, `data-delay=`)
}

// TestWrite_GrumpyOnFailure proves the artwork matches the outcome: a
// grumpy heart when authorization fails, the smiling one when it works.
// The two are told apart by a path unique to each drawing.
func TestWrite_GrumpyOnFailure(t *testing.T) {
	t.Parallel()

	// The grumpy drawing carries a dark red accent (#ab2454) that the
	// smiling heart does not.
	const grumpy = "#ab2454"

	var failed strings.Builder
	require.NoError(t, Write(&failed, Result{ErrorCode: "access_denied"}))
	require.Contains(t, failed.String(), grumpy)

	var ok strings.Builder
	require.NoError(t, Write(&ok, Result{Subject: "linear"}))
	require.NotContains(t, ok.String(), grumpy)
}

// TestWrite_TerseFailure covers providers that report an error code with no
// description: the page must still explain itself rather than trailing off.
func TestWrite_TerseFailure(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	require.NoError(t, Write(&b, Result{ErrorCode: "server_error"}))
	page := b.String()

	require.Contains(t, page, "server_error")
	require.Contains(t, page, "did not")
	// With no subject the sentence must not dangle on a preposition.
	require.NotContains(t, page, "access to <span")
	require.Contains(t, page, "Crush was not granted access.")
}

// TestWrite_EscapesUntrustedText proves provider-supplied strings cannot
// inject markup. Error descriptions come straight off a query string.
func TestWrite_EscapesUntrustedText(t *testing.T) {
	t.Parallel()

	var b strings.Builder
	require.NoError(t, Write(&b, Result{
		Subject:          `<img src=x onerror=alert(1)>`,
		ErrorCode:        `<script>`,
		ErrorDescription: `</div><script>alert(2)</script>`,
	}))
	page := b.String()

	require.NotContains(t, page, "<img src=x")
	require.NotContains(t, page, "<script>alert(2)")
	require.Contains(t, page, "&lt;img")
}

func TestServe_StatusCodes(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		result Result
		want   int
	}{
		"success": {Result{Subject: "linear"}, http.StatusOK},
		"failure": {Result{ErrorCode: "access_denied"}, http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			require.NoError(t, Serve(rec, tc.result))
			require.Equal(t, tc.want, rec.Code)
			require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
			// The page reports a one-time result and must not be replayed
			// from cache on a later visit to the same localhost URL.
			require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		})
	}
}
