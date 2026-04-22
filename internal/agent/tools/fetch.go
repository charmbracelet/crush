package tools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/fantasy"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/itchyny/gojq"
)

const (
	FetchToolName = "fetch"
	MaxFetchSize  = 1 * 1024 * 1024 // 1MB
	// jqHintThreshold is the response size above which fetch will
	// append a trailing [crush-hint: ...] banner nudging the caller
	// toward the `jq` parameter when the body looks like JSON and no
	// filter was provided. Appended (not prepended) so that any
	// downstream consumer that parses the body from the start still
	// sees valid JSON up to the banner.
	jqHintThreshold = 50 * 1024 // 50 KB
)

//go:embed fetch.md
var fetchDescription []byte

func NewFetchTool(permissions permission.Service, workingDir string, client *http.Client) fantasy.AgentTool {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 100
		transport.MaxIdleConnsPerHost = 10
		transport.IdleConnTimeout = 90 * time.Second

		client = &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		}
	}

	return fantasy.NewParallelAgentTool(
		FetchToolName,
		FirstLineDescription(fetchDescription),
		func(ctx context.Context, params FetchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if params.URL == "" {
				return fantasy.NewTextErrorResponse("URL parameter is required"), nil
			}

			// When a jq expression is provided, format is ignored. We
			// skip validation entirely in that case and normalize format
			// to "text" so any later code paths inspecting it see a
			// valid value. Without jq, format must be one of the
			// supported values.
			format := strings.ToLower(params.Format)
			if params.JQ != "" {
				format = "text"
			} else if format != "text" && format != "markdown" && format != "html" {
				return fantasy.NewTextErrorResponse(
					"Format must be one of: text, markdown, html. " +
						"For JSON responses, set the `jq` parameter to filter " +
						"server-side — format then becomes optional " +
						"(e.g. fetch(url=..., jq=\"length\")).",
				), nil
			}

			if !strings.HasPrefix(params.URL, "http://") && !strings.HasPrefix(params.URL, "https://") {
				return fantasy.NewTextErrorResponse("URL must start with http:// or https://"), nil
			}

			sessionID := GetSessionFromContext(ctx)
			if sessionID == "" {
				return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for creating a new file")
			}

			p, err := permissions.Request(ctx,
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
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !p {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			// maxFetchTimeoutSeconds is the maximum allowed timeout for fetch requests (2 minutes)
			const maxFetchTimeoutSeconds = 120

			// Handle timeout with context
			requestCtx := ctx
			if params.Timeout > 0 {
				if params.Timeout > maxFetchTimeoutSeconds {
					params.Timeout = maxFetchTimeoutSeconds
				}
				var cancel context.CancelFunc
				requestCtx, cancel = context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
				defer cancel()
			}

			req, err := http.NewRequestWithContext(requestCtx, "GET", params.URL, nil)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to create request: %w", err)
			}

			req.Header.Set("User-Agent", "crush/1.0")

			resp, err := client.Do(req)
			if err != nil {
				return fantasy.ToolResponse{}, fmt.Errorf("failed to fetch URL: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Request failed with status code: %d", resp.StatusCode)), nil
			}

			body, err := io.ReadAll(io.LimitReader(resp.Body, MaxFetchSize))
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to read response body: " + err.Error()), nil
			}

			content := string(body)

			validUTF8 := utf8.ValidString(content)
			if !validUTF8 {
				return fantasy.NewTextErrorResponse("Response content is not valid UTF-8"), nil
			}
			contentType := resp.Header.Get("Content-Type")

			// If a jq expression was provided, parse the body as JSON,
			// apply the filter, and return the result directly (format is
			// ignored).
			if params.JQ != "" {
				filtered, err := applyJQ(content, params.JQ)
				if err != nil {
					return fantasy.NewTextErrorResponse("jq: " + err.Error()), nil
				}
				return fantasy.NewTextResponse(filtered), nil
			}

			largeJSONWithoutFilter := format == "text" &&
				len(body) > jqHintThreshold &&
				looksLikeJSON(contentType, body)

			switch format {
			case "text":
				if strings.Contains(contentType, "text/html") {
					text, err := extractTextFromHTML(content)
					if err != nil {
						return fantasy.NewTextErrorResponse("Failed to extract text from HTML: " + err.Error()), nil
					}
					content = text
				}

			case "markdown":
				if strings.Contains(contentType, "text/html") {
					markdown, err := convertHTMLToMarkdown(content)
					if err != nil {
						return fantasy.NewTextErrorResponse("Failed to convert HTML to Markdown: " + err.Error()), nil
					}
					content = markdown
				}

				content = "```\n" + content + "\n```"

			case "html":
				// return only the body of the HTML document
				if strings.Contains(contentType, "text/html") {
					doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
					if err != nil {
						return fantasy.NewTextErrorResponse("Failed to parse HTML: " + err.Error()), nil
					}
					body, err := doc.Find("body").Html()
					if err != nil {
						return fantasy.NewTextErrorResponse("Failed to extract body from HTML: " + err.Error()), nil
					}
					if body == "" {
						return fantasy.NewTextErrorResponse("No body content found in HTML"), nil
					}
					content = "<html>\n<body>\n" + body + "\n</body>\n</html>"
				}
			}
			// truncate content if it exceeds max read size
			if int64(len(content)) >= MaxFetchSize {
				content = content[:MaxFetchSize]
				content += fmt.Sprintf("\n\n[Content truncated to %d bytes]", MaxFetchSize)
			}

			// Append the jq hint last so it's always at the true end of
			// the response, even if the format switch above rewrote the
			// content (HTML extraction, markdown wrapping, etc.) and
			// after MaxFetchSize truncation.
			if largeJSONWithoutFilter {
				content += fmt.Sprintf(
					"\n\n[crush-hint: response body is %d bytes of JSON. "+
						"Prefer re-calling fetch() with a `jq` expression to "+
						"filter server-side (e.g. jq=\"length\", jq=\"[.[].name]\") "+
						"instead of loading the full payload into context.]",
					len(body),
				)
			}

			return fantasy.NewTextResponse(content), nil
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

// applyJQ parses body as JSON and runs the given jq expression against it,
// returning pretty-printed results joined by newlines. Multiple top-level
// JSON values in the body are supported (each is filtered independently).
//
// When the filter errors against the actual shape of the body, the error
// is annotated with a short shape description so the caller (usually an
// LLM) can fix the filter on the next attempt instead of guessing.
func applyJQ(body, expr string) (string, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return "", fmt.Errorf("compile: %w", err)
	}

	dec := json.NewDecoder(strings.NewReader(body))
	dec.UseNumber()
	var inputs []any
	for {
		var v any
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("invalid JSON: %w", err)
		}
		inputs = append(inputs, v)
	}
	if len(inputs) == 0 {
		return "", fmt.Errorf("empty response body")
	}

	var out strings.Builder
	for _, in := range inputs {
		iter := code.Run(in)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if e, ok := v.(error); ok {
				return "", fmt.Errorf("%w (input shape: %s)", e, describeShape(in))
			}
			bs, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return "", err
			}
			out.Write(bs)
			out.WriteByte('\n')
		}
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// describeShape returns a short, human-readable description of v. Used in
// jq error messages so the caller can see what the body actually looks
// like without us dumping the whole payload back.
func describeShape(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case json.Number:
		return "number"
	case string:
		return "string"
	case []any:
		if len(x) == 0 {
			return "empty array"
		}
		return fmt.Sprintf("array of %d items; first item is %s", len(x), describeShape(x[0]))
	case map[string]any:
		if len(x) == 0 {
			return "empty object"
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		const maxKeys = 8
		suffix := ""
		if len(keys) > maxKeys {
			keys = keys[:maxKeys]
			suffix = ", ..."
		}
		return fmt.Sprintf("object with keys: %s%s", strings.Join(keys, ", "), suffix)
	}
	return fmt.Sprintf("unknown (%T)", v)
}

// looksLikeJSON reports whether body is most likely JSON based on the
// Content-Type header and/or the first non-whitespace byte.
func looksLikeJSON(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "json") {
		return true
	}
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}
