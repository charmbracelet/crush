package usage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// geminiShape is the report shape most subscription gateways answer with:
// groups of buckets, each bucket carrying a share, a window, and a reset time.
const geminiShape = `{
	"groups": [
		{
			"displayName": "Gemini Models",
			"buckets": [
				{
					"displayName": "Weekly Limit Remaining",
					"window": "weekly",
					"resetTime": "%s",
					"description": "You have used some of your weekly limit",
					"remainingFraction": 0.9997
				},
				{
					"displayName": "Five Hour Limit Remaining",
					"window": "5h",
					"resetTime": "%s",
					"remainingFraction": 0.5
				}
			]
		},
		{
			"displayName": "Third-party Models",
			"buckets": [
				{
					"displayName": "Weekly Limit Remaining",
					"resetTime": "%s",
					"remainingFraction": 1
				}
			}
		}
	]
}`

func quotaServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
}

func TestFetchGeminiShapedQuota(t *testing.T) {
	t.Parallel()

	var sawAuth, sawUserAgent string
	reset := time.Now().Add(4 * time.Hour).UTC().Format(time.RFC3339)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		sawUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, geminiShape, reset, reset, reset)
	}))
	defer server.Close()

	meters, err := Fetch(context.Background(), &oauth.UsageSpec{
		URL:    server.URL + "/v1internal:retrieveUserQuotaSummary",
		Method: "POST",
		Groups: "groups",
		Meters: "buckets",
	}, "at-123", map[string]string{"User-Agent": "example-cli/1.2.4"})
	require.NoError(t, err)
	require.Len(t, meters, 3)
	// Some gateways refuse the quota call outright unless the request looks
	// like a supported client, so the provider's headers must ride along.
	require.Equal(t, "Bearer at-123", sawAuth)
	require.Equal(t, "example-cli/1.2.4", sawUserAgent)

	require.Equal(t, "Gemini Models", meters[0].Group)
	require.Equal(t, "Weekly Limit Remaining", meters[0].Label)
	require.Equal(t, "weekly", meters[0].Short)
	require.InDelta(t, 0.9997, meters[0].Left, 1e-9)
	require.True(t, meters[0].HasReset)
	require.Equal(t, "Third-party Models", meters[2].Group)
	require.Contains(t, meters[1].Summary(), "Gemini Models · Five Hour Limit Remaining")
	require.Contains(t, meters[1].Summary(), "50% left")
	// Four hours minus the microseconds already elapsed reads as 3h 59m.
	require.Regexp(t, `resets in \d+h \d+m`, meters[1].Summary())
}

// A spec that only names the URL relies on the defaults, which match the
// shape most providers use.
func TestFetchAppliesPathDefaults(t *testing.T) {
	t.Parallel()

	server := quotaServer(t, `{"groups":[{"displayName":"G","buckets":[{"displayName":"B","remainingFraction":0.25}]}]}`)
	defer server.Close()

	meters, err := Fetch(context.Background(), &oauth.UsageSpec{
		URL:    server.URL,
		Method: "POST",
		Groups: "groups",
	}, "at-123", nil)
	require.NoError(t, err)
	require.Equal(t, "G", meters[0].Group)
	require.Equal(t, "B", meters[0].Label)
	require.InDelta(t, 0.25, meters[0].Left, 1e-9)
	require.False(t, meters[0].HasReset)
}

// A flat endpoint returning a single number, like a credit balance, is read
// without groups.
func TestFetchFlatAmount(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer key-1", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Content-Type"), "a GET carries no body")
		fmt.Fprint(w, `{"credits": 42.5}`)
	}))
	defer server.Close()

	meters, err := Fetch(context.Background(), &oauth.UsageSpec{
		URL:       server.URL,
		Meters:    oauth.MetersAtRoot,
		Remaining: "credits",
		Title:     "Credits",
	}, "key-1", nil)
	require.NoError(t, err)
	require.Equal(t, "Credits", meters[0].Label)
	require.InDelta(t, 42.5, meters[0].Left, 1e-9)
	require.False(t, meters[0].IsFraction())
	require.Contains(t, meters[0].Summary(), "42.5 left")
}

func TestFetchErrors(t *testing.T) {
	t.Parallel()

	_, err := Fetch(context.Background(), nil, "x", nil)
	require.ErrorContains(t, err, "no usage endpoint")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"message":"no valid license"}}`)
	}))
	defer server.Close()

	_, err = Fetch(context.Background(), &oauth.UsageSpec{URL: server.URL}, "x", nil)
	require.ErrorContains(t, err, "usage endpoint returned 403")
	require.ErrorContains(t, err, "no valid license")

	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"groups":[]}`)
	}))
	defer empty.Close()

	_, err = Fetch(context.Background(), &oauth.UsageSpec{
		URL: empty.URL, Method: "POST", Groups: "groups",
	}, "x", nil)
	require.ErrorContains(t, err, "no meters")
}

func TestHumanUntil(t *testing.T) {
	t.Parallel()

	require.Equal(t, "in 6d 23h", humanUntil(6*24*time.Hour+23*time.Hour+40*time.Minute))
	require.Equal(t, "in 4h 17m", humanUntil(4*time.Hour+17*time.Minute))
	require.Equal(t, "in 30m", humanUntil(30*time.Minute))
	require.Equal(t, "now", humanUntil(-time.Minute))
}

// A plan that reports different limits per model family narrows to the ones
// the model in use draws from.
func TestForModel(t *testing.T) {
	t.Parallel()

	meters := []Meter{
		{Group: "Gemini Models", Label: "Weekly", Short: "weekly"},
		{Group: "Gemini Models", Label: "Five hour", Short: "5h"},
		{Group: "Claude and GPT models", Label: "Weekly", Short: "weekly"},
	}
	spec := &oauth.UsageSpec{
		ModelGroups: map[string]string{
			"gemini": "Gemini",
			"cld":    "Claude",
			"gpt":    "Claude",
		},
	}

	gemini := ForModel(meters, spec, "gemini-3.8-flash")
	require.Len(t, gemini, 2)
	require.Equal(t, "Gemini Models", gemini[0].Group)
	require.Equal(t, "5h", gemini[1].Short)

	claude := ForModel(meters, spec, "cld-sonnet-5-5")
	require.Len(t, claude, 1)
	require.Equal(t, "Claude and GPT models", claude[0].Group)

	// A family the mapping does not cover shows nothing: one family's limits
	// beside another's model reads as a plan being spent when it is not.
	require.Empty(t, ForModel(meters, spec, "o1-something"))
	require.Equal(t, meters, ForModel(meters, nil, "gemini-3.8-flash"), "no mapping means no filtering")

	// The longest matching prefix wins, so families that share a stem stay
	// distinct.
	tiered := []Meter{{Group: "Tier A"}, {Group: "Tier B"}}
	tieredSpec := &oauth.UsageSpec{
		ModelGroups: map[string]string{"model-a": "Tier A", "model": "Tier B"},
	}
	require.Equal(t, "Tier A", ForModel(tiered, tieredSpec, "model-a-1")[0].Group)
	require.Equal(t, "Tier B", ForModel(tiered, tieredSpec, "model-b-1")[0].Group)
}

// Tag picks the short window label a one-line display needs.
func TestMeterTagAndPercent(t *testing.T) {
	t.Parallel()

	require.Equal(t, "5h", Meter{Short: "5h", Label: "Weekly"}.Tag())
	require.Equal(t, "Weekly", Meter{Label: "Weekly"}.Tag())
	require.Equal(t, 99, Meter{Left: 0.9997}.Percent())
}

// A plan reporting the share already spent, as a whole percentage, is the
// mirror image of one reporting the share left: the reader complements it, and
// a gjson filter narrows the rows to the plan-wide limits.
func TestFetchSpentPercentage(t *testing.T) {
	t.Parallel()

	reset := time.Now().Add(4 * time.Hour).UTC().Format(time.RFC3339)
	server := quotaServer(t, fmt.Sprintf(
		`{"limits":[{"group":"session","kind":"session","percent":0,"resets_at":%q},`+
			`{"group":"weekly","kind":"weekly_all","percent":1,"resets_at":%q},`+
			`{"group":"weekly","kind":"weekly_scoped","percent":40,"scope":{"model":{}},"resets_at":%q}]}`,
		reset, reset, reset))
	defer server.Close()

	meters, err := Fetch(context.Background(), &oauth.UsageSpec{
		URL:          server.URL,
		Meters:       `limits.#(kind!="weekly_scoped")#`,
		Label:        "group",
		SpentPercent: "percent",
		Reset:        "resets_at",
	}, "at-123", nil)
	require.NoError(t, err)
	require.Len(t, meters, 2, "the filtered row is left out")

	// Untouched reads as the whole allowance, and a plan 1% spent reads as 99%
	// left rather than 1%: the value is a percentage, so it is complemented on
	// that scale rather than judged by size.
	require.InDelta(t, 1, meters[0].Left, 1e-9)
	require.InDelta(t, 0.99, meters[1].Left, 1e-9)
	require.Contains(t, meters[1].Summary(), "weekly: 99% left")
	require.True(t, meters[1].HasReset)
}
