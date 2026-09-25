package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
)

// The Codex backend reports where a ChatGPT plan stands on every response it
// serves, so these figures cost nothing to collect and are only ever as stale
// as the last request. A plan is metered over two rolling windows at once, a
// short one and a long one, and the backend reports the length of each
// alongside its utilization, so the labels people read are derived rather
// than assumed.

// Window is one rolling usage window of a ChatGPT plan.
type Window struct {
	// UsedPercent is how much of the window is spent, 0 to 100. The backend
	// reports whole percent here, where Anthropic reports a fraction.
	UsedPercent float64
	// Length is how long the window rolls over, zero when unreported.
	Length time.Duration
	// Resets is when the window refills, zero when unreported.
	Resets time.Time
}

// Spent returns the window's utilization from 0 to 1, which is what the
// styling thresholds are written against.
func (w Window) Spent() float64 {
	return min(max(w.UsedPercent/100, 0), 1)
}

// namedWindows are the window lengths worth a human label, matched within 5%
// because the backend's rolling windows are approximate.
var namedWindows = []struct {
	length time.Duration
	label  string
}{
	{5 * time.Hour, "5h"},
	{24 * time.Hour, "daily"},
	{7 * 24 * time.Hour, "weekly"},
	{30 * 24 * time.Hour, "monthly"},
	{365 * 24 * time.Hour, "annual"},
}

// Label names the window for a person reading it: "5h", "weekly", and so on.
// It is empty for a window whose length is unknown or unrecognized, so a
// plan metered over something unexpected goes unlabelled rather than
// mislabelled.
func (w Window) Label() string {
	for _, known := range namedWindows {
		if w.Length >= known.length*95/100 && w.Length <= known.length*105/100 {
			return known.label
		}
	}
	return ""
}

// Usage is a snapshot of a ChatGPT plan's usage.
type Usage struct {
	// Windows holds the windows the backend reported, shortest first.
	Windows []Window
	// LimitReached is set once the backend has stopped serving the plan.
	// A window at 100% is not the same thing: the backend keeps serving a
	// spent window while credits or the other window have room.
	LimitReached bool
}

// Spent returns the utilization of whichever window is closest to its limit.
// The backend names no representative window, so the fullest one speaks for
// the plan.
func (u Usage) Spent() float64 {
	var spent float64
	for _, w := range u.Windows {
		spent = max(spent, w.Spent())
	}
	return spent
}

// ResetsAt returns when the plan's fullest window refills, which is when a
// blocked plan starts serving again. On a tie the later refill wins: both
// windows must clear. Zero when the backend reported no time.
func (u Usage) ResetsAt() time.Time {
	var (
		spent float64
		at    time.Time
	)
	for _, w := range u.Windows {
		if s := w.Spent(); s > spent || (s == spent && w.Resets.After(at)) {
			spent, at = s, w.Resets
		}
	}
	return at
}

// ParseUsage reads a snapshot from response headers. The second return is
// false when the response carries no plan headers, which is how a response
// billed against API credits rather than a plan looks.
func ParseUsage(h http.Header) (Usage, bool) {
	u := Usage{LimitReached: h.Get("x-codex-rate-limit-reached-type") != ""}
	for _, position := range []string{"primary", "secondary"} {
		if w, ok := parseWindow(h, position); ok {
			u.Windows = append(u.Windows, w)
		}
	}
	if len(u.Windows) == 0 {
		return Usage{}, false
	}
	return u, true
}

// parseWindow reads one window's headers. It reports false when the
// utilization header is absent, and when every field reads as zero, which is
// how the backend spells a window it has nothing to say about.
func parseWindow(h http.Header, position string) (Window, bool) {
	const prefix = "x-codex-"
	used := h.Get(prefix + position + "-used-percent")
	if used == "" {
		return Window{}, false
	}

	w := Window{UsedPercent: parseFloat(used)}
	if mins := parseFloat(h.Get(prefix + position + "-window-minutes")); mins > 0 {
		w.Length = time.Duration(mins) * time.Minute
	}
	if secs, err := strconv.ParseInt(h.Get(prefix+position+"-reset-at"), 10, 64); err == nil && secs > 0 {
		w.Resets = time.Unix(secs, 0)
	}

	if w.UsedPercent == 0 && w.Length == 0 && w.Resets.IsZero() {
		return Window{}, false
	}
	return w, true
}

func parseFloat(v string) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return f
}

var (
	usageMu     sync.RWMutex
	latestUsage *Usage
)

// LatestUsage returns the most recent snapshot seen, and whether one has been
// seen at all.
//
// This is process-global rather than threaded through the agent because the
// figures arrive as a side effect of requests the UI does not otherwise
// watch, and every caller wants the same most-recent answer.
func LatestUsage() (Usage, bool) {
	usageMu.RLock()
	defer usageMu.RUnlock()
	if latestUsage == nil {
		return Usage{}, false
	}
	return *latestUsage, true
}

// RecordUsage stores a snapshot, so one fetched and one read off a response
// header land in the same place.
func RecordUsage(u Usage) {
	usageMu.Lock()
	defer usageMu.Unlock()
	latestUsage = &u
}

// UsagePath is where the Codex backend reports plan utilization without
// spending a request.
const UsagePath = "/wham/usage"

// usageEndpoint is the full URL. A var so tests can point it at a stub.
var usageEndpoint = BackendBaseURL + UsagePath

// usageWindow is one window as the usage endpoint reports it, which names the
// same figures differently from the headers and measures the window in whole
// seconds rather than minutes.
type usageWindow struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

func (w usageWindow) window() Window {
	out := Window{
		UsedPercent: w.UsedPercent,
		Length:      time.Duration(w.LimitWindowSeconds) * time.Second,
	}
	if w.ResetAt > 0 {
		out.Resets = time.Unix(w.ResetAt, 0)
	}
	return out
}

type usageResponse struct {
	RateLimit struct {
		LimitReached bool         `json:"limit_reached"`
		Allowed      bool         `json:"allowed"`
		Primary      *usageWindow `json:"primary_window"`
		Secondary    *usageWindow `json:"secondary_window"`
	} `json:"rate_limit"`
}

func (r usageResponse) usage() Usage {
	// The endpoint says outright whether requests are still being served,
	// which the headers only imply.
	u := Usage{LimitReached: r.RateLimit.LimitReached || !r.RateLimit.Allowed}
	for _, w := range []*usageWindow{r.RateLimit.Primary, r.RateLimit.Secondary} {
		if w != nil {
			u.Windows = append(u.Windows, w.window())
		}
	}
	return u
}

// FetchUsage reads plan utilization from the Codex backend directly, so usage
// is known before the first message of a session rather than after it.
func FetchUsage(ctx context.Context, token *oauth.Token) (Usage, error) {
	if token == nil {
		return Usage{}, fmt.Errorf("an OAuth token is required to read ChatGPT plan usage")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageEndpoint, nil)
	if err != nil {
		return Usage{}, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if token.AccountID != "" {
		req.Header.Set("chatgpt-account-id", token.AccountID)
	}
	req.Header.Set("originator", "crush")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return Usage{}, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return Usage{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var body usageResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return Usage{}, fmt.Errorf("could not decode response: %w", err)
	}
	return body.usage(), nil
}
