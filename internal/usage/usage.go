// Package usage reads the remaining quota a provider reports for a
// subscription plan. A plugin declares the endpoint and the paths through
// which the meters are found, so surfacing "how much is left" needs no
// Crush code for each new provider.
package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/oauth"
	"github.com/tidwall/gjson"
)

// HTTPClient is used for quota requests. Tests point the spec at a stub
// server rather than replacing the client.
var HTTPClient = &http.Client{Timeout: 15 * time.Second}

// Meter is one allowance reported by the provider: what it covers, how much
// is left, and when it refills.
type Meter struct {
	// Group is the allowance group this meter belongs to, e.g. "Gemini
	// Models", when the provider reports several.
	Group string
	// Label names the allowance within its group, e.g. "Weekly Limit".
	Label string
	// Short is the provider's own compact tag for the meter, such as "5h",
	// for one-line displays.
	Short string
	// Left is the allowance remaining. Values up to 1 are read as a fraction
	// of the limit; larger values are a raw amount, such as a credit balance.
	Left float64
	// ResetAt is when the allowance refills, when the provider says.
	ResetAt time.Time
	// HasReset reports whether ResetAt carries a real time.
	HasReset bool
}

// IsFraction reports whether the provider expressed the allowance as a share
// of the limit rather than an amount.
func (m Meter) IsFraction() bool { return m.Left <= 1 }

// Percent is the share of the allowance left, rounded to a whole percent.
func (m Meter) Percent() int { return int(m.Left * 100) }

// Tag names the meter as briefly as the provider allows: its short tag when
// it has one, otherwise its label.
func (m Meter) Tag() string {
	if m.Short != "" {
		return m.Short
	}
	return m.Label
}

// Summary renders the meter as one line for a terminal or sidebar.
func (m Meter) Summary() string {
	var out strings.Builder
	if m.Group != "" {
		out.WriteString(m.Group)
		out.WriteString(" · ")
	}
	out.WriteString(m.Label)
	out.WriteString(": ")
	if m.IsFraction() {
		fmt.Fprintf(&out, "%d%% left", m.Percent())
	} else {
		fmt.Fprintf(&out, "%g left", m.Left)
	}
	if m.HasReset {
		out.WriteString(" · resets ")
		out.WriteString(humanUntil(time.Until(m.ResetAt)))
	}
	return out.String()
}

// Fetch reads the meters declared by the spec. bearer is the provider's
// credential (an OAuth access token, or the configured API key), and headers
// are the provider's extra headers, which some gateways demand: a quota
// endpoint can reject a request that does not identify a supported client.
func Fetch(ctx context.Context, spec *oauth.UsageSpec, bearer string, headers map[string]string) ([]Meter, error) {
	if spec == nil || spec.URL == "" {
		return nil, fmt.Errorf("provider declares no usage endpoint")
	}

	method := strings.ToUpper(strings.TrimSpace(orDefault(spec.Method, http.MethodGet)))
	body := io.Reader(strings.NewReader(orDefault(spec.Body, "{}")))
	if method == http.MethodGet {
		body = nil
	}

	req, err := http.NewRequestWithContext(ctx, method, spec.URL, body)
	if err != nil {
		return nil, err
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	req.Header.Set("Accept", "application/json")
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch usage: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read usage response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage endpoint returned %d: %s",
			resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var meters []Meter
	doc := string(payload)
	if spec.Groups != "" {
		for _, group := range iterate(doc, spec.Groups) {
			prefix := group.Get(spec.GroupLabelPath()).String()
			meters = append(meters, collect(group.Raw, spec, prefix)...)
		}
	} else {
		meters = append(meters, collect(doc, spec, "")...)
	}
	if len(meters) == 0 {
		return nil, fmt.Errorf("usage endpoint reported no meters")
	}
	return meters, nil
}

// iterate resolves a gjson path to the list of values it names. An array
// yields its elements, a single object yields itself, and MetersAtRoot means
// the document is the value.
func iterate(raw, path string) []gjson.Result {
	if path == "" || path == oauth.MetersAtRoot {
		parsed := gjson.Parse(raw)
		if parsed.IsArray() {
			return parsed.Array()
		}
		return []gjson.Result{parsed}
	}
	result := gjson.Get(raw, path)
	switch {
	case result.IsArray():
		return result.Array()
	case result.Type == gjson.JSON:
		return []gjson.Result{result}
	default:
		return nil
	}
}

// collect walks the meters under a document, labelling them with the group
// they belong to.
func collect(raw string, spec *oauth.UsageSpec, group string) []Meter {
	var meters []Meter
	for _, item := range iterate(raw, spec.MetersPath()) {
		meter := Meter{
			Group: group,
			Label: item.Get(spec.LabelPath()).String(),
			Short: item.Get(spec.WindowPath()).String(),
			Left:  item.Get(spec.RemainingPath()).Float(),
		}
		// A plan that reports how much has been spent states the other side
		// of the same allowance; report the complement.
		if spent := spec.SpentPercentPath(); spent != "" {
			meter.Left = max(0, 1-item.Get(spent).Float()/100)
		}
		if meter.Label == "" {
			meter.Label = spec.TitleOr()
		}
		if reset := item.Get(spec.ResetPath()).String(); reset != "" {
			if at, err := time.Parse(time.RFC3339, reset); err == nil {
				meter.ResetAt = at
				meter.HasReset = true
			}
		}
		meters = append(meters, meter)
	}
	return meters
}

// ForModel returns the meters that apply to a model id: those in the group the
// spec maps the model to.
//
// A spec with no mapping reports one plan-wide allowance, so every meter
// applies. A spec that does map families but has none for this model id shows
// nothing: guessing wide would put one family's limits beside another's model,
// which reads as the plan being spent when it is not.
func ForModel(meters []Meter, spec *oauth.UsageSpec, modelID string) []Meter {
	if spec == nil || len(spec.ModelGroups) == 0 {
		return meters
	}
	group := GroupForModel(spec, modelID)
	if group == "" {
		return nil
	}
	var matched []Meter
	for _, meter := range meters {
		if strings.Contains(strings.ToLower(meter.Group), group) {
			matched = append(matched, meter)
		}
	}
	if len(matched) == 0 {
		return meters
	}
	return matched
}

// GroupForModel resolves the group a model id belongs to, as a lowercase
// substring of a group label, or "" when the spec maps nothing to it. The
// longest matching prefix wins, so "claude-3" can be distinguished from
// "claude".
func GroupForModel(spec *oauth.UsageSpec, modelID string) string {
	if spec == nil || len(spec.ModelGroups) == 0 || modelID == "" {
		return ""
	}
	prefixes := make([]string, 0, len(spec.ModelGroups))
	for prefix := range spec.ModelGroups {
		prefixes = append(prefixes, prefix)
	}
	slices.SortFunc(prefixes, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}
		return strings.Compare(a, b)
	})
	model := strings.ToLower(modelID)
	for _, prefix := range prefixes {
		if strings.HasPrefix(model, strings.ToLower(prefix)) {
			return strings.ToLower(spec.ModelGroups[prefix])
		}
	}
	return ""
}

// humanUntil shortens a duration for one-line display: "4h 17m", "6d 23h".
func humanUntil(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	const day = 24 * time.Hour
	var parts []string
	if d >= day {
		parts = append(parts, fmt.Sprintf("%dd", int(d/day)))
		d -= time.Duration(int(d/day)) * day
	}
	if d >= time.Hour {
		parts = append(parts, fmt.Sprintf("%dh", int(d/time.Hour)))
		d -= time.Duration(int(d/time.Hour)) * time.Hour
	}
	if len(parts) < 2 && d >= time.Minute {
		parts = append(parts, fmt.Sprintf("%dm", int(d/time.Minute)))
	}
	return "in " + strings.Join(parts, " ")
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
