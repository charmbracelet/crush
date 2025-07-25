package prompt

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/llm/tools"
)

func CoderPrompt(p string, contextFiles ...string) string {
	var basePrompt string

	if os.Getenv("CRUSH_CODER_V2") == "true" {
		basePrompt = string(baseCoderV2Prompt)
	} else {
		switch p {
		case string(catwalk.InferenceProviderOpenAI):
			basePrompt = string(baseOpenAICoderPrompt)
		case string(catwalk.InferenceProviderGemini), string(catwalk.InferenceProviderVertexAI):
			basePrompt = string(baseGeminiCoderPrompt)
		default:
			basePrompt = string(baseAnthropicCoderPrompt)
		}
	}
	envInfo := getEnvironmentInfo()

	basePrompt = fmt.Sprintf("%s\n\n%s\n%s", basePrompt, envInfo, lspInformation())

	contextContent := getContextFromPaths(config.Get().WorkingDir(), contextFiles)
	slog.Debug("Context content", "Context", contextContent)
	if contextContent != "" {
		return fmt.Sprintf("%s\n\n# Project-Specific Context\n Make sure to follow the instructions in the context below\n%s", basePrompt, contextContent)
	}
	return basePrompt
}

//go:embed v2.md
var baseCoderV2Prompt []byte

//go:embed openai.md
var baseOpenAICoderPrompt []byte

//go:embed anthropic.md
var baseAnthropicCoderPrompt []byte

//go:embed gemini.md
var baseGeminiCoderPrompt []byte

func getEnvironmentInfo() string {
	cwd := config.Get().WorkingDir()
	isGit := isGitRepo(cwd)
	platform := runtime.GOOS
	date := time.Now().Format("1/2/2006")
	ls := tools.NewLsTool(cwd)
	r, _ := ls.Run(context.Background(), tools.ToolCall{
		Input: `{"path":"."}`,
	})
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
		`, cwd, boolToYesNo(isGit), platform, date, r.Content)
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
