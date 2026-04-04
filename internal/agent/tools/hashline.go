package tools

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/cespare/xxhash/v2"
)

const (
	HashlineEditToolName   = "hashline_edit"
	hashlineNibbleAlphabet = "ZPMQVRWSNKTXJBYH"

	hashlineEditOpReplaceLine  = "replace_line"
	hashlineEditOpReplaceRange = "replace_range"
	hashlineEditOpPrepend      = "prepend"
	hashlineEditOpAppend       = "append"
)

type HashlineEditOperation struct {
	Operation string `json:"operation" description:"The operation to apply: replace_line, replace_range, prepend, or append"`
	Line      string `json:"line,omitempty" description:"Target line reference in LINE#HASH format for replace_line, prepend, and append operations"`
	Start     string `json:"start,omitempty" description:"Start line reference in LINE#HASH format for replace_range operations"`
	End       string `json:"end,omitempty" description:"End line reference in LINE#HASH format for replace_range operations"`
	Content   string `json:"content" description:"Text content to write or insert. For replace_range, this can include multiple lines"`
}

type HashlineEditParams struct {
	FilePath   string                  `json:"file_path" description:"The absolute path to the file to modify"`
	Operations []HashlineEditOperation `json:"operations" description:"Operations to apply sequentially in a single atomic write"`
}

type HashlineEditPermissionsParams struct {
	FilePath   string `json:"file_path"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

type HashlineEditResponseMetadata struct {
	Additions  int    `json:"additions"`
	Removals   int    `json:"removals"`
	OldContent string `json:"old_content,omitempty"`
	NewContent string `json:"new_content,omitempty"`
}

var hashlineReferencePattern = regexp.MustCompile(`^\s*[>+\-]*\s*(\d+)\s*#\s*([ZPMQVRWSNKTXJBYH]{2})`)

type hashlineRef struct {
	Line int
	Hash string
}

func computeHashlineID(lineNumber int, line string) string {
	normalized := strings.TrimRightFunc(strings.ReplaceAll(line, "\r", ""), unicode.IsSpace)
	input := normalized
	if !containsSignificantRune(normalized) {
		input = fmt.Sprintf("%d:%s", lineNumber, normalized)
	}

	hashValue := xxhash.Sum64String(input)
	byteValue := byte(hashValue & 0xff)
	return string([]byte{
		hashlineNibbleAlphabet[byteValue>>4],
		hashlineNibbleAlphabet[byteValue&0x0f],
	})
}

func formatHashlineReference(lineNumber int, line string) string {
	return fmt.Sprintf("%d#%s", lineNumber, computeHashlineID(lineNumber, line))
}

func parseHashlineReference(reference string) (hashlineRef, error) {
	match := hashlineReferencePattern.FindStringSubmatch(reference)
	if len(match) != 3 {
		return hashlineRef{}, fmt.Errorf("invalid line reference %q. Expected format \"LINE#ID\" (for example \"5#aa\")", reference)
	}

	lineNumber, err := strconv.Atoi(match[1])
	if err != nil || lineNumber < 1 {
		return hashlineRef{}, fmt.Errorf("invalid line number in reference %q", reference)
	}

	return hashlineRef{
		Line: lineNumber,
		Hash: match[2],
	}, nil
}

func validateHashlineReference(reference hashlineRef, lines []string) (string, error) {
	if reference.Line < 1 || reference.Line > len(lines) {
		return "", fmt.Errorf("line %d does not exist (file has %d lines)", reference.Line, len(lines))
	}

	currentHash := computeHashlineID(reference.Line, lines[reference.Line-1])
	if currentHash != reference.Hash {
		return currentHash, fmt.Errorf("hash mismatch for %d#%s", reference.Line, reference.Hash)
	}

	return currentHash, nil
}

func containsSignificantRune(value string) bool {
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
