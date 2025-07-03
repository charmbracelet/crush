package autolsp_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/caarlos0/testfs"
	"github.com/charmbracelet/crush/internal/lsp/autolsp"
)

func TestLangDetector(t *testing.T) {
	tests := []struct {
		name     string
		expected []autolsp.LangName
		file     string
	}{
		{"Dart", []autolsp.LangName{"dart", "yaml"}, "pubspec.yaml"},
		{"Elixir", []autolsp.LangName{"elixir"}, "mix.exs"},
		{"Go", []autolsp.LangName{"go"}, "go.mod"},
		{"Java-Gradle", []autolsp.LangName{"java"}, "build.gradle"},
		{"Java-POM", []autolsp.LangName{"java"}, "pom.xml"},
		{"JavaScript", []autolsp.LangName{"javascript", "json"}, "package.json"},
		{"PHP", []autolsp.LangName{"php", "json"}, "composer.json"},
		{"Python-Pyproject", []autolsp.LangName{"python"}, "pyproject.toml"},
		{"Python-Requirements", []autolsp.LangName{"python"}, "requirements.txt"},
		{"Python-SetupPy", []autolsp.LangName{"python"}, "setup.py"},
		{"Ruby", []autolsp.LangName{"ruby"}, "Gemfile"},
		{"Rust", []autolsp.LangName{"rust"}, "Cargo.toml"},
		{"YAML-yaml", []autolsp.LangName{"yaml"}, "config.yaml"},
		{"YAML-yml", []autolsp.LangName{"yaml"}, "config.yml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfs := testfs.New(t)
			_ = tmpfs.MkdirAll(filepath.Dir(tt.file), 0o755)
			_ = tmpfs.WriteFile(tt.file, []byte(""), 0o644)

			d := autolsp.NewLangDetector(
				autolsp.LangDetectorWithFS(tmpfs),
			)
			langs := d.Detect()

			if !reflect.DeepEqual(tt.expected, langs) {
				t.Errorf("expected languages %s, got %s", tt.expected, langs)
			}
		})
	}
}

func TestLangDetectorMultiple(t *testing.T) {
	tmpfs := testfs.New(t)

	{
		_ = tmpfs.MkdirAll("backend", 0o755)
		_ = tmpfs.WriteFile("backend/Gemfile", []byte(""), 0o644)
	}
	{
		_ = tmpfs.MkdirAll("frontend", 0o755)
		_ = tmpfs.WriteFile("frontend/package.json", []byte(""), 0o644)
	}

	d := autolsp.NewLangDetector(
		autolsp.LangDetectorWithFS(tmpfs),
	)
	expected := []autolsp.LangName{"javascript", "ruby", "json"}
	langs := d.Detect()

	if !reflect.DeepEqual(expected, langs) {
		t.Errorf("expected languages %s, got %s", expected, langs)
	}
}

func TestLangDetectorIgnoredDir(t *testing.T) {
	tmpfs := testfs.New(t)

	{
		_ = tmpfs.MkdirAll("node_modules", 0o755)
		_ = tmpfs.WriteFile("node_modules/package.json", []byte(""), 0o644)
	}

	d := autolsp.NewLangDetector(
		autolsp.LangDetectorWithFS(tmpfs),
	)
	langs := d.Detect()

	if len(langs) != 0 {
		t.Errorf("expected languages to be of length %d, got %d", 0, len(langs))
	}
}

func TestLangDetectorThisProject(t *testing.T) {
	d := autolsp.NewLangDetector(
		autolsp.LangDetectorWithFS(os.DirFS("../../..")),
	)
	expected := []autolsp.LangName{
		"go",
		"yaml",
		"json",
		"bash",
	}
	langs := d.Detect()

	if !reflect.DeepEqual(expected, langs) {
		t.Errorf("expected languages %s, got %s", expected, langs)
	}
}
