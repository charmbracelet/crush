package tools

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/fantasy/ai"
)

type FetchParams struct {
	URL     string `json:"url" description:"The URL to fetch content from"`
	Format  string `json:"format" description:"The format to return the content in (text, markdown, or html)"`
	Timeout int    `json:"timeout,omitempty" description:"Optional timeout in seconds (max 120)"`
}

type FetchPermissionsParams struct {
	URL     string `json:"url"`
	Format  string `json:"format"`
	Timeout int    `json:"timeout,omitempty"`
}

type fetchTool struct {
	client      *http.Client
	permissions permission.Service
	workingDir  string
}

const FetchToolName = "fetch"

//go:embed fetch.md
var fetchDescription []byte

func NewFetchTool(permissions permission.Service, workingDir string) ai.AgentTool {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return ai.NewAgentTool(
		FetchToolName,
		string(fetchDescription),
		func(ctx context.Context, params FetchParams, call ai.ToolCall) (ai.ToolResponse, error) {
			if params.URL == "" {
				return ai.NewTextErrorResponse("URL parameter is required"), nil
			}

			format := strings.ToLower(params.Format)
			if format != "text" && format != "markdown" && format != "html" {
				return ai.NewTextErrorResponse("Format must be one of: text, markdown, html"), nil
			}

			if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
				return ai.NewTextErrorResponse("URL must start with http:// or https://"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return ai.ToolResponse{}, fmt.Errorf("session ID is required for creating a new file")
			}

			p := permissions.Request(
				permission.CreatePermissionRequest{
					SessionID:   sessionID,
					Path:        workingDir,
					ToolCallID:  call.ID,
					ToolName:    FetchToolName,
					Action:      "fetch",
					Description: fmt.Sprintf("Fetch content from URL: %s", params.URL),
					Params:      FetchPermissionsParams(params),
				},
			)

			if !p {
				return ai.ToolResponse{}, permission.ErrorPermissionDenied
			}

			// Handle timeout with context
			requestCtx := ctx
			if params.Timeout > 0 {
				maxTimeout := 120 // 2 minutes
				if params.Timeout > maxTimeout {
					params.Timeout = maxTimeout
				}
				var cancel context.CancelFunc
				requestCtx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
				defer cancel()
			}

			req, err := http.NewRequestWithContext(requestCtx, "GET", params.URL, nil)
			if err != nil {
				return ai.ToolResponse{}, fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("User-Agent", "crush/1.0")

			resp, err := client.Do(req)
			if err != nil {
				return ai.ToolResponse{}, fmt.Errorf("failed to fetch URL: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return ai.NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d", resp.StatusCode)), nil
			}

			maxSize := int64(5 * 1024 * 1024) // 5MB
			body, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
			if err != nil {
				return ai.NewTextErrorResponse("Failed to read response body: " + err.Error()), nil
			}

			content := string(body)

			isValidUt8 := utf8.ValidString(content)
			if !isValidUt8 {
				return ai.NewTextErrorResponse("Response content is not valid UTF-8"), nil
			}
			contentType := resp.Header.Get("Content-Type")

			switch format {
			case "text":
				if strings.Contains(contentType, "text/html") {
					text, err := extractTextFromHTML(content)
					if err != nil {
						return ai.NewTextErrorResponse("Failed to extract text from HTML: " + err.Error()), nil
					}
					content = text
				}

			case "markdown":
				if strings.Contains(contentType, "text/html") {
					markdown, err := convertHTMLToMarkdown(content)
					if err != nil {
						return ai.NewTextErrorResponse("Failed to convert HTML to Markdown: " + err.Error()), nil
					}
					content = markdown
				}

				content = "```\n" + content + "\n```"

			case "html":
				// return only the body of the HTML document
				if strings.Contains(contentType, "text/html") {
					doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
					if err != nil {
						return ai.NewTextErrorResponse("Failed to parse HTML: " + err.Error()), nil
					}
					body, err := doc.Find("body").Html()
					if err != nil {
						return ai.NewTextErrorResponse("Failed to extract body from HTML: " + err.Error()), nil
					}
					if body == "" {
						return ai.NewTextErrorResponse("No body content found in HTML"), nil
					}
					content = "<html>\n<body>\n" + body + "\n</body>\n</html>"
				}
			}
			// calculate byte size of content
			contentSize := int64(len(content))
			if contentSize > MaxReadSize {
				content = content[:MaxReadSize]
				content += fmt.Sprintf("\n\n[Content truncated to %d bytes]", MaxReadSize)
			}

			return ai.NewTextResponse(content), nil
		})
}

func extractTextFromHTML(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	text := doc.Find("body").Text()
	text = strings.Join(strings.Fields(text), " ")

	return text, nil
}

func convertHTMLToMarkdown(html string) (string, error) {
	converter := md.NewConverter("", true, nil)

	markdown, err := converter.ConvertString(html)
	if err != nil {
		return "", err
	}

	return markdown, nil
}
