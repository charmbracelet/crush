package copilot

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"strings"
)

// reasoningFieldMapping maps non-standard reasoning field names returned by
// Copilot API proxies to the standard "reasoning_content" field that the
// openai-compat fantasy provider expects.
//
// The Copilot API proxy returns "reasoning_text" for the thinking text
// instead of the standard OpenAI "reasoning_content" field.
var reasoningFieldMapping = map[string]string{
	`"reasoning_text":`: `"reasoning_content":`,
}

// wrapReasoningTransform wraps an HTTP response to normalize non-standard
// reasoning field names in SSE streams to the standard "reasoning_content"
// that the openai-compat provider expects.
func wrapReasoningTransform(resp *http.Response) *http.Response {
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") {
		return resp
	}
	resp.Body = newReasoningTransformReader(resp.Body)
	return resp
}

// reasoningTransformReader is an io.ReadCloser that transforms reasoning field
// names in SSE stream data as it is read.
type reasoningTransformReader struct {
	inner   io.ReadCloser
	scanner *bufio.Scanner
	buf     bytes.Buffer
}

func newReasoningTransformReader(r io.ReadCloser) *reasoningTransformReader {
	return &reasoningTransformReader{inner: r, scanner: bufio.NewScanner(r)}
}

func (r *reasoningTransformReader) Read(p []byte) (int, error) {
	for r.buf.Len() == 0 {
		if !r.scanner.Scan() {
			if err := r.scanner.Err(); err != nil {
				return 0, err
			}
			return 0, io.EOF
		}
		line := r.scanner.Text()
		transformed := transformReasoningLine(line)
		r.buf.WriteString(transformed)
		r.buf.WriteByte('\n')
	}
	return r.buf.Read(p)
}

func (r *reasoningTransformReader) Close() error {
	return r.inner.Close()
}

// transformReasoningLine replaces non-standard reasoning field names with the
// standard "reasoning_content" field in an SSE data line.
func transformReasoningLine(line string) string {
	if !strings.HasPrefix(line, "data: ") {
		return line
	}
	for from, to := range reasoningFieldMapping {
		if strings.Contains(line, from) {
			line = strings.ReplaceAll(line, from, to)
		}
	}
	return line
}
