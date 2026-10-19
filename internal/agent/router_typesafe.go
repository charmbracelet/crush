package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
)

const typesafeDefaultBaseURL = "https://api.typesafe.ai/v1"

// typesafeRequest is the body of a TypeSafe System One evaluation.
type typesafeRequest struct {
	State     map[string]string           `json:"state"`
	Model     string                      `json:"model"`
	Questions map[string]typesafeQuestion `json:"questions"`
}

type typesafeQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type typesafeResponse struct {
	Answers map[string]struct {
		Choice     string  `json:"choice"`
		Confidence float64 `json:"confidence"`
	} `json:"answers"`
}

// classifyTypeSafe asks a TypeSafe model (e.g. Jev) a single Choice
// question whose options are the router's routes. Jev returns the route
// name directly, so no text parsing is involved.
func (r *classifierRouter) classifyTypeSafe(ctx context.Context, pc config.ProviderConfig, in RouteInput) (string, float64, error) {
	apiKey, err := r.c.cfg.Resolve(pc.APIKey)
	if err != nil {
		return "", 0, fmt.Errorf("resolve typesafe api key: %w", err)
	}
	baseURL, _ := r.c.cfg.Resolve(pc.BaseURL)

	cctx, cancel := context.WithTimeout(ctx, r.cfg.TimeoutDuration())
	defer cancel()
	choice, err := typesafeChoose(cctx, http.DefaultClient, baseURL, apiKey, pc.ExtraHeaders, r.cfg, in)
	return choice, 0, err
}

// typesafeChoose calls the System One endpoint with one Choice question
// over the router's routes and returns the chosen route name.
func typesafeChoose(ctx context.Context, client *http.Client, baseURL, apiKey string, headers map[string]string, rc config.RouterConfig, in RouteInput) (string, error) {
	if baseURL == "" {
		baseURL = typesafeDefaultBaseURL
	}

	criteria := make(map[string]string, len(rc.Routes))
	for _, rt := range rc.Routes {
		criteria[rt.Name] = rt.Description
	}
	instructions := "Which route should handle the user's `request`? Consider `previous_assistant_reply` for short follow-ups."
	if in.Activity != "" {
		instructions = "The assistant is partway through the user's `request`; `work_so_far` is its latest step. Which route should handle the NEXT step?"
	}
	if s := strings.TrimSpace(rc.Instructions); s != "" {
		instructions += " " + s
	}
	state := map[string]string{"request": firstRunes(in.Prompt, routerMaxPromptRunes)}
	if in.PrevAssistant != "" {
		state["previous_assistant_reply"] = lastRunes(in.PrevAssistant, routerMaxPrevRunes)
	}
	if in.Activity != "" {
		state["work_so_far"] = in.Activity
	}
	body, err := json.Marshal(typesafeRequest{
		State: state,
		Model: rc.Classifier.Model,
		Questions: map[string]typesafeQuestion{
			"route": {Type: "choice", Instructions: instructions, Criteria: criteria},
		},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/systemone", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("typesafe: %s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	var out typesafeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("typesafe: decode response: %w", err)
	}
	answer, ok := out.Answers["route"]
	if !ok || answer.Choice == "" {
		return "", fmt.Errorf("typesafe: no route in response")
	}
	return answer.Choice, nil
}
