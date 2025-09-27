package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/home"
)

// Prompt represents a template-based prompt generator.
type Prompt struct {
	name     string
	template string
}

type PromptDat struct {
	Provider   string
	Model      string
	Config     config.Config
	WorkingDir string
	IsGitRepo  bool
	Platform   string
	Date       string
}

type ContextFile struct {
	Path    string
	Content string
}

func NewPrompt(name, promptTemplate string) (*Prompt, error) {
	return &Prompt{
		name:     name,
		template: promptTemplate,
	}, nil
}

func (p *Prompt) Build(provider, model string, cfg config.Config) (string, error) {
	t, err := template.New(p.name).Funcs(p.funcMap(cfg)).Parse(p.template)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	var sb strings.Builder
	if err := t.Execute(&sb, promptData(provider, model, cfg)); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return sb.String(), nil
}

func (p *Prompt) funcMap(cfg config.Config) template.FuncMap {
	return template.FuncMap{
		"contextFiles": func(path string) []ContextFile {
			path = expandPath(path, cfg)
			return processContextPath(path, cfg)
		},
	}
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

func processContextPath(p string, cfg config.Config) []ContextFile {
	var contexts []ContextFile
	fullPath := p
	if !filepath.IsAbs(p) {
		fullPath = filepath.Join(cfg.WorkingDir(), p)
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
func expandPath(path string, cfg config.Config) string {
	path = home.Long(path)
	// Handle environment variable expansion using the same pattern as config
	if strings.HasPrefix(path, "$") {
		if expanded, err := cfg.Resolver().ResolveValue(path); err == nil {
			path = expanded
		}
	}

	return path
}

func promptData(provider, model string, cfg config.Config) PromptDat {
	return PromptDat{
		Provider:   provider,
		Model:      model,
		Config:     cfg,
		WorkingDir: cfg.WorkingDir(),
		IsGitRepo:  isGitRepo(cfg.WorkingDir()),
		Platform:   runtime.GOOS,
		Date:       time.Now().Format("1/2/2006"),
	}
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func (p *Prompt) Name() string {
	return p.name
}
