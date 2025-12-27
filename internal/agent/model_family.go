package agent

import "strings"

type ModelFamily string

const (
	ModelFamilyAnthropic ModelFamily = "anthropic"
	ModelFamilyOpenAI    ModelFamily = "openai"
	ModelFamilyGoogle    ModelFamily = "google"
	ModelFamilyDefault   ModelFamily = "default"
)

// DetectModelFamily determines the model family based on the model name.
func DetectModelFamily(modelName string) ModelFamily {
	modelLower := strings.ToLower(modelName)

	if strings.Contains(modelLower, "claude") {
		return ModelFamilyAnthropic
	}

	if strings.HasPrefix(modelLower, "gpt-") ||
		strings.HasPrefix(modelLower, "o1-") ||
		strings.HasPrefix(modelLower, "o3-") ||
		strings.HasPrefix(modelLower, "o4-") ||
		strings.Contains(modelLower, "chatgpt") {
		return ModelFamilyOpenAI
	}

	if strings.HasPrefix(modelLower, "gemini") {
		return ModelFamilyGoogle
	}

	return ModelFamilyDefault
}
