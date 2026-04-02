package openaicodex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBackendBaseURL = "https://chatgpt.com/backend-api"
	defaultLimitID        = "codex"
	ProviderID            = "openai-codex"
)

type UsageRequest struct {
	BaseURL     string
	AccessToken string
	AccountID   string
}

type Window struct {
	UsedPercent   float64
	WindowMinutes int64
	ResetsAtUnix  int64
}

type Credits struct {
	HasCredits bool
	Unlimited  bool
	Balance    string
}

type Snapshot struct {
	LimitID   string
	LimitName string
	Primary   *Window
	Secondary *Window
	Credits   *Credits
}

type UsageReport struct {
	PlanType  string
	Snapshots []Snapshot
}

func FetchUsage(ctx context.Context, req UsageRequest) (*UsageReport, error) {
	if strings.TrimSpace(req.AccessToken) == "" {
		return nil, fmt.Errorf("missing access token")
	}
	if strings.TrimSpace(req.AccountID) == "" {
		return nil, fmt.Errorf("missing chatgpt-account-id header")
	}

	endpoint, err := usageEndpoint(req.BaseURL)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.AccessToken)
	httpReq.Header.Set("chatgpt-account-id", req.AccountID)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request usage endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read usage response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = http.StatusText(resp.StatusCode)
		}
		return nil, fmt.Errorf("usage endpoint returned %d: %s", resp.StatusCode, msg)
	}

	report, err := parseUsagePayload(body)
	if err != nil {
		return nil, fmt.Errorf("parse usage payload: %w", err)
	}
	return report, nil
}

func usageEndpoint(baseURL string) (string, error) {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		raw = defaultBackendBaseURL
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid base URL %q: %w", raw, err)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(u.Path, "/codex") {
		u.Path = strings.TrimSuffix(u.Path, "/codex")
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/wham/usage"
	return u.String(), nil
}

func parseUsagePayload(body []byte) (*UsageReport, error) {
	var payload usagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	report := &UsageReport{
		PlanType: strings.ToLower(strings.TrimSpace(payload.PlanType)),
	}

	credits := mapCredits(payload.Credits)
	if snap := mapSnapshot(defaultLimitID, defaultLimitID, payload.RateLimit, credits); snap != nil {
		report.Snapshots = append(report.Snapshots, *snap)
	}

	for _, add := range payload.AdditionalRateLimits {
		limitID := strings.TrimSpace(add.MeteredFeature)
		if limitID == "" {
			limitID = strings.TrimSpace(add.LimitName)
		}
		if limitID == "" {
			limitID = "unknown"
		}

		limitName := strings.TrimSpace(add.LimitName)
		if limitName == "" {
			limitName = limitID
		}

		if snap := mapSnapshot(limitID, limitName, add.RateLimit, credits); snap != nil {
			report.Snapshots = append(report.Snapshots, *snap)
		}
	}

	return report, nil
}

func mapSnapshot(limitID, limitName string, src *rateLimitStatusDetails, credits *Credits) *Snapshot {
	if src == nil {
		return nil
	}
	primary := mapWindow(src.PrimaryWindow)
	secondary := mapWindow(src.SecondaryWindow)
	if primary == nil && secondary == nil && credits == nil {
		return nil
	}
	return &Snapshot{
		LimitID:   limitID,
		LimitName: limitName,
		Primary:   primary,
		Secondary: secondary,
		Credits:   credits,
	}
}

func mapWindow(window *rateLimitWindowSnapshot) *Window {
	if window == nil {
		return nil
	}
	minutes := window.LimitWindowSeconds / 60
	if window.LimitWindowSeconds > 0 && window.LimitWindowSeconds%60 != 0 {
		minutes++
	}
	return &Window{
		UsedPercent:   window.UsedPercent,
		WindowMinutes: minutes,
		ResetsAtUnix:  window.ResetAt,
	}
}

func mapCredits(details *creditStatusDetails) *Credits {
	if details == nil {
		return nil
	}
	return &Credits{
		HasCredits: details.HasCredits,
		Unlimited:  details.Unlimited,
		Balance:    strings.TrimSpace(details.Balance),
	}
}

func (r *UsageReport) PreferredSnapshot() *Snapshot {
	if r == nil || len(r.Snapshots) == 0 {
		return nil
	}
	for i := range r.Snapshots {
		if strings.EqualFold(r.Snapshots[i].LimitID, defaultLimitID) {
			return &r.Snapshots[i]
		}
	}
	return &r.Snapshots[0]
}

func FormatDetailedReport(report *UsageReport, now time.Time) string {
	if report == nil {
		return "OpenAI Codex usage: unavailable."
	}

	var lines []string
	lines = append(lines, "OpenAI Codex usage")
	if report.PlanType != "" {
		lines = append(lines, "Plan: "+report.PlanType)
	}
	if len(report.Snapshots) == 0 {
		lines = append(lines, "Limits: unavailable")
		return strings.Join(lines, "\n")
	}

	for i, snap := range report.Snapshots {
		if i > 0 {
			lines = append(lines, "")
		}
		prefix := snap.LimitID
		if prefix == "" {
			prefix = defaultLimitID
		}
		lines = append(lines, "Limit bucket: "+prefix)
		if snap.Primary != nil {
			lines = append(lines, formatWindowLine(labelForWindow(snap.Primary.WindowMinutes, "5h"), snap.Primary, now))
		}
		if snap.Secondary != nil {
			lines = append(lines, formatWindowLine(labelForWindow(snap.Secondary.WindowMinutes, "weekly"), snap.Secondary, now))
		}
		if snap.Credits != nil && snap.Credits.HasCredits {
			creditsText := "Credits: available"
			if snap.Credits.Unlimited {
				creditsText = "Credits: unlimited"
			} else if snap.Credits.Balance != "" {
				creditsText = "Credits: " + snap.Credits.Balance
			}
			lines = append(lines, creditsText)
		}
	}

	return strings.Join(lines, "\n")
}

func FormatStatusSummary(report *UsageReport, now time.Time) string {
	if report == nil {
		return "Codex limits unavailable."
	}
	snapshot := report.PreferredSnapshot()
	if snapshot == nil {
		return "Codex limits unavailable."
	}

	var parts []string
	parts = append(parts, "Codex limits:")
	if snapshot.Primary != nil {
		parts = append(parts, formatWindowSummary(labelForWindow(snapshot.Primary.WindowMinutes, "5h"), snapshot.Primary, now))
	}
	if snapshot.Secondary != nil {
		parts = append(parts, formatWindowSummary(labelForWindow(snapshot.Secondary.WindowMinutes, "weekly"), snapshot.Secondary, now))
	}
	if len(parts) == 1 {
		return "Codex limits unavailable."
	}
	return strings.Join(parts, " ")
}

func formatWindowLine(label string, w *Window, now time.Time) string {
	return fmt.Sprintf("%s: %.0f%% used, %s", label, w.UsedPercent, resetText(w.ResetsAtUnix, now))
}

func formatWindowSummary(label string, w *Window, now time.Time) string {
	return fmt.Sprintf("%s %.0f%% (%s)", label, w.UsedPercent, resetText(w.ResetsAtUnix, now))
}

func resetText(resetsAtUnix int64, now time.Time) string {
	if resetsAtUnix <= 0 {
		return "reset unknown"
	}
	reset := time.Unix(resetsAtUnix, 0).In(now.Location())
	return "resets " + reset.Format("Mon 15:04")
}

func labelForWindow(minutes int64, fallback string) string {
	if minutes <= 0 {
		return fallback
	}
	if minutes == 7*24*60 {
		return "weekly"
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}

type usagePayload struct {
	PlanType             string                       `json:"plan_type"`
	RateLimit            *rateLimitStatusDetails      `json:"rate_limit"`
	AdditionalRateLimits []additionalRateLimitDetails `json:"additional_rate_limits"`
	Credits              *creditStatusDetails         `json:"credits"`
}

type additionalRateLimitDetails struct {
	LimitName      string                  `json:"limit_name"`
	MeteredFeature string                  `json:"metered_feature"`
	RateLimit      *rateLimitStatusDetails `json:"rate_limit"`
}

type rateLimitStatusDetails struct {
	PrimaryWindow   *rateLimitWindowSnapshot `json:"primary_window"`
	SecondaryWindow *rateLimitWindowSnapshot `json:"secondary_window"`
}

type rateLimitWindowSnapshot struct {
	UsedPercent        float64 `json:"used_percent"`
	LimitWindowSeconds int64   `json:"limit_window_seconds"`
	ResetAt            int64   `json:"reset_at"`
}

type creditStatusDetails struct {
	HasCredits bool   `json:"has_credits"`
	Unlimited  bool   `json:"unlimited"`
	Balance    string `json:"balance"`
}
