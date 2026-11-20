package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const acpLLMDocIndexURL = "https://agentclientprotocol.com/llms.txt"

func fetchACPDocs() error {
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Get(acpLLMDocIndexURL)
	if err != nil {
		return fmt.Errorf("fetching LLM index: %w", err)
	}
	defer resp.Body.Close()

	// URL_RE matches https:// URLs not containing whitespace or closing paren.
	// Used to extract URLs from markdown-style lines such as:
	//   - [Title](https://example.com/page.md): description
	// where the closing ')' of the markdown link delimits the URL.
	const urlRE = `https?://[^\s)]+`
	re := regexp.MustCompile(urlRE)

	var acpDocs []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		acpDocs = append(acpDocs, re.FindAllString(scanner.Text(), -1)...)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading LLM index: %w", err)
	}

	fmt.Printf("Found %d documents\n", len(acpDocs))

	// docs/acp relative to working directory (run from project root).
	destFolder := filepath.Join("docs", "acp")
	_ = os.MkdirAll(destFolder, 0o755)
	absDest, _ := filepath.Abs(destFolder)
	fmt.Printf("Storing files in %s\n", absDest)

	maxURLLen := 0
	for _, doc := range acpDocs {
		if len(doc) > maxURLLen {
			maxURLLen = len(doc)
		}
	}

	downloaded := 0
	skipped := 0
	for i, acpDoc := range acpDocs {
		u, err := url.Parse(acpDoc)
		if err != nil || u.Scheme == "" || u.Host == "" {
			fmt.Printf("\n[%d/%d] SKIP invalid URL: %q\n", i+1, len(acpDocs), acpDoc)
			skipped++
			continue
		}

		localPath := filepath.Clean(filepath.Join(destFolder, u.Path))

		progress := fmt.Sprintf("[%d/%d] Downloading: %s", i+1, len(acpDocs), acpDoc)
		if maxURLLen > len(acpDoc) {
			progress += strings.Repeat(" ", maxURLLen-len(acpDoc))
		}
		fmt.Print(progress + "\r")

		if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
			fmt.Printf("\n[%d/%d] SKIP mkdir %s: %v\n", i+1, len(acpDocs), filepath.Dir(localPath), err)
			skipped++
			continue
		}

		docResp, err := client.Get(acpDoc)
		if err != nil {
			fmt.Printf("\n[%d/%d] SKIP download %s: %v\n", i+1, len(acpDocs), acpDoc, err)
			skipped++
			continue
		}

		f, err := os.OpenFile(localPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			docResp.Body.Close()
			fmt.Printf("\n[%d/%d] SKIP open %s: %v\n", i+1, len(acpDocs), localPath, err)
			skipped++
			continue
		}

		_, err = io.Copy(f, docResp.Body)
		docResp.Body.Close()
		f.Close()
		if err != nil {
			fmt.Printf("\n[%d/%d] SKIP write %s: %v\n", i+1, len(acpDocs), localPath, err)
			skipped++
			continue
		}
		downloaded++
	}

	fmt.Printf("\nDone: %d downloaded, %d skipped, %d total\n", downloaded, skipped, len(acpDocs))
	return nil
}

func main() {
	if err := fetchACPDocs(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
