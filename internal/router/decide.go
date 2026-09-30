package router

import (
	"fmt"
	"math"
)

const (
	reasoningQuestionID = "reasoning_effort"
	modelQuestionID     = "model_choice"
)

// reasoningLevels are the score criteria sent to the backend, in order.
// MapDecision maps the returned mean-level score back onto this slice.
var reasoningLevels = []string{"low", "medium", "high"}

// modelFamilyDescriptions maps OpenRouter model-family prefixes to a
// human-readable capability description, sourced verbatim from models.dev
// (https://models.dev/api.json, an open-source public database of model
// metadata), which curates one consistent description per capability tier
// (flagship/balanced/fast/compact/advanced) across providers. Fetched here
// once and pinned rather than queried live: the router calls this on every
// user message, and the spec requires it to fail open and stay fast — a
// second outbound HTTP call to models.dev before every message would add
// another latency/failure point for data that only changes when a new
// model ships. Re-derive this table by hand from models.dev/api.json when
// adding new families; it does not update itself.
var modelFamilyDescriptions = map[string]string{
	// Anthropic Claude.
	"anthropic/claude-opus":   "Flagship Claude model for deep reasoning, coding, and long-horizon agents",
	"anthropic/claude-sonnet": "Balanced Claude model for coding, analysis, agent workflows, and cost control",
	"anthropic/claude-haiku":  "Fast Claude model for responsive assistance, classification, and lightweight agents",

	// OpenAI GPT.
	"openai/gpt-5-mini":    "Compact GPT model for low-latency assistance and high-volume workloads",
	"openai/gpt-4o-mini":   "Compact GPT model for low-latency assistance and high-volume workloads",
	"openai/gpt-3.5-turbo": "Compact GPT model for low-latency assistance and high-volume workloads",
	"openai/gpt-5":         "GPT model for general reasoning, writing, coding, and tool-assisted tasks",
	"openai/gpt-4o":        "GPT model for general reasoning, writing, coding, and tool-assisted tasks",
	"openai/gpt-4-turbo":   "GPT model for general reasoning, writing, coding, and tool-assisted tasks",

	// Google Gemini.
	"google/gemini-2.5-pro":        "Advanced Gemini model for complex reasoning, coding, and multimodal analysis",
	"google/gemini-2.0-pro":        "Advanced Gemini model for complex reasoning, coding, and multimodal analysis",
	"google/gemini-1.5-pro":        "Advanced Gemini model for complex reasoning, coding, and multimodal analysis",
	"google/gemini-2.5-flash-lite": "Low-latency Gemini model for high-volume multimodal and agent workloads",
	"google/gemini-2.0-flash-lite": "Low-latency Gemini model for high-volume multimodal and agent workloads",
	"google/gemini-2.5-flash":      "Fast Gemini model balancing multimodal reasoning, tool use, and cost",
	"google/gemini-2.0-flash":      "Fast Gemini model balancing multimodal reasoning, tool use, and cost",
	"google/gemini-1.5-flash":      "Fast Gemini model balancing multimodal reasoning, tool use, and cost",

	// Meta Llama.
	"meta-llama/llama-4-maverick": "Open multimodal Llama model for strong reasoning and fast responses",
	"meta-llama/llama-4-scout":    "Open multimodal Llama model for long-context analysis and efficient agents",
	"meta-llama/llama-3.3":        "Compact Llama instruction model for fast chat and local deployment",
	"meta-llama/llama-3.1":        "Compact Llama instruction model for fast chat and local deployment",
}

// describeModelForRouter looks up modelID's capability description in
// modelFamilyDescriptions for the System One router's model_choice question,
// so the classifier (Jev, Kev, Laya, etc.) has more to go on than a bare
// model id. Matching is by longest known prefix, since OpenRouter ids carry
// a version suffix the table does not enumerate (e.g.
// "anthropic/claude-opus-4.5" matches the "anthropic/claude-opus" entry).
// Returns modelID itself when no family matches (fail-open: the router
// backend still works, just with a less informative criteria label).
func describeModelForRouter(modelID string) string {
	if desc, ok := modelFamilyDescriptions[modelID]; ok {
		return desc
	}

	bestPrefix := ""
	bestDesc := ""
	for prefix, desc := range modelFamilyDescriptions {
		if len(prefix) <= len(bestPrefix) {
			continue
		}
		if len(modelID) > len(prefix) && modelID[:len(prefix)] == prefix {
			bestPrefix = prefix
			bestDesc = desc
		}
	}
	if bestPrefix != "" {
		return bestDesc
	}

	return modelID
}

// BuildQuestions builds the System One request body for one user prompt:
// how much reasoning effort it needs, and — when modelPool is non-empty —
// which of the configured OpenRouter models to use. The router never
// chooses which agent handles the prompt — only, optionally, which model
// and how much reasoning effort to apply to the currently active agent.
func BuildQuestions(modelPool []string) map[string]QuestionSpec {
	questions := map[string]QuestionSpec{
		reasoningQuestionID: {
			Type:         "score",
			Instructions: "How much reasoning effort does this request need?",
			Criteria:     reasoningLevels,
		},
	}
	if len(modelPool) > 0 {
		criteria := make(map[string]string, len(modelPool))
		for _, id := range modelPool {
			criteria[id] = describeModelForRouter(id)
		}
		questions[modelQuestionID] = QuestionSpec{
			Type:         "choice",
			Instructions: "Which model should handle this request?",
			Criteria:     criteria,
		}
	}
	return questions
}

// Decision is the router's answer for one user message: which reasoning
// effort to apply, how confident the backend was in that score, and,
// when a model pool is configured, which model to use. ModelID is empty
// when no model_choice question was asked (empty pool) or answered.
type Decision struct {
	ReasoningEffort string
	Confidence      float64
	LowConfidence   bool
	ModelID         string
	// ModelConfidence is the model_choice answer's own confidence —
	// distinct from Confidence/LowConfidence above, which come from the
	// reasoning_effort answer only. A classifier can be well-calibrated
	// on reasoning effort while essentially guessing on model_choice (it
	// was never trained to judge unfamiliar model ids), so the two must
	// gate independently: the caller should not apply a model override
	// just because the *effort* answer was confident. 0 when no
	// model_choice question was asked or answered.
	ModelConfidence float64
	// CallCost is the classifier call's own cost in USD, as reported by
	// the backend (0 for a local server that doesn't report one, or for
	// any backend that simply doesn't bill per call). Set regardless of
	// whether this decision ends up applied — the classifier call itself
	// was made and cost this either way.
	CallCost float64
}

// MapDecision turns a System One response into a Decision. The
// reasoning_effort answer is required; model_choice is optional — its
// absence (empty pool) leaves Decision.ModelID empty, not an error. The
// score is clamped into range instead of erroring, since backends are
// not guaranteed to bound their own output.
func MapDecision(answers map[string]Answer, confidenceThreshold float64) (Decision, error) {
	effortAnswer, ok := answers[reasoningQuestionID]
	if !ok || effortAnswer.Type != "score" {
		return Decision{}, fmt.Errorf("router: missing or invalid %q answer", reasoningQuestionID)
	}

	idx := int(math.Round(effortAnswer.Score))
	if idx < 0 {
		idx = 0
	}
	if idx > len(reasoningLevels)-1 {
		idx = len(reasoningLevels) - 1
	}

	decision := Decision{
		ReasoningEffort: reasoningLevels[idx],
		Confidence:      effortAnswer.Confidence,
		LowConfidence:   effortAnswer.Confidence < confidenceThreshold,
	}

	if modelAnswer, ok := answers[modelQuestionID]; ok && modelAnswer.Type == "choice" {
		decision.ModelID = modelAnswer.Choice
		decision.ModelConfidence = modelAnswer.Confidence
	}

	return decision, nil
}
