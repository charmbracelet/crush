package planmode

import "strings"

const (
	ProposedPlanOpenTag  = "<proposed_plan>"
	ProposedPlanCloseTag = "</proposed_plan>"
)

func ExtractProposedPlan(content string) (string, bool) {
	start := strings.Index(content, ProposedPlanOpenTag)
	if start == -1 {
		return "", false
	}
	start += len(ProposedPlanOpenTag)
	end := strings.Index(content[start:], ProposedPlanCloseTag)
	if end == -1 {
		return "", false
	}
	plan := strings.TrimSpace(content[start : start+end])
	if plan == "" {
		return "", false
	}
	return plan, true
}

func WrapProposedPlan(plan string) string {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		return ProposedPlanOpenTag + "\n" + ProposedPlanCloseTag
	}
	return ProposedPlanOpenTag + "\n" + plan + "\n" + ProposedPlanCloseTag
}

func BuildExecutionPrompt(plan string) string {
	wrapped := WrapProposedPlan(plan)
	return strings.TrimSpace("Execute the approved plan below. You are no longer in Plan Mode, so you should implement it now.\n\n" + wrapped)
}
