package prompt

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
	"github.com/charmbracelet/crush/internal/shell"
	"github.com/charmbracelet/crush/internal/skills"
)

// Prompt represents a template-based prompt generator.
type Prompt struct {
	name       string
	template   string
	now        func() time.Time
	platform   string
	workingDir string
}

type PromptDat struct {
	Provider      string
	Model         string
	Config        config.Config
	WorkingDir    string
	IsGitRepo     bool
	Platform      string
	Date          string
	GitStatus     string
	ContextFiles  []ContextFile
	MemoryFiles   []ContextFile
	AvailSkillXML string
}

type ContextFile struct {
	Path    string
	Content string
}

type Option func(*Prompt)

func WithTimeFunc(fn func() time.Time) Option {
	return func(p *Prompt) {
		p.now = fn
	}
}

func WithPlatform(platform string) Option {
	return func(p *Prompt) {
		p.platform = platform
	}
}

func WithWorkingDir(workingDir string) Option {
	return func(p *Prompt) {
		p.workingDir = workingDir
	}
}

func NewPrompt(name, promptTemplate string, opts ...Option) (*Prompt, error) {
	p := &Prompt{
		name:     name,
		template: promptTemplate,
		now:      time.Now,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p, nil
}

func (p *Prompt) Build(ctx context.Context, provider, model string, store *config.ConfigStore) (string, error) {
	t, err := template.New(p.name).Parse(p.template)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	var sb strings.Builder
	d, err := p.promptData(ctx, provider, model, store)
	if err != nil {
		return "", err
	}
	if err := t.Execute(&sb, d); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
}

func processFile(filePath string) *ContextFile {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	return &ContextFile{
		Path:    filePath,
		Content: string(content),
	}
}

func processContextPath(p string, store *config.ConfigStore) []ContextFile {
	var contexts []ContextFile
	fullPath := p
	if !filepath.IsAbs(p) {
		fullPath = filepath.Join(store.WorkingDir(), p)
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return contexts
	}
	if info.IsDir() {
		filepath.WalkDir(fullPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				if result := processFile(path); result != nil {
					contexts = append(contexts, *result)
				}
			}
			return nil
		})
	} else {
		result := processFile(fullPath)
		if result != nil {
			contexts = append(contexts, *result)
		}
	}
	return contexts
}

// expandPath expands ~ and environment variables in file paths
func expandPath(path string, store *config.ConfigStore) string {
	path = home.Long(path)
	// Handle environment variable expansion using the same pattern as config
	if strings.HasPrefix(path, "$") {
		if expanded, err := store.Resolver().ResolveValue(path); err == nil {
			path = expanded
		}
	}

	return path
}

func (p *Prompt) promptData(ctx context.Context, provider, model string, store *config.ConfigStore) (PromptDat, error) {
	workingDir := cmp.Or(p.workingDir, store.WorkingDir())
	platform := cmp.Or(p.platform, runtime.GOOS)

	contextFiles := map[string][]ContextFile{}
	memoryFiles := map[string][]ContextFile{}

	cfg := store.Config()
	for _, pth := range cfg.Options.ContextPaths {
		expanded := expandPath(pth, store)
		pathKey := strings.ToLower(expanded)
		if _, ok := contextFiles[pathKey]; ok {
			continue
		}
		content := processContextPath(expanded, store)
		contextFiles[pathKey] = content
	}

	for _, pth := range cfg.Options.MemoryPaths {
		expanded := expandPath(pth, store)
		pathKey := strings.ToLower(expanded)
		if _, ok := memoryFiles[pathKey]; ok {
			continue
		}
		content := processContextPath(expanded, store)
		memoryFiles[pathKey] = content
	}

	// Discover and load skills metadata.
	var availSkillXML string

	// Start with builtin skills.
	allSkills := skills.DiscoverBuiltin()
	builtinNames := make(map[string]bool, len(allSkills))
	for _, s := range allSkills {
		builtinNames[s.Name] = true
	}

	// Discover user skills from configured paths.
	if len(cfg.Options.SkillsPaths) > 0 {
		expandedPaths := make([]string, 0, len(cfg.Options.SkillsPaths))
		for _, pth := range cfg.Options.SkillsPaths {
			expandedPaths = append(expandedPaths, expandPath(pth, store))
		}
		for _, userSkill := range skills.Discover(expandedPaths) {
			if builtinNames[userSkill.Name] {
				slog.Warn("User skill overrides builtin skill", "name", userSkill.Name)
			}
			allSkills = append(allSkills, userSkill)
		}
	}

	// Deduplicate: user skills override builtins with the same name.
	allSkills = skills.Deduplicate(allSkills)

	// Filter out disabled skills.
	allSkills = skills.Filter(allSkills, cfg.Options.DisabledSkills)

	if len(allSkills) > 0 {
		availSkillXML = skills.ToPromptXML(allSkills)
	}

	isGit := isGitRepo(store.WorkingDir())
	data := PromptDat{
		Provider:      provider,
		Model:         model,
		Config:        *cfg,
		WorkingDir:    filepath.ToSlash(workingDir),
		IsGitRepo:     isGit,
		Platform:      platform,
		Date:          p.now().Format("1/2/2006"),
		AvailSkillXML: availSkillXML,
	}
	if isGit {
		var err error
		data.GitStatus, err = getGitStatus(ctx, store.WorkingDir())
		if err != nil {
			return PromptDat{}, err
		}
	}

	for _, files := range contextFiles {
		data.ContextFiles = append(data.ContextFiles, files...)
	}
	for _, files := range memoryFiles {
		data.MemoryFiles = append(data.MemoryFiles, files...)
	}
	return data, nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func getGitStatus(ctx context.Context, dir string) (string, error) {
	sh := shell.NewShell(&shell.Options{
		WorkingDir: dir,
	})
	branch, err := getGitBranch(ctx, sh)
	if err != nil {
		return "", err
	}
	status, err := getGitStatusSummary(ctx, sh)
	if err != nil {
		return "", err
	}
	commits, err := getGitRecentCommits(ctx, sh)
	if err != nil {
		return "", err
	}
	return branch + status + commits, nil
}

func getGitBranch(ctx context.Context, sh *shell.Shell) (string, error) {
	out, _, err := sh.Exec(ctx, "git branch --show-current 2>/dev/null")
	if err != nil {
		return "", nil
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", nil
	}
	return fmt.Sprintf("Current branch: %s\n", out), nil
}

func getGitStatusSummary(ctx context.Context, sh *shell.Shell) (string, error) {
	out, _, err := sh.Exec(ctx, "git status --short 2>/dev/null | head -20")
	if err != nil {
		return "", nil
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "Status: clean\n", nil
	}
	return fmt.Sprintf("Status:\n%s\n", out), nil
}

func getGitRecentCommits(ctx context.Context, sh *shell.Shell) (string, error) {
	out, _, err := sh.Exec(ctx, "git log --oneline -n 3 2>/dev/null")
	if err != nil || out == "" {
		return "", nil
	}
	out = strings.TrimSpace(out)
	return fmt.Sprintf("Recent commits:\n%s\n", out), nil
}

func (p *Prompt) Name() string {
	return p.name
}
