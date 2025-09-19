package prompt

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
)

func CoderPrompt(p string, contextFiles ...string) string {
	var basePrompt string

	basePrompt = string(anthropicCoderPrompt)
	switch p {
	case string(catwalk.InferenceProviderOpenAI):
		// seems to behave better
		basePrompt = string(coderV2Prompt)
	case string(catwalk.InferenceProviderGemini):
		basePrompt = string(geminiCoderPrompt)
	}
	if ok, _ := strconv.ParseBool(os.Getenv("CRUSH_CODER_V2")); ok {
		basePrompt = string(coderV2Prompt)
	}
	envInfo := getEnvironmentInfo()

	basePrompt = fmt.Sprintf("%s\n\n%s\n%s", basePrompt, envInfo, lspInformation())

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n Make sure to follow the instructions in the context below\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

//go:embed anthropic.md
var anthropicCoderPrompt []byte

//go:embed gemini.md
var geminiCoderPrompt []byte

//go:embed v2.md
var coderV2Prompt []byte

func getEnvironmentInfo() string {
	cwd := config.Get().WorkingDir()
	isGit := isGitRepo(cwd)
	platform := runtime.GOOS
	date := time.Now().Format("1/2/2006")
	output, _ := tools.ListDirectoryTree(cwd, nil)
	return fmt.Sprintf(`Here is useful information about the environment you are running in:
<env>
Working directory: %s
Is directory a git repo: %s
Platform: %s
Today's date: %s
</env>
<project>
%s
</project>
		`, cwd, boolToYesNo(isGit), platform, date, output)
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func lspInformation() string {
	cfg := config.Get()
	hasLSP := false
	for _, v := range cfg.LSP {
		if !v.Disabled {
			hasLSP = true
			break
		}
	}
	if !hasLSP {
		return ""
	}
	return `# LSP Information
Tools that support it will also include useful diagnostics such as linting and typechecking.
- These diagnostics will be automatically enabled when you run the tool, and will be displayed in the output at the bottom within the <file_diagnostics></file_diagnostics> and <project_diagnostics></project_diagnostics> tags.
- Take necessary actions to fix the issues.
- You should ignore diagnostics of files that you did not change or are not related or caused by your changes unless the user explicitly asks you to fix them.
`
}

func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
