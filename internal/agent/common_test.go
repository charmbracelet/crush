package agent

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
	"charm.land/x/vcr"
	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/csync"
	"github.com/charmbracelet/crush/internal/db"
	"github.com/charmbracelet/crush/internal/filetracker"
	"github.com/charmbracelet/crush/internal/history"
	"github.com/charmbracelet/crush/internal/lsp"
	"github.com/charmbracelet/crush/internal/message"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"

	_ "github.com/joho/godotenv/autoload"
)

// fakeEnv is an environment for testing.
type fakeEnv struct {
	workingDir  string
	recordedDir string
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	filetracker *filetracker.Service
	lspClients  *csync.Map[string, *lsp.Client]
}

type builderFunc func(t *testing.T, r *vcr.Recorder) (fantasy.LanguageModel, error)

type modelPair struct {
	name       string
	largeModel builderFunc
	smallModel builderFunc
}

func hyperBuilder(model string) builderFunc {
	return func(t *testing.T, r *vcr.Recorder) (fantasy.LanguageModel, error) {
		provider, err := openaicompat.New(
			openaicompat.WithBaseURL("https://hyper.charm.land/v1"),
			openaicompat.WithAPIKey(os.Getenv("CRUSH_HYPER_API_KEY")),
			openaicompat.WithHTTPClient(&http.Client{Transport: r}),
		)
		if err != nil {
			return nil, err
		}
		return provider.LanguageModel(t.Context(), model)
	}
}

func testEnv(t *testing.T) fakeEnv {
	// Resolve symlinks because the temporary root is one on macOS, and tools
	// report resolved paths.
	workingDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	conn, err := db.Connect(t.Context(), t.TempDir())
	require.NoError(t, err)

	q := db.New(conn)
	sessions := session.NewService(q, conn)
	messages := message.NewService(q)

	permissions := permission.NewPermissionService(workingDir, []string{}, nil)
	// Auto-approve every tool call so headless test/record runs never block
	// on a permission prompt.
	permissions.SetPermissionMode(permission.PermissionModeSysadmin)
	history := history.NewService(q, conn)
	filetrackerService := filetracker.NewService(q)
	lspClients := csync.NewMap[string, *lsp.Client]()

	t.Cleanup(func() {
		conn.Close()
	})

	return fakeEnv{
		workingDir,
		"/tmp/crush-test/" + t.Name(),
		sessions,
		messages,
		permissions,
		history,
		&filetrackerService,
		lspClients,
	}
}

func testSessionAgent(env fakeEnv, large, small fantasy.LanguageModel, systemPrompt string, tools ...fantasy.AgentTool) SessionAgent {
	largeModel := Model{
		Model: large,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
	}
	smallModel := Model{
		Model: small,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
	}
	agent := NewSessionAgent(SessionAgentOptions{
		Models:       fixedModels(largeModel, smallModel),
		SystemPrompt: systemPrompt,
		Sessions:     env.sessions,
		Messages:     env.messages,
		Tools:        tools,
	})
	return agent
}

func coderAgent(r *vcr.Recorder, env fakeEnv, large, small fantasy.LanguageModel) (SessionAgent, error) {
	fixedTime := func() time.Time {
		t, _ := time.Parse("1/2/2006", "1/1/2025")
		return t
	}
	prompt, err := coderPrompt(
		prompt.WithTimeFunc(fixedTime),
		prompt.WithPlatform("linux"),
		prompt.WithWorkingDir(env.recordedDir),
	)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Init(context.Background(), env.workingDir, "", false)
	if err != nil {
		return nil, err
	}

	// NOTE(@andreynering): Set a fixed config to ensure cassettes match
	// independently of user config on `$HOME/.config/crush/crush.json`.
	cfg.Config().Options.Attribution = &config.Attribution{
		TrailerStyle:  "co-authored-by",
		GeneratedWith: true,
	}

	// Clear some fields to avoid issues with VCR cassette matching.
	cfg.Config().Options.SkillsPaths = nil
	cfg.Config().Options.DisabledSkills = []string{"crush-config"}
	cfg.Config().Options.ContextPaths = nil
	cfg.Config().Options.GlobalContextPaths = nil
	cfg.Config().LSP = nil

	systemPrompt, err := prompt.Build(context.TODO(), large.Provider(), large.Model(), cfg)
	if err != nil {
		return nil, err
	}

	// Get the model name for the bash tool
	modelName := large.Model() // fallback to ID if Name not available
	if model := cfg.Config().GetModel(large.Provider(), large.Model()); model != nil {
		modelName = model.Name
	}

	allTools := []fantasy.AgentTool{
		tools.NewBashTool(env.permissions, env.workingDir, cfg.Config().Options.DataDirectory, cfg.Config().Options.Attribution, modelName, cfg.Config().Options.GetMaxBackgroundJobs(), cfg.Config().Permissions),
		tools.NewDownloadTool(env.permissions, env.workingDir, r.GetDefaultClient()),
		tools.NewEditTool(nil, env.permissions, env.history, *env.filetracker, env.workingDir),
		tools.NewMultiEditTool(nil, env.permissions, env.history, *env.filetracker, env.workingDir),
		tools.NewFetchTool(env.permissions, env.workingDir, r.GetDefaultClient()),
		tools.NewGlobTool(env.workingDir, cfg.Config().Tools.Glob),
		tools.NewGrepTool(env.workingDir, cfg.Config().Tools.Grep),
		tools.NewLsTool(env.permissions, env.workingDir, cfg.Config().Tools.Ls),
		tools.NewSourcegraphTool(r.GetDefaultClient()),
		tools.NewViewTool(nil, env.permissions, *env.filetracker, nil, env.workingDir),
		tools.NewWriteTool(nil, env.permissions, env.history, *env.filetracker, env.workingDir),
	}

	for i, tool := range allTools {
		allTools[i] = recordedPathTool{tool, env}
	}

	return testSessionAgent(env, large, small, systemPrompt, allTools...), nil
}

// recordedPathTool runs a tool in the private workspace while the model, and
// so the cassettes, only ever see the recorded path.
type recordedPathTool struct {
	fantasy.AgentTool
	env fakeEnv
}

func (r recordedPathTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	call.Input = strings.ReplaceAll(call.Input, r.env.recordedDir, r.env.workingDir)
	resp, err := r.AgentTool.Run(ctx, call)
	resp.Content = strings.ReplaceAll(resp.Content, r.env.workingDir, r.env.recordedDir)
	return resp, err
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

// fixedModels returns a model source that always hands out the same pair.
func fixedModels(large, small Model) func() ModelSet {
	return func() ModelSet { return ModelSet{Large: large, Small: small} }
}

// noModels is the model source for tests that never reach a provider.
func noModels() ModelSet { return ModelSet{} }
