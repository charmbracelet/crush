package agent

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/session"
)

const maxActivatedSourceSize = 25 * 1024 * 1024

// activatedSourceParts converts explicitly resolved media sources into native
// model file parts. Source bodies remain absent from ordinary turns.
func activatedSourceParts(steps []fantasy.StepResult, supportsImages bool) []fantasy.MessagePart {
	seen := make(map[string]struct{})
	var parts []fantasy.MessagePart
	for _, step := range steps {
		for _, result := range step.Content.ToolResults() {
			if result.ToolName != tools.SourcesToolName || result.ClientMetadata == "" {
				continue
			}
			var source session.Source
			if err := json.Unmarshal([]byte(result.ClientMetadata), &source); err != nil || source.Kind != session.SourceKindFile {
				continue
			}
			if _, ok := seen[source.ID]; ok {
				continue
			}
			seen[source.ID] = struct{}{}

			part, warning := activatedSourceFilePart(source, supportsImages)
			if warning != "" {
				parts = append(parts, fantasy.TextPart{Text: warning})
			}
			if part != nil {
				parts = append(parts, *part)
			}
		}
	}
	return parts
}

func activatedSourceFilePart(source session.Source, supportsImages bool) (*fantasy.FilePart, string) {
	file, err := os.Open(source.Location)
	if err != nil {
		return nil, fmt.Sprintf("Attached source %q could not be opened: %v", source.Label, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Sprintf("Attached source %q could not be inspected: %v", source.Label, err)
	}
	if info.Size() > maxActivatedSourceSize {
		return nil, fmt.Sprintf("Attached source %q is too large to activate (%d bytes; maximum %d).", source.Label, info.Size(), maxActivatedSourceSize)
	}

	header := make([]byte, 512)
	read, _ := file.Read(header)
	mediaType := sourceMediaType(source.Location, header[:read])
	if mediaType != "application/pdf" && !strings.HasPrefix(mediaType, "image/") {
		return nil, ""
	}
	if strings.HasPrefix(mediaType, "image/") && !supportsImages {
		return nil, fmt.Sprintf("Attached image source %q was not activated because the selected model does not advertise image support.", source.Label)
	}

	data, err := os.ReadFile(source.Location)
	if err != nil {
		return nil, fmt.Sprintf("Attached source %q could not be read: %v", source.Label, err)
	}
	return &fantasy.FilePart{
		Filename:  filepath.Base(source.Location),
		Data:      data,
		MediaType: mediaType,
	}, ""
}

func sourceMediaType(path string, header []byte) string {
	extensionType := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	if separator := strings.IndexByte(extensionType, ';'); separator >= 0 {
		extensionType = extensionType[:separator]
	}
	if extensionType == "application/pdf" || strings.HasPrefix(extensionType, "image/") {
		return extensionType
	}
	if len(header) == 0 {
		return extensionType
	}
	return http.DetectContentType(header)
}
