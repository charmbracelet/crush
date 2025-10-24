package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/cassette"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"
)

func newRecorder(t *testing.T) *recorder.Recorder {
	cassetteName := filepath.Join("testdata", t.Name())

	r, err := recorder.New(
		cassetteName,
		recorder.WithMode(recorder.ModeRecordOnce),
		recorder.WithMatcher(customMatcher(t)),
		recorder.WithMarshalFunc(marshalFunc),
		recorder.WithSkipRequestLatency(true), // disable sleep to simulate response time, makes tests faster
		recorder.WithHook(hookRemoveHeaders, recorder.AfterCaptureHook),
	)
	if err != nil {
		t.Fatalf("recorder: failed to create recorder: %v", err)
	}

	t.Cleanup(func() {
		if err := r.Stop(); err != nil {
			t.Errorf("recorder: failed to stop recorder: %v", err)
		}
	})

	return r
}

func customMatcher(t *testing.T) recorder.MatcherFunc {
	return func(r *http.Request, i cassette.Request) bool {
		if r.Body == nil || r.Body == http.NoBody {
			return cassette.DefaultMatcher(r, i)
		}
		if r.Method != i.Method || r.URL.String() != i.URL {
			return false
		}

		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("recorder: failed to read request body")
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewBuffer(reqBody))

		// Some providers can sometimes generate JSON requests with keys in
		// a different order, which means a direct string comparison will fail.
		// Falling back to deserializing the content if we don't have a match.
		requestContent := normalizeLineEndings(reqBody)
		cassetteContent := normalizeLineEndings(i.Body)
		if requestContent == cassetteContent {
			return true
		}
		var content1, content2 any
		if err := json.Unmarshal([]byte(requestContent), &content1); err != nil {
			return false
		}
		if err := json.Unmarshal([]byte(cassetteContent), &content2); err != nil {
			return false
		}
		return reflect.DeepEqual(content1, content2)
	}
}

func marshalFunc(in any) ([]byte, error) {
	var buff bytes.Buffer
	enc := yaml.NewEncoder(&buff)
	enc.SetIndent(2)
	enc.CompactSeqIndent()
	if err := enc.Encode(in); err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}

var headersToKeep = map[string]struct{}{
	"accept":       {},
	"content-type": {},
	"user-agent":   {},
}

func hookRemoveHeaders(i *cassette.Interaction) error {
	for k := range i.Request.Headers {
		if _, ok := headersToKeep[strings.ToLower(k)]; !ok {
			delete(i.Request.Headers, k)
		}
	}
	for k := range i.Response.Headers {
		if _, ok := headersToKeep[strings.ToLower(k)]; !ok {
			delete(i.Response.Headers, k)
		}
	}
	return nil
}

// normalizeLineEndings does not only replace `\r\n` into `\n`,
// but also replaces `\\r\\n` into `\\n`. That's because we want the content
// inside JSON string to be replaces as well.
func normalizeLineEndings[T string | []byte](s T) string {
	str := string(s)
	str = strings.ReplaceAll(str, "\r\n", "\n")
	str = strings.ReplaceAll(str, `\r\n`, `\n`)
	return str
}
