package agent

import (
	"context"
	"os"
	"path/filepath"
	_ "embed"

	"github.com/charmbracelet/crush/internal/agent/prompt"
	"github.com/charmbracelet/crush/internal/config"
)

//go:embed templates/coder.md.tpl
var coderPromptTmpl []byte

//go:embed templates/task.md.tpl
var taskPromptTmpl []byte

//go:embed templates/initialize.md.tpl
var initializePromptTmpl []byte

// TemplateLoader loads templates from a filesystem path with fallback to embedded templates.
type TemplateLoader struct {
	config *config.ConfigStore
}

func NewTemplateLoader(cfg *config.ConfigStore) *TemplateLoader {
	return &TemplateLoader{config: cfg}
}

// LoadTemplateString loads a template from the filesystem if TemplatePath is configured,
// otherwise falls back to the provided embedded template.
func (tl *TemplateLoader) LoadTemplateString(name string, embedded []byte) (string, error) {
	if tl.config == nil || tl.config.Config().Options == nil {
		return string(embedded), nil
	}

	templatePath := tl.config.Config().Options.TemplatePath
	if templatePath == "" {
		return string(embedded), nil
	}

	fullPath := filepath.Join(templatePath, name)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		// Fall back to embedded template
		return string(embedded), nil
	}

	return string(data), nil
}

func coderPrompt(cfg *config.ConfigStore, opts ...prompt.Option) (*prompt.Prompt, error) {
	tmpl := string(coderPromptTmpl)
	if cfg != nil {
		loader := NewTemplateLoader(cfg)
		if s, err := loader.LoadTemplateString("coder.md.tpl", coderPromptTmpl); err == nil {
			tmpl = s
		}
	}
	systemPrompt, err := prompt.NewPrompt("coder", tmpl, opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func taskPrompt(cfg *config.ConfigStore, opts ...prompt.Option) (*prompt.Prompt, error) {
	tmpl := string(taskPromptTmpl)
	if cfg != nil {
		loader := NewTemplateLoader(cfg)
		if s, err := loader.LoadTemplateString("task.md.tpl", taskPromptTmpl); err == nil {
			tmpl = s
		}
	}
	systemPrompt, err := prompt.NewPrompt("task", tmpl, opts...)
	if err != nil {
		return nil, err
	}
	return systemPrompt, nil
}

func InitializePrompt(cfg *config.ConfigStore) (string, error) {
	tmpl := string(initializePromptTmpl)
	if cfg != nil {
		loader := NewTemplateLoader(cfg)
		if s, err := loader.LoadTemplateString("initialize.md.tpl", initializePromptTmpl); err == nil {
			tmpl = s
		}
	}
	systemPrompt, err := prompt.NewPrompt("initialize", tmpl)
	if err != nil {
		return "", err
	}
	return systemPrompt.Build(context.Background(), "", "", cfg)
}