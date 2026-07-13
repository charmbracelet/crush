package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"charm.land/fantasy"
	"github.com/charmbracelet/crush/internal/agent/tools"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

func TestActivatedSourcePartsAddsResolvedPDFOnce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "reference.pdf")
	data := []byte("%PDF-1.7\nsource")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	source := session.Source{ID: "source-1", Kind: session.SourceKindFile, Label: "Reference", Location: path}
	metadata, err := json.Marshal(source)
	require.NoError(t, err)
	result := fantasy.ToolResultContent{
		ToolCallID:     "resolve-1",
		ToolName:       tools.SourcesToolName,
		ClientMetadata: string(metadata),
		Result:         fantasy.ToolResultOutputContentText{Text: "selected"},
	}
	steps := []fantasy.StepResult{{Response: fantasy.Response{Content: fantasy.ResponseContent{result}}}, {Response: fantasy.Response{Content: fantasy.ResponseContent{result}}}}

	parts := activatedSourceParts(steps, true)
	require.Len(t, parts, 1)
	file, ok := fantasy.AsMessagePart[fantasy.FilePart](parts[0])
	require.True(t, ok)
	require.Equal(t, "application/pdf", file.MediaType)
	require.Equal(t, data, file.Data)
}

func TestActivatedSourcePartsDoesNotInjectTextFiles(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("read lazily"), 0o644))
	source := session.Source{ID: "source-1", Kind: session.SourceKindFile, Label: "Notes", Location: path}
	metadata, err := json.Marshal(source)
	require.NoError(t, err)
	steps := []fantasy.StepResult{{Response: fantasy.Response{Content: fantasy.ResponseContent{fantasy.ToolResultContent{
		ToolCallID:     "resolve-1",
		ToolName:       tools.SourcesToolName,
		ClientMetadata: string(metadata),
		Result:         fantasy.ToolResultOutputContentText{Text: "selected"},
	}}}}}

	require.Empty(t, activatedSourceParts(steps, true))
}
