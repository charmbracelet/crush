package autolsp

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
)

// LangDetector detects programming languages in a workspace dir.
type LangDetector struct {
	fs fs.FS
}

// LangDetectorOption configures a LangDetector.
type LangDetectorOption func(d *LangDetector)

// NewLangDetector creates a new language detector with the given options.
func NewLangDetector(options ...LangDetectorOption) *LangDetector {
	d := LangDetector{}
	for _, opt := range options {
		opt(&d)
	}
	if d.fs == nil {
		d.fs = os.DirFS(".")
	}
	return &d
}

// LangDetectorWithFS configures the detector to use a specific filesystem.
func LangDetectorWithFS(fs fs.FS) LangDetectorOption {
	return func(d *LangDetector) {
		d.fs = fs
	}
}

// LangDetectorWithDir configures the detector to use a specific directory.
func LangDetectorWithDir(dir string) LangDetectorOption {
	return func(d *LangDetector) {
		d.fs = os.DirFS(dir)
	}
}

// Detect scans the filesystem and returns detected languages sorted by priority.
func (d *LangDetector) Detect() []LangName {
	priorities := make(map[LangName]int)

	_ = fs.WalkDir(d.fs, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if _, ok := dirsToIgnore[e.Name()]; ok {
			return filepath.SkipDir
		}
		for _, lang := range Langs {
			for _, pattern := range lang.WorkspacePatterns {
				if match, _ := filepath.Match(pattern, e.Name()); match {
					priorities[lang.Name] += 10
					break
				}
			}
			for _, pattern := range lang.FilePatterns {
				if match, _ := filepath.Match(pattern, e.Name()); match {
					priorities[lang.Name] += 1
					break
				}
			}
		}
		return nil
	})

	var langNames []LangName
	for _, lang := range Langs {
		if priorities[lang.Name] > 0 {
			langNames = append(langNames, lang.Name)
		}
	}
	slices.SortFunc(langNames, func(a, b LangName) int {
		return priorities[b] - priorities[a]
	})
	return langNames
}
