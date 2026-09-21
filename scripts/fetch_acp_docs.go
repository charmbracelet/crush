package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const acpLLMDocIndexURL = "https://agentclientprotocol.com/llms.txt"

func fetchACPDocs() error {
	resp, err := http.Get(acpLLMDocIndexURL)
	if err != nil {
		return fmt.Errorf("fetching LLM index: %w", err)
	}
	defer resp.Body.Close()

	var acpDocs []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "](") {
			continue
		}
		start := strings.Index(line, "](") + 2
		end := strings.Index(line[start:], ")")
		if end < 0 {
			continue
		}
		acpDocs = append(acpDocs, line[start:end])
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading LLM index: %w", err)
	}

	fmt.Printf("Found %d documents\n", len(acpDocs))

	_, execPath, _, _ := runtime.Caller(0)
	destFolder := filepath.Join(filepath.Dir(execPath), "..", "..", "docs", "acp")
	destFolder, _ = filepath.Abs(destFolder)
	fmt.Printf("Storing files in %s\n", destFolder)

	maxURLLen := 0
	for _, doc := range acpDocs {
		if len(doc) > maxURLLen {
			maxURLLen = len(doc)
		}
	}

	for i, acpDoc := range acpDocs {
		u, err := url.Parse(acpDoc)
		if err != nil {
			return fmt.Errorf("parsing URL %q: %w", acpDoc, err)
		}
		localPath := filepath.Clean(filepath.Join(destFolder, u.Path))

		progress := fmt.Sprintf("[%d/%d] Downloading: %s", i+1, len(acpDocs), acpDoc)
		if maxURLLen > len(acpDoc) {
			progress += strings.Repeat(" ", maxURLLen-len(acpDoc))
		}
		fmt.Print(progress + "\r")

		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", filepath.Dir(localPath), err)
		}

		docResp, err := http.Get(acpDoc)
		if err != nil {
			return fmt.Errorf("downloading %s: %w", acpDoc, err)
		}

		f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			docResp.Body.Close()
			return fmt.Errorf("opening %s: %w", localPath, err)
		}

		_, err = io.Copy(f, docResp.Body)
		docResp.Body.Close()
		f.Close()
		if err != nil {
			return fmt.Errorf("writing %s: %w", localPath, err)
		}
	}

	fmt.Println("All files downloaded!")
	return nil
}

func main() {
	if err := fetchACPDocs(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
