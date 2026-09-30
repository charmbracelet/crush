package router

// QuestionSpec is one question in a System One request. Type is "choice",
// "score", or "noul". Criteria shape depends on Type: choice takes
// map[string]string (option -> description), score takes []string
// (ordered levels), noul takes an optional map[string]string{"true", "false"}
// or nil.
type QuestionSpec struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions,omitempty"`
	Criteria     any    `json:"criteria,omitempty"`
}

// Answer is one answer in a System One response. Only the fields matching
// the question's Type are populated.
type Answer struct {
	Type       string  `json:"type"`
	Choice     string  `json:"choice,omitempty"`
	Score      float64 `json:"score,omitempty"`
	Noul       float64 `json:"noul,omitempty"`
	Confidence float64 `json:"confidence,omitempty"`
}

type decideRequest struct {
	Model     string                  `json:"model"`
	State     string                  `json:"state"`
	Questions map[string]QuestionSpec `json:"questions"`
}

// nestedDecideRequest is the same request with the state and questions
// under "input", the shape Cloudflare Workers AI expects.
type nestedDecideRequest struct {
	Model string                   `json:"model"`
	Input nestedDecideRequestInput `json:"input"`
}

type nestedDecideRequestInput struct {
	State     string                  `json:"state"`
	Questions map[string]QuestionSpec `json:"questions"`
}

type decideResponse struct {
	Answers map[string]Answer `json:"answers"`
	// Usage carries the classifier call's own cost, when the backend
	// reports one (OpenRouter's alpha Decisions API does; a local
	// System One-compatible server may not, in which case Cost is 0 —
	// self-hosting the classifier has no per-call $ cost to report).
	Usage *DecideUsage `json:"usage,omitempty"`
}

// DecideUsage is the classifier call's own token usage and cost, as
// reported by the backend — distinct from the usage of the model the
// router's decision then routes the actual message to.
type DecideUsage struct {
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	Cost         float64 `json:"cost,omitempty"`
}
