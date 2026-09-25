package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/stretchr/testify/require"
)

// planHeaders is the header set the Codex backend returns on a plan-billed
// response, as observed on the wire: whole-percent utilization, window
// lengths in minutes, and absolute reset timestamps.
func planHeaders() http.Header {
	h := http.Header{}
	h.Set("x-codex-plan-type", "education")
	h.Set("x-codex-active-limit", "premium")
	h.Set("x-codex-credits-has-credits", "True")
	h.Set("x-codex-credits-unlimited", "False")
	h.Set("x-codex-primary-used-percent", "100")
	h.Set("x-codex-primary-window-minutes", "300")
	h.Set("x-codex-primary-reset-at", "1790369264")
	h.Set("x-codex-secondary-used-percent", "64")
	h.Set("x-codex-secondary-window-minutes", "10080")
	h.Set("x-codex-secondary-reset-at", "1790666659")
	return h
}

func TestParseUsage(t *testing.T) {
	t.Parallel()

	t.Run("reads both windows", func(t *testing.T) {
		t.Parallel()

		u, ok := ParseUsage(planHeaders())
		require.True(t, ok)

		require.Len(t, u.Windows, 2)

		require.Equal(t, 100.0, u.Windows[0].UsedPercent)
		require.Equal(t, 5*time.Hour, u.Windows[0].Length)
		require.Equal(t, time.Unix(1790369264, 0), u.Windows[0].Resets)

		require.Equal(t, 64.0, u.Windows[1].UsedPercent)
		require.Equal(t, 7*24*time.Hour, u.Windows[1].Length)
	})

	t.Run("a response without plan headers is not a plan response", func(t *testing.T) {
		t.Parallel()

		h := http.Header{}
		h.Set("content-type", "text/event-stream")
		u, ok := ParseUsage(h)
		require.False(t, ok, "an API-credit-billed response reports no plan usage")
		require.Equal(t, Usage{}, u)
	})

	t.Run("a window the backend has nothing to say about is dropped", func(t *testing.T) {
		t.Parallel()

		h := planHeaders()
		h.Set("x-codex-secondary-used-percent", "0")
		h.Del("x-codex-secondary-window-minutes")
		h.Del("x-codex-secondary-reset-at")

		u, ok := ParseUsage(h)
		require.True(t, ok)
		require.Len(t, u.Windows, 1, "an all-zero window carries no information")
		require.Equal(t, 5*time.Hour, u.Windows[0].Length)
	})

	t.Run("a genuinely empty window survives when it has a length", func(t *testing.T) {
		t.Parallel()

		h := planHeaders()
		h.Set("x-codex-primary-used-percent", "0")

		u, ok := ParseUsage(h)
		require.True(t, ok)
		require.Len(t, u.Windows, 2, "0% of a known window is a real reading")
		require.Zero(t, u.Windows[0].UsedPercent)
	})

	t.Run("missing windows leave only what arrived", func(t *testing.T) {
		t.Parallel()

		h := http.Header{}
		h.Set("x-codex-secondary-used-percent", "12")
		h.Set("x-codex-secondary-window-minutes", "10080")

		u, ok := ParseUsage(h)
		require.True(t, ok)
		require.Len(t, u.Windows, 1)
		require.Equal(t, 7*24*time.Hour, u.Windows[0].Length, "the window that arrived is the weekly one")
	})
}

func TestUsageExhausted(t *testing.T) {
	t.Parallel()

	t.Run("a spent window is not exhaustion", func(t *testing.T) {
		t.Parallel()

		// Observed live: a 5h window at 100% while the backend kept
		// serving requests, because credits backed it. Reading the
		// percentage as exhaustion would have called a working plan dead.
		u, ok := ParseUsage(planHeaders())
		require.True(t, ok)
		require.Equal(t, 1.0, u.Spent())
		require.False(t, u.LimitReached)
	})

	t.Run("the backend saying so is exhaustion", func(t *testing.T) {
		t.Parallel()

		h := planHeaders()
		h.Set("x-codex-rate-limit-reached-type", "rate_limit_reached")

		u, ok := ParseUsage(h)
		require.True(t, ok)
		require.True(t, u.LimitReached)
	})
}

func TestUsageSpentReportsTheFullestWindow(t *testing.T) {
	t.Parallel()

	// The backend names no representative window, so the one closest to its
	// limit speaks for the plan whichever position it holds.
	h := planHeaders()
	h.Set("x-codex-primary-used-percent", "10")
	h.Set("x-codex-secondary-used-percent", "82")

	u, ok := ParseUsage(h)
	require.True(t, ok)
	require.InDelta(t, 0.82, u.Spent(), 0.0001)
	require.Equal(t, "weekly", u.Windows[1].Label())
}

func TestWindowLabel(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		mins  int
		label string
	}{
		{"five hours", 300, "5h"},
		{"daily", 1440, "daily"},
		{"weekly", 10080, "weekly"},
		{"monthly", 43200, "monthly"},
		{"annual", 525600, "annual"},
		// The backend reports a rolling window whose length is
		// approximate, so a near miss still earns the name.
		{"near enough to a week", 10000, "weekly"},
		// A length matching nothing known gets no name rather than
		// being mislabelled as the nearest one.
		{"unknown length", 90, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			w := Window{Length: time.Duration(tt.mins) * time.Minute}
			require.Equal(t, tt.label, w.Label())
		})
	}

	t.Run("no length means no label", func(t *testing.T) {
		t.Parallel()
		require.Empty(t, Window{}.Label())
	})
}

func TestWindowSpentIsClamped(t *testing.T) {
	t.Parallel()

	require.Equal(t, 1.0, Window{UsedPercent: 140}.Spent())
	require.Equal(t, 0.0, Window{UsedPercent: -5}.Spent())
	require.InDelta(t, 0.64, Window{UsedPercent: 64}.Spent(), 0.0001)
}

func TestFetchUsage(t *testing.T) {
	// Not parallel: this repoints the package's endpoint.
	body := map[string]any{
		"plan_type": "education",
		"rate_limit": map[string]any{
			"allowed":       true,
			"limit_reached": false,
			"primary_window": map[string]any{
				"used_percent":         100,
				"limit_window_seconds": 18000,
				"reset_at":             1790369264,
			},
			"secondary_window": map[string]any{
				"used_percent":         64,
				"limit_window_seconds": 604800,
				"reset_at":             1790666659,
			},
		},
		"credits": map[string]any{"has_credits": true, "unlimited": false},
	}

	var gotAuth, gotAccount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("chatgpt-account-id")
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(body))
	}))
	defer srv.Close()

	original := usageEndpoint
	usageEndpoint = srv.URL + UsagePath
	defer func() { usageEndpoint = original }()

	u, err := FetchUsage(context.Background(), &oauth.Token{
		AccessToken: "at-1",
		AccountID:   "acct-1",
	})
	require.NoError(t, err)

	require.Equal(t, "Bearer at-1", gotAuth)
	require.Equal(t, "acct-1", gotAccount)

	require.False(t, u.LimitReached, "the endpoint said requests are still allowed")

	// The endpoint reports window length in seconds where the headers use
	// minutes; both must land on the same duration.
	require.Len(t, u.Windows, 2)
	require.Equal(t, 5*time.Hour, u.Windows[0].Length)
	require.Equal(t, "5h", u.Windows[0].Label())
	require.Equal(t, 7*24*time.Hour, u.Windows[1].Length)
	require.Equal(t, "weekly", u.Windows[1].Label())
}

func TestFetchUsageReadsRefusalAsExhaustion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"rate_limit":{"allowed":false,"limit_reached":true,` +
			`"primary_window":{"used_percent":100,"limit_window_seconds":18000}}}`))
	}))
	defer srv.Close()

	original := usageEndpoint
	usageEndpoint = srv.URL + UsagePath
	defer func() { usageEndpoint = original }()

	u, err := FetchUsage(context.Background(), &oauth.Token{AccessToken: "at-1"})
	require.NoError(t, err)
	require.True(t, u.LimitReached)
}

func TestFetchUsageRequiresAToken(t *testing.T) {
	t.Parallel()

	_, err := FetchUsage(context.Background(), nil)
	require.Error(t, err)
}

func TestFetchUsageRejectsAnErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	original := usageEndpoint
	usageEndpoint = srv.URL + UsagePath
	defer func() { usageEndpoint = original }()

	_, err := FetchUsage(context.Background(), &oauth.Token{AccessToken: "at-1"})
	require.Error(t, err)
}

func TestResetsAtPicksTheBlockingWindow(t *testing.T) {
	t.Parallel()

	fiveHour := time.Unix(1790369264, 0)
	weekly := time.Unix(1790666659, 0)

	t.Run("the fullest window decides", func(t *testing.T) {
		t.Parallel()

		u := Usage{Windows: []Window{
			{UsedPercent: 100, Resets: fiveHour},
			{UsedPercent: 64, Resets: weekly},
		}}
		require.Equal(t, fiveHour, u.ResetsAt())
	})

	t.Run("both spent means waiting for the later one", func(t *testing.T) {
		t.Parallel()

		u := Usage{Windows: []Window{
			{UsedPercent: 100, Resets: fiveHour},
			{UsedPercent: 100, Resets: weekly},
		}}
		require.Equal(t, weekly, u.ResetsAt(), "a cleared 5h window does not unblock a spent weekly one")
	})

	t.Run("order does not matter", func(t *testing.T) {
		t.Parallel()

		u := Usage{Windows: []Window{
			{UsedPercent: 100, Resets: weekly},
			{UsedPercent: 100, Resets: fiveHour},
		}}
		require.Equal(t, weekly, u.ResetsAt())
	})
}
