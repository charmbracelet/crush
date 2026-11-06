package agent

import (
	"context"
	"io"
	"net/http"
	"os"
	"path"
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
	sessions    session.Service
	messages    message.Service
	permissions permission.Service
	history     history.Service
	filetracker *filetracker.Service
	lspClients  *csync.Map[string, *lsp.Client]
}

type builderFunc func(t *testing.T, r cassetteTransporter) (fantasy.LanguageModel, error)

type modelPair struct {
	name       string
	largeModel builderFunc
	smallModel builderFunc
}

// cassetteTransporter is what provider builders and tool clients consume: an
// HTTP round tripper that can also hand out a default client. The vcr recorder
// satisfies it; so does the workspace-mapping wrapper below.
type cassetteTransporter interface {
	http.RoundTripper
	GetDefaultClient() *http.Client
}

// canonicalWorkspacePath is the workspace path the recorded cassettes carry:
// the fixed /tmp/crush-test/<name> root that testEnv used before it switched
// to private per-allocation directories (#4242).
func canonicalWorkspacePath(t *testing.T) string {
	return path.Join("/tmp/crush-test", t.Name())
}

// workspaceCassetteTransport rewrites the private per-allocation workspace
// path to the canonical cassette path in every outgoing request body. testEnv
// hands out private directories now, while the recorded cassettes (system
// prompt, tool outputs) carry the canonical path — without the rewrite, every
// cassette replay would mismatch on the system prompt alone.
type workspaceCassetteTransport struct {
	inner cassetteTransporter
	from  string
	to    string
}

func (t workspaceCassetteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		_ = req.Body.Close()
		body := strings.ReplaceAll(string(b), t.from, t.to)
		req.Body = io.NopCloser(strings.NewReader(body))
		req.ContentLength = int64(len(body))
	}
	return t.inner.RoundTrip(req)
}

func (t workspaceCassetteTransport) GetDefaultClient() *http.Client {
	return &http.Client{Transport: t}
}

// workspacePathTool rewrites cassette workspace paths in tool inputs before
// execution. Replay carries tool_use inputs whose absolute paths point at the
// canonical /tmp/crush-test/<name> workspace recorded on the authoring
// machine; on a private-directory run that path does not exist, so the tool
// would fail and the follow-up request would diverge from the cassette. The
// canonical prefix maps to the private workspace before Run, and the transport
// wrapper maps the private path back on the way out.
type workspacePathTool struct {
	inner     fantasy.AgentTool
	canonical string
	private   string
}

func (t workspacePathTool) Info() fantasy.ToolInfo { return t.inner.Info() }

func (t workspacePathTool) ProviderOptions() fantasy.ProviderOptions { return t.inner.ProviderOptions() }

func (t workspacePathTool) SetProviderOptions(opts fantasy.ProviderOptions) { t.inner.SetProviderOptions(opts) }

func (t workspacePathTool) Run(ctx context.Context, params fantasy.ToolCall) (fantasy.ToolResponse, error) {
	params.Input = strings.ReplaceAll(params.Input, t.canonical, t.private)
	res, err := t.inner.Run(ctx, params)
	res.Content = strings.ReplaceAll(res.Content, t.private, t.canonical)
	return res, err
}

// workspaceTools wraps the agent's tools with the cassette path mapping.
func workspaceTools(tools []fantasy.AgentTool, env fakeEnv, t *testing.T) []fantasy.AgentTool {
	mapped := make([]fantasy.AgentTool, 0, len(tools))
	for _, tool := range tools {
		mapped = append(mapped, workspacePathTool{
			inner:     tool,
			canonical: canonicalWorkspacePath(t),
			private:   filepath.ToSlash(env.workingDir),
		})
	}
	return mapped
}

func hyperBuilder(model string) builderFunc {
	return func(t *testing.T, r cassetteTransporter) (fantasy.LanguageModel, error) {
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
	// Private workspace per allocation (#4242): two processes running the
	// same named test used to share /tmp/crush-test/<name> and delete each
	// other's files, and the fixed root ignored TMPDIR. t.TempDir() is
	// unique per call, honors the configured temporary root, and cleans up
	// with the test. The cassettes keep the canonical path; the transport
	// wrapper maps the private path back at the request boundary.
	workingDir := t.TempDir()

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

func coderAgent(t *testing.T, r cassetteTransporter, env fakeEnv, large, small fantasy.LanguageModel) (SessionAgent, error) {
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

	return testSessionAgent(env, large, small, systemPrompt, workspaceTools(allTools, env, t)...), nil
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

// Two allocations of the same named test must not share a workspace: parallel
// `go test` processes run the same test name, and the fixed
// /tmp/crush-test/<name> path let either process delete the other's files
// (#4242). A configured temporary root must also be honored.
func TestTestEnvWorkspaceIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows for now")
	}

	// Set TMPDIR before anything calls t.TempDir: the per-test base root is
	// cached at the first call, so the switch must land first.
	tmpRoot, err := os.MkdirTemp("", "crush-test-tmprt-")
	require.NoError(t, err)
	t.Setenv("TMPDIR", tmpRoot)
	t.Cleanup(func() { os.RemoveAll(tmpRoot) })

	first := testEnv(t)
	marker := filepath.Join(first.workingDir, "marker.txt")
	require.NoError(t, os.WriteFile(marker, []byte("one"), 0o644))

	// A second process would run testEnv under its own TMPDIR for the same
	// test name. Within one process, two allocations stand in for the two
	// setups: the second must not reach into the first's workspace.
	second := testEnv(t)
	require.NotEqual(t, first.workingDir, second.workingDir)
	require.NoError(t, os.WriteFile(filepath.Join(second.workingDir, "marker.txt"), []byte("two"), 0o644))

	data, err := os.ReadFile(marker)
	require.NoError(t, err, "the second setup deleted the first workspace's marker")
	require.Equal(t, "one", string(data))

	require.True(t, strings.HasPrefix(first.workingDir, tmpRoot),
		"workspace ignores the configured temporary root: %s", first.workingDir)
}
