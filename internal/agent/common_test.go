package agent

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charmbracelet/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/fantasy/ai"
	"github.com/charmbracelet/fantasy/anthropic"
	"github.com/charmbracelet/fantasy/openai"
	"github.com/charmbracelet/fantasy/openaicompat"
	"github.com/charmbracelet/fantasy/openrouter"
	"github.com/stretchr/testify/require"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	_ "github.com/joho/godotenv/autoload"
)

type env struct {
	workingDir  string
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	lspClients  *csync.Map[string, *lsp.Client]
}

type builderFunc func(t *testing.T, r *recorder.Recorder) (ai.LanguageModel, error)

type modelPair struct {
	name       string
	largeModel builderFunc
	smallModel builderFunc
}

func anthropicBuilder(model string) builderFunc {
	return func(_ *testing.T, r *recorder.Recorder) (ai.LanguageModel, error) {
		provider := anthropic.New(
			anthropic.WithAPIKey(os.Getenv("CRUSH_ANTHROPIC_API_KEY")),
			anthropic.WithHTTPClient(&http.Client{Transport: r}),
		)
		return provider.LanguageModel(model)
	}
}

func openaiBuilder(model string) builderFunc {
	return func(_ *testing.T, r *recorder.Recorder) (ai.LanguageModel, error) {
		provider := openai.New(
			openai.WithAPIKey(os.Getenv("CRUSH_OPENAI_API_KEY")),
			openai.WithHTTPClient(&http.Client{Transport: r}),
		)
		return provider.LanguageModel(model)
	}
}

func openRouterBuilder(model string) builderFunc {
	return func(t *testing.T, r *recorder.Recorder) (ai.LanguageModel, error) {
		provider := openrouter.New(
			openrouter.WithAPIKey(os.Getenv("CRUSH_OPENROUTER_API_KEY")),
			openrouter.WithHTTPClient(&http.Client{Transport: r}),
		)
		return provider.LanguageModel(model)
	}
}

func zAIBuilder(model string) builderFunc {
	return func(t *testing.T, r *recorder.Recorder) (ai.LanguageModel, error) {
		provider := openaicompat.New(
			"https://api.z.ai/api/coding/paas/v4",
			openaicompat.WithAPIKey(os.Getenv("CRUSH_ZAI_API_KEY")),
			openaicompat.WithHTTPClient(&http.Client{Transport: r}),
		)
		return provider.LanguageModel(model)
	}
}

func testEnv(t *testing.T) env {
	testDir := filepath.Join("/tmp/crush-test/", t.Name())
	os.RemoveAll(testDir)
	err := os.MkdirAll(testDir, 0o755)
	t.Cleanup(func() {
		os.RemoveAll(testDir)
	})
	require.NoError(t, err)
	workingDir := testDir
	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)
	q := db.New(conn)
	sessions := session.NewService(q)
	messages := message.NewService(q)
	permissions := permission.NewPermissionService(workingDir, true, []string{})
	history := history.NewService(q, conn)
	lspClients := csync.NewMap[string, *lsp.Client]()
	return env{
		workingDir,
		sessions,
		messages,
		permissions,
		history,
		lspClients,
	}
}

func testSessionAgent(env env, large, small ai.LanguageModel, systemPrompt string, tools ...ai.AgentTool) SessionAgent {
	largeModel := Model{
		Model:      large,
		CatwalkCfg: catwalk.Model{
			// todo: add values
		},
	}
	smallModel := Model{
		Model:      small,
		CatwalkCfg: catwalk.Model{
			// todo: add values
		},
	}
	agent := NewSessionAgent(SessionAgentOptions{largeModel, smallModel, systemPrompt, false, env.sessions, env.messages, tools})
	return agent
}

func coderAgent(r *recorder.Recorder, env env, large, small ai.LanguageModel) (SessionAgent, error) {
	fixedTime := func() time.Time {
		t, _ := time.Parse("1/2/2006", "1/1/2025")
		return t
	}
	prompt, err := coderPrompt(
		prompt.WithTimeFunc(fixedTime),
		prompt.WithPlatform("linux"),
		prompt.WithWorkingDir(env.workingDir),
	)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Init(env.workingDir, "", false)
	if err != nil {
		return nil, err
	}

	systemPrompt, err := prompt.Build(large.Provider(), large.Model(), *cfg)
	if err != nil {
		return nil, err
	}
	allTools := []ai.AgentTool{
		tools.NewBashTool(env.permissions, env.workingDir, cfg.Options.Attribution),
		tools.NewDownloadTool(env.permissions, env.workingDir, r.GetDefaultClient()),
		tools.NewEditTool(env.lspClients, env.permissions, env.history, env.workingDir),
		tools.NewMultiEditTool(env.lspClients, env.permissions, env.history, env.workingDir),
		tools.NewFetchTool(env.permissions, env.workingDir, r.GetDefaultClient()),
		tools.NewGlobTool(env.workingDir),
		tools.NewGrepTool(env.workingDir),
		tools.NewLsTool(env.permissions, env.workingDir, cfg.Tools.Ls),
		tools.NewSourcegraphTool(r.GetDefaultClient()),
		tools.NewViewTool(env.lspClients, env.permissions, env.workingDir),
		tools.NewWriteTool(env.lspClients, env.permissions, env.history, env.workingDir),
	}

	return testSessionAgent(env, large, small, systemPrompt, allTools...), nil
}

// createSimpleGoProject creates a simple Go project structure in the given directory.
// It creates a go.mod file and a main.go file with a basic hello world program.
func createSimpleGoProject(t *testing.T, dir string) {
	goMod := `module example.com/testproject

go 1.23
`
	err := os.WriteFile(dir+"/go.mod", []byte(goMod), 0o644)
	require.NoError(t, err)

	mainGo := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
`
	err = os.WriteFile(dir+"/main.go", []byte(mainGo), 0o644)
	require.NoError(t, err)
}
