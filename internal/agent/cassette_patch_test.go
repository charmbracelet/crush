package agent

// Throwaway test that rewrites the TestCoderAgent VCR cassettes to
// match the prompt changes from the prompt-optimization work: the new
// coder system prompt, the trimmed bash tool description, and the
// removed synthetic system_reminder message.
//
// Run once with:
//
//	CRUSH_PATCH_CASSETTES=1 go test ./internal/agent/ -run TestPatchCoderCassettes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v4"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
)

func TestPatchCoderCassettes(t *testing.T) {
	if os.Getenv("CRUSH_PATCH_CASSETTES") != "1" {
		t.Skip("set CRUSH_PATCH_CASSETTES=1 to rewrite cassettes")
	}

	dir := filepath.Join("testdata", "TestCoderAgent", "deepseek-v4")
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	fixedTime := func() time.Time {
		ts, _ := time.Parse("1/2/2006", "1/1/2025")
		return ts
	}

	for _, file := range files {
		base := strings.TrimSuffix(filepath.Base(file), ".yaml")
		workingDir := filepath.Join("/tmp/crush-test/TestCoderAgent/deepseek-v4", base)
		require.NoError(t, os.MkdirAll(workingDir, 0o755))

		// Render the new system prompt exactly as coderAgent does.
		p, err := coderPrompt(
			prompt.WithTimeFunc(fixedTime),
			prompt.WithPlatform("linux"),
			prompt.WithWorkingDir(filepath.ToSlash(workingDir)),
		)
		require.NoError(t, err)

		cfg, err := config.Init(workingDir, "", false)
		require.NoError(t, err)
		cfg.Config().Options.Attribution = &config.Attribution{
			TrailerStyle:  "co-authored-by",
			GeneratedWith: true,
		}
		cfg.Config().Options.SkillsPaths = nil
		cfg.Config().Options.DisabledSkills = []string{"crush-config"}
		cfg.Config().Options.ContextPaths = nil
		cfg.Config().Options.GlobalContextPaths = nil
		cfg.Config().LSP = nil
		notebookOff := false
		cfg.Config().Options.NotebookEnabled = &notebookOff

		built, err := p.Build(t.Context(), "hyper", "deepseek-v4-pro-0813", cfg)
		require.NoError(t, err)
		newPrompt := built.Text

		// Render the new bash description exactly as the test tool list does.
		perms := permission.NewPermissionService(workingDir, true, []string{})
		bashTool := tools.NewBashTool(perms, workingDir, cfg.Config().Options.Attribution, "")
		newBashDesc := bashTool.Info().Description

		// Patch every interaction's request body.
		c, err := cassette.Load(strings.TrimSuffix(file, ".yaml"))
		require.NoError(t, err)
		for _, in := range c.Interactions {
			if in.Request.Body == "" {
				continue // Non-LLM requests (e.g. file downloads).
			}
			var body map[string]any
			require.NoError(t, json.Unmarshal([]byte(in.Request.Body), &body))

			msgs, ok := body["messages"].([]any)
			if !ok {
				continue // Non-LLM requests (e.g. sourcegraph API).
			}

			// messages[0] is the system prompt.
			first, ok := msgs[0].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "system", first["role"])
			first["content"] = newPrompt

			// Drop the synthetic system_reminder user message.
			filtered := msgs[:0]
			for _, m := range msgs {
				if mm, ok := m.(map[string]any); ok {
					if content, _ := mm["content"].(string); mm["role"] == "user" && strings.HasPrefix(content, "<system_reminder>") {
						continue
					}
				}
				filtered = append(filtered, m)
			}
			body["messages"] = filtered

			// Update the bash tool description.
			if tl, ok := body["tools"].([]any); ok {
				for _, tool := range tl {
					tm, ok := tool.(map[string]any)
					if !ok {
						continue
					}
					fn, ok := tm["function"].(map[string]any)
					if !ok || fn["name"] != "bash" {
						continue
					}
					fn["description"] = newBashDesc
				}
			}

			patched, err := json.Marshal(body)
			require.NoError(t, err)
			in.Request.Body = string(patched)
			in.Request.ContentLength = int64(len(patched))
		}
		out, err := yaml.Marshal(c)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(file, out, 0o644))
		t.Logf("patched %s", file)
	}
}
