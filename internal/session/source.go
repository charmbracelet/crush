package session

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/filepathext"
	"github.com/google/uuid"
)

type SourceKind string

const (
	SourceKindFile SourceKind = "file"
	SourceKindURL  SourceKind = "url"
	SourceKindText SourceKind = "text"
)

// Source is a persistent, session-scoped reference. File and URL contents
// remain lazy until the source is explicitly resolved.
type Source struct {
	ID        string     `json:"id"`
	Kind      SourceKind `json:"kind"`
	Label     string     `json:"label"`
	Location  string     `json:"location,omitempty"`
	Content   string     `json:"content,omitempty"`
	CreatedAt int64      `json:"created_at"`
}

// NewSource normalizes a file path, URL, or text value into a session source.
func NewSource(value string, kind SourceKind, label, workingDir string) (Source, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Source{}, fmt.Errorf("source value cannot be empty")
	}
	kind = SourceKind(strings.ToLower(strings.TrimSpace(string(kind))))
	if kind == "" {
		if IsHTTPSource(value) {
			kind = SourceKindURL
		} else if sourcePathExists(filepathext.SmartJoin(workingDir, value)) {
			kind = SourceKindFile
		} else {
			kind = SourceKindText
		}
	}

	source := Source{
		ID:        uuid.NewString(),
		Kind:      kind,
		Label:     strings.TrimSpace(label),
		CreatedAt: time.Now().Unix(),
	}
	switch kind {
	case SourceKindFile:
		path := filepathext.SmartJoin(workingDir, value)
		absolute, err := filepath.Abs(path)
		if err != nil {
			return Source{}, fmt.Errorf("resolve source file %q: %w", value, err)
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return Source{}, fmt.Errorf("source file %q is unavailable: %w", absolute, err)
		}
		if info.IsDir() {
			return Source{}, fmt.Errorf("source file %q is a directory", absolute)
		}
		source.Location = absolute
		if source.Label == "" {
			source.Label = filepath.Base(absolute)
		}
	case SourceKindURL:
		parsed, err := url.ParseRequestURI(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return Source{}, fmt.Errorf("source URL must be a valid http(s) URL: %q", value)
		}
		source.Location = parsed.String()
		if source.Label == "" {
			source.Label = parsed.Hostname()
		}
	case SourceKindText:
		source.Content = value
		if source.Label == "" {
			source.Label = "Text source"
		}
	default:
		return Source{}, fmt.Errorf("source kind must be file, url, or text")
	}
	return source, nil
}

// IsHTTPSource reports whether value is an absolute HTTP or HTTPS URL.
func IsHTTPSource(value string) bool {
	parsed, err := url.ParseRequestURI(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func sourcePathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
