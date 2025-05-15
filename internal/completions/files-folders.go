package completions

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/opencode-ai/opencode/internal/fileutil"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/tui/components/dialog"
)

type filesAndFoldersContextGroup struct {
	prefix string
}

func (cg *filesAndFoldersContextGroup) GetId() string {
	return cg.prefix
}

func (cg *filesAndFoldersContextGroup) GetEntry() dialog.CompletionItemI {
	return dialog.NewCompletionItem(dialog.CompletionItem{
		Title: "Files & Folders",
		Value: "files",
	})
}

func processNullTerminatedOutput(outputBytes []byte) []string {
	if len(outputBytes) > 0 && outputBytes[len(outputBytes)-1] == 0 {
		outputBytes = outputBytes[:len(outputBytes)-1]
	}

	if len(outputBytes) == 0 {
		return []string{}
	}

	split := bytes.Split(outputBytes, []byte{0})
	matches := make([]string, 0, len(split))

	for _, p := range split {
		if len(p) == 0 {
			continue
		}

		path := string(p)
		path = filepath.Join(".", path)

		if !fileutil.SkipHidden(path) {
			matches = append(matches, path)
		}
	}

	return matches
}

func (cg *filesAndFoldersContextGroup) getFiles(query string) ([]string, error) {
	cmdRg := fileutil.GetRgCmd("") // No glob pattern for this use case
	cmdFzf := fileutil.GetFzfCmd(query)

	var matches []string
	// Case 1: Both rg and fzf available
	if cmdRg != nil && cmdFzf != nil {
		rgPipe, err := cmdRg.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to get rg stdout pipe: %w", err)
		}
		defer rgPipe.Close()

		cmdFzf.Stdin = rgPipe
		var fzfOut bytes.Buffer
		var fzfErr bytes.Buffer
		cmdFzf.Stdout = &fzfOut
		cmdFzf.Stderr = &fzfErr

		if err := cmdFzf.Start(); err != nil {
			return nil, fmt.Errorf("failed to start fzf: %w", err)
		}

		errRg := cmdRg.Run()
		errFzf := cmdFzf.Wait()

		if errRg != nil {
			logging.Warn(fmt.Sprintf("rg command failed during pipe: %v", errRg))
		}

		if errFzf != nil {
			if exitErr, ok := errFzf.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return []string{}, nil // No matches from fzf
			}
			return nil, fmt.Errorf("fzf command failed: %w\nStderr: %s", errFzf, fzfErr.String())
		}

		matches = processNullTerminatedOutput(fzfOut.Bytes())

		// Case 2: Only rg available
	} else if cmdRg != nil {
		logging.Debug("Using Ripgrep with fuzzy match fallback for file completions")
		var rgOut bytes.Buffer
		var rgErr bytes.Buffer
		cmdRg.Stdout = &rgOut
		cmdRg.Stderr = &rgErr

		if err := cmdRg.Run(); err != nil {
			return nil, fmt.Errorf("rg command failed: %w\nStderr: %s", err, rgErr.String())
		}

		allFiles := processNullTerminatedOutput(rgOut.Bytes())
		matches = fuzzy.Find(query, allFiles)

		// Case 3: Only fzf available
	} else if cmdFzf != nil {
		logging.Debug("Using FZF with doublestar fallback for file completions")
		files, _, err := fileutil.GlobWithDoublestar("**/*", ".", 0)
		if err != nil {
			return nil, fmt.Errorf("failed to list files for fzf: %w", err)
		}

		allFiles := make([]string, 0, len(files))
		for _, file := range files {
			if !fileutil.SkipHidden(file) {
				allFiles = append(allFiles, file)
			}
		}

		var fzfIn bytes.Buffer
		for _, file := range allFiles {
			fzfIn.WriteString(file)
			fzfIn.WriteByte(0)
		}

		cmdFzf.Stdin = &fzfIn
		var fzfOut bytes.Buffer
		var fzfErr bytes.Buffer
		cmdFzf.Stdout = &fzfOut
		cmdFzf.Stderr = &fzfErr

		if err := cmdFzf.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
				return []string{}, nil
			}
			return nil, fmt.Errorf("fzf command failed: %w\nStderr: %s", err, fzfErr.String())
		}

		matches = processNullTerminatedOutput(fzfOut.Bytes())

		// Case 4: Fallback to doublestar with fuzzy match
	} else {
		logging.Debug("Using doublestar with fuzzy match for file completions")
		allFiles, _, err := fileutil.GlobWithDoublestar("**/*", ".", 0)
		if err != nil {
			return nil, fmt.Errorf("failed to glob files: %w", err)
		}

		filteredFiles := make([]string, 0, len(allFiles))
		for _, file := range allFiles {
			if !fileutil.SkipHidden(file) {
				filteredFiles = append(filteredFiles, file)
			}
		}

		matches = fuzzy.Find(query, filteredFiles)
	}

	return matches, nil
}

func (cg *filesAndFoldersContextGroup) GetChildEntries(query string) ([]dialog.CompletionItemI, error) {
	matches, err := cg.getFiles(query)
	if err != nil {
		return nil, err
	}

	items := make([]dialog.CompletionItemI, 0, len(matches))
	for _, file := range matches {
		item := dialog.NewCompletionItem(dialog.CompletionItem{
			Title: file,
			Value: file,
		})
		items = append(items, item)
	}

	return items, nil
}

func NewFileAndFolderContextGroup() dialog.CompletionProvider {
	return &filesAndFoldersContextGroup{
		prefix: "file",
	}
}
