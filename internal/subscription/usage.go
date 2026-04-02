package subscription

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/oauth/openai_codex"
	"github.com/charmbracelet/crush/internal/openaicodex"
)

type WindowUsage struct {
	Label        string
	UsedPercent  float64
	ResetsAtUnix int64
}

type UsageReport struct {
	ProviderID  string
	Plan        string
	LimitBucket string
	Windows     []WindowUsage
}

type Fetcher interface {
	ProviderID() string
	Fetch(ctx context.Context, providerCfg config.ProviderConfig) (*UsageReport, error)
}

var fetchers = map[string]Fetcher{
	openaicodex.ProviderID: openAICodexFetcher{},
}

func HasFetcher(providerID string) bool {
	_, ok := fetchers[providerID]
	return ok
}

func FetchUsage(ctx context.Context, providerID string, providerCfg config.ProviderConfig) (*UsageReport, error) {
	fetcher, ok := fetchers[providerID]
	if !ok {
		return nil, fmt.Errorf("no subscription usage fetcher for provider %q", providerID)
	}
	return fetcher.Fetch(ctx, providerCfg)
}

func SidebarLines(report *UsageReport, now time.Time) []string {
	if report == nil {
		return []string{"Limits unavailable"}
	}

	var lines []string
	if report.Plan != "" {
		lines = append(lines, "Plan: "+report.Plan)
	}
	if report.LimitBucket != "" {
		lines = append(lines, "Limit bucket: "+report.LimitBucket)
	}
	for _, w := range report.Windows {
		lines = append(lines, formatWindowLines(w, now)...)
	}

	if len(lines) == 0 {
		return []string{"Limits unavailable"}
	}
	return lines
}

func formatWindowLines(w WindowUsage, now time.Time) []string {
	first := fmt.Sprintf("%s: %.0f%% used", w.Label, w.UsedPercent)
	indent := strings.Repeat(" ", len(w.Label)+2)
	second := indent + resetText(w.ResetsAtUnix, now)
	return []string{first, second}
}

type openAICodexFetcher struct{}

func (openAICodexFetcher) ProviderID() string {
	return openaicodex.ProviderID
}

func (openAICodexFetcher) Fetch(ctx context.Context, providerCfg config.ProviderConfig) (*UsageReport, error) {
	if providerCfg.OAuthToken == nil || strings.TrimSpace(providerCfg.OAuthToken.AccessToken) == "" {
		return nil, fmt.Errorf("missing OAuth token")
	}

	accountID := strings.TrimSpace(providerCfg.ExtraHeaders["chatgpt-account-id"])
	if accountID == "" {
		parsed, err := openai_codex.ExtractAccountID(providerCfg.OAuthToken.AccessToken)
		if err != nil {
			return nil, fmt.Errorf("resolve account ID: %w", err)
		}
		accountID = parsed
	}

	raw, err := openaicodex.FetchUsage(ctx, openaicodex.UsageRequest{
		BaseURL:     providerCfg.BaseURL,
		AccessToken: providerCfg.OAuthToken.AccessToken,
		AccountID:   accountID,
	})
	if err != nil {
		return nil, err
	}

	snap := raw.PreferredSnapshot()
	if snap == nil {
		return nil, fmt.Errorf("usage snapshot unavailable")
	}

	report := &UsageReport{
		ProviderID:  openaicodex.ProviderID,
		Plan:        raw.PlanType,
		LimitBucket: snap.LimitID,
	}
	if snap.Primary != nil {
		report.Windows = append(report.Windows, WindowUsage{
			Label:        labelForWindow(snap.Primary.WindowMinutes, "5h"),
			UsedPercent:  snap.Primary.UsedPercent,
			ResetsAtUnix: snap.Primary.ResetsAtUnix,
		})
	}
	if snap.Secondary != nil {
		report.Windows = append(report.Windows, WindowUsage{
			Label:        labelForWindow(snap.Secondary.WindowMinutes, "weekly"),
			UsedPercent:  snap.Secondary.UsedPercent,
			ResetsAtUnix: snap.Secondary.ResetsAtUnix,
		})
	}
	return report, nil
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
