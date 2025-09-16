package prompt

import (
	_ "embed"
	"fmt"

	"github.com/nom-nom-hub/blush/internal/config"
)

func ProjectManagerPrompt(p string, contextFiles ...string) string {
	basePrompt := string(projectManagerPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func ArchitectPrompt(p string, contextFiles ...string) string {
	basePrompt := string(architectPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func FrontendPrompt(p string, contextFiles ...string) string {
	basePrompt := string(frontendPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func BackendPrompt(p string, contextFiles ...string) string {
	basePrompt := string(backendPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func DatabasePrompt(p string, contextFiles ...string) string {
	basePrompt := string(databasePrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func DevOpsPrompt(p string, contextFiles ...string) string {
	basePrompt := string(devopsPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func QAPrompt(p string, contextFiles ...string) string {
	basePrompt := string(qaPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func SecurityPrompt(p string, contextFiles ...string) string {
	basePrompt := string(securityPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func DocumentationPrompt(p string, contextFiles ...string) string {
	basePrompt := string(documentationPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

func ReviewerPrompt(p string, contextFiles ...string) string {
	basePrompt := string(reviewerPrompt)
	
	envInfo := getEnvironmentInfo()
	basePrompt = fmt.Sprintf("%s\n\n%s", basePrompt, envInfo)

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

//go:embed agents/project_manager.md
var projectManagerPrompt []byte

//go:embed agents/architect.md
var architectPrompt []byte

//go:embed agents/frontend.md
var frontendPrompt []byte

//go:embed agents/backend.md
var backendPrompt []byte

//go:embed agents/database.md
var databasePrompt []byte

//go:embed agents/devops.md
var devopsPrompt []byte

//go:embed agents/qa.md
var qaPrompt []byte

//go:embed agents/security.md
var securityPrompt []byte

//go:embed agents/documentation.md
var documentationPrompt []byte

//go:embed agents/reviewer.md
var reviewerPrompt []byte