package agent

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"charm.land/catwalk/v3/pkg/catwalk"
	"charm.land/fantasy"
	"charm.land/fantasy/providers/openaicompat"
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

type builderFunc func(t *testing.T, r cassetteClient) (fantasy.LanguageModel, error)

// cassetteClient is the surface provider builders and tool clients need:
// an HTTP round tripper that can also hand out a default client. The VCR
// recorder satisfies it, and so does the path rewriting wrapper below.
type cassetteClient interface {
	http.RoundTripper
	GetDefaultClient() *http.Client
}

// recordedWorkspace is the workspace path the recorded cassettes were made
// with: the fixed /tmp/crush-test/<name> root testEnv used before #4242.
func recordedWorkspace(t *testing.T) string {
	return filepath.Join("/tmp/crush-test", t.Name())
}

// workspaceRewriter maps the private per-allocation workspace back to the
// recorded path in every outgoing request body. The system prompt carries
// the working directory, so without this rewrite no recorded cassette
// would match a request from a private workspace.
type workspaceRewriter struct {
	next     cassetteClient
	private  string
	recorded string
}

func (w workspaceRewriter) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		rewritten := strings.ReplaceAll(string(body), w.private, w.recorded)
		req.Body = io.NopCloser(strings.NewReader(rewritten))
		req.ContentLength = int64(len(rewritten))
	}
	return w.next.RoundTrip(req)
}

func (w workspaceRewriter) GetDefaultClient() *http.Client {
	return &http.Client{Transport: w}
}

// workspaceTool translates workspace paths around a tool call. Replayed
// tool inputs point at the recorded workspace, which does not exist on a
// private-directory run, so the recorded prefix is swapped for the private
// one before execution; tool output is translated back so the follow-up
// request still matches the cassette.
type workspaceTool struct {
	tool     fantasy.AgentTool
	recorded string
	private  string
}

func (w workspaceTool) Info() fantasy.ToolInfo { return w.tool.Info() }

func (w workspaceTool) ProviderOptions() fantasy.ProviderOptions {
	return w.tool.ProviderOptions()
}

func (w workspaceTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	w.tool.SetProviderOptions(opts)
}

func (w workspaceTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	params.Input = strings.ReplaceAll(params.Input, w.recorded, w.private)
	res, err := w.tool.Run(ctx, params)
	res.Content = strings.ReplaceAll(res.Content, w.private, w.recorded)
	return res, err
}

// mapWorkspaceTools wraps every tool so cassette paths are translated at
// the tool boundary.
func mapWorkspaceTools(tools []fantasy.AgentTool, env fakeEnv) []fantasy.AgentTool {
	mapped := make([]fantasy.AgentTool, 0, len(tools))
	for _, tool := range tools {
		mapped = append(mapped, workspaceTool{
			tool:     tool,
			recorded: filepath.ToSlash(env.recordedDir),
			private:  filepath.ToSlash(env.workingDir),
		})
	}
	return mapped
}

type modelPair struct {
	name       string
	largeModel builderFunc
	smallModel builderFunc
}

func hyperBuilder(model string) builderFunc {
	return func(t *testing.T, r cassetteClient) (fantasy.LanguageModel, error) {
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
	// Each allocation gets its own private workspace (#4242). The old
	// fixed /tmp/crush-test/<name> root let two parallel go test runs of
	// the same test delete each other's files, and it ignored TMPDIR.
	// t.TempDir is unique per call, honors the temporary root, and is
	// cleaned up with the test. The cassettes still carry the recorded
	// path, which is kept alongside for the boundary translators above.
	workingDir := t.TempDir()
	recordedDir := recordedWorkspace(t)

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
		recordedDir,
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

func coderAgent(t *testing.T, r cassetteClient, env fakeEnv, large, small fantasy.LanguageModel) (SessionAgent, error) {
	fixedTime := func() time.Time {
		t, _ := time.Parse("1/2/2006", "1/1/2025")
		return t
	}
	prompt, err := coderPrompt(
		prompt.WithTimeFunc(fixedTime),
		prompt.WithPlatform("linux"),
		prompt.WithWorkingDir(filepath.ToSlash(env.workingDir)),
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

	return testSessionAgent(env, large, small, systemPrompt, mapWorkspaceTools(allTools, env)...), nil
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

// Two allocations of the same named test must not share a workspace:
// parallel go test processes run the same test name, and the fixed
// /tmp/crush-test/<name> path let either one delete the other's files
// (#4242). The temporary root must also be honored.
func TestTestEnvWorkspaceIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}

	// The temporary root is read when t.TempDir is first called, so the
	// switch has to land before any allocation. Go's testing TempDir
	// reads GOTMPDIR first, then TMPDIR, so set both.
	tmpRoot, err := os.MkdirTemp("", "crush-test-tmproot-")
	require.NoError(t, err)
	t.Setenv("TMPDIR", tmpRoot)
	t.Setenv("GOTMPDIR", tmpRoot)
	t.Cleanup(func() { os.RemoveAll(tmpRoot) })

	first := testEnv(t)
	marker := filepath.Join(first.workingDir, "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("one"), 0o644))

	// A second process would allocate under its own temporary root for
	// the same test name; within one process a second allocation stands
	// in for it, and must not reach into the first workspace.
	second := testEnv(t)
	require.NotEqual(t, first.workingDir, second.workingDir)
	require.NotEqual(t, first.workingDir, first.recordedDir)
	require.NoError(t, os.WriteFile(filepath.Join(second.workingDir, "marker.txt"), []byte("two"), 0o644))

	data, err := os.ReadFile(marker)
	require.NoError(t, err, "the second allocation deleted the first workspace's marker")
	require.Equal(t, "one", string(data))

	require.True(t, strings.HasPrefix(first.workingDir, tmpRoot),
		"workspace ignores the configured temporary root: %s", first.workingDir)
}
