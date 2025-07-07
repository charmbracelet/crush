// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// ReplayAPIClient is a client that reads responses from a replay session file.
type replayAPIClient struct {
	ReplayFile              *replayFile
	ReplaysDirectory        string
	currentInteractionIndex int
	t                       *testing.T
	server                  *httptest.Server
}

// NewReplayAPIClient creates a new ReplayAPIClient from a replay session file.
func newReplayAPIClient(t *testing.T) *replayAPIClient {
	t.Helper()
	// The replay files are expected to be in a directory specified by the environment variable
	// GOOGLE_GENAI_REPLAYS_DIRECTORY.
	replaysDirectory := os.Getenv("GOOGLE_GENAI_REPLAYS_DIRECTORY")
	rac := &replayAPIClient{
		ReplayFile:              nil,
		ReplaysDirectory:        replaysDirectory,
		currentInteractionIndex: 0,
		t:                       t,
	}
	rac.server = httptest.NewServer(rac)
	rac.t.Cleanup(func() {
		rac.server.Close()
	})
	return rac
}

// GetBaseURL returns the URL of the mocked HTTP server.
func (rac *replayAPIClient) GetBaseURL() string {
	return rac.server.URL
}

// LoadReplay populates a replay session from a file based on the provided path.
func (rac *replayAPIClient) LoadReplay(replayFilePath string) {
	rac.t.Helper()
	fullReplaysPath := replayFilePath
	if rac.ReplaysDirectory != "" {
		fullReplaysPath = filepath.Join(rac.ReplaysDirectory, replayFilePath)
	}
	var replayFile replayFile
	if err := readFileForReplayTest(fullReplaysPath, &replayFile, true); err != nil {
		rac.t.Errorf("error loading replay file, %v", err)
	}
	rac.ReplayFile = &replayFile
}

// LatestInteraction returns the interaction that was returned by the last call to ServeHTTP.
func (rac *replayAPIClient) LatestInteraction() *replayInteraction {
	rac.t.Helper()
	if rac.currentInteractionIndex == 0 {
		rac.t.Fatalf("no interactions has been made in replay session so far")
	}
	return rac.ReplayFile.Interactions[rac.currentInteractionIndex-1]
}

// ServeHTTP mocks serving HTTP requests.
func (rac *replayAPIClient) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	rac.t.Helper()
	if rac.ReplayFile == nil {
		rac.t.Fatalf("no replay file loaded")
	}
	if rac.currentInteractionIndex >= len(rac.ReplayFile.Interactions) {
		rac.t.Fatalf("no more interactions in replay session")
	}
	interaction := rac.ReplayFile.Interactions[rac.currentInteractionIndex]

	rac.assertRequest(req, interaction.Request)
	rac.currentInteractionIndex++
	var bodySegments []string
	for i := 0; i < len(interaction.Response.BodySegments); i++ {
		responseBodySegment, err := json.Marshal(interaction.Response.BodySegments[i])
		if err != nil {
			rac.t.Errorf("error marshalling responseBodySegment [%s], err: %+v", rac.ReplayFile.ReplayID, err)
		}
		bodySegments = append(bodySegments, string(responseBodySegment))
	}
	if interaction.Response.StatusCode != 0 {
		w.WriteHeader(int(interaction.Response.StatusCode))
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_, err := w.Write([]byte(strings.Join(bodySegments, "\n")))
	if err != nil {
		rac.t.Errorf("error writing response, err: %+v", err)
	}
}

func readFileForReplayTest[T any](path string, output *T, omitempty bool) error {
	dat, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var m map[string]any
	if err := json.Unmarshal(dat, &m); err != nil {
		return fmt.Errorf("error unmarshalling to map: %w", err)
	}

	if omitempty {
		omitEmptyValues(m)
	}
	m = convertKeysToCamelCase(m, "").(map[string]any)

	// Marshal the modified map back to struct
	err = mapToStruct(m, output)
	if err != nil {
		return fmt.Errorf("error converting map to struct: %w", err)
	}

	return nil
}

// In testing server, host and scheme is empty.
func redactReplayURL(url string) string {
	url = strings.ReplaceAll(url, "{MLDEV_URL_PREFIX}/", "")
	url = strings.ReplaceAll(url, "{VERTEX_URL_PREFIX}/", "")
	return url
}

func redactSDKURL(url string) string {
	if strings.Contains(url, "project") {
		vertexRegexp := regexp.MustCompile(`.*/projects/[^/]+/locations/[^/]+/`)
		url = vertexRegexp.ReplaceAllString(url, "")
	} else {
		mldevRegexp := regexp.MustCompile(`^\/[^/]+\/`)
		url = mldevRegexp.ReplaceAllString(url, "")
	}

	return url
}

func redactProjectLocationPath(path string) string {
	// Redact a field in the request that is known to vary based on project and
	// location.
	projectLocationRegexp := regexp.MustCompile(`projects/[^/]+/locations/[^/]+`)
	return projectLocationRegexp.ReplaceAllString(path, "{PROJECT_AND_LOCATION_PATH}")
}

func redactRequestBody(body map[string]any) map[string]any {
	for key, value := range body {
		if _, ok := value.(string); ok {
			body[key] = redactProjectLocationPath(value.(string))
		}
	}
	return body
}

func (rac *replayAPIClient) assertRequest(sdkRequest *http.Request, replayRequest *replayRequest) {
	rac.t.Helper()
	sdkRequestBody, err := io.ReadAll(sdkRequest.Body)
	if err != nil {
		rac.t.Errorf("Error reading request body, err: %+v", err)
	}
	bodySegment := make(map[string]any)
	if len(sdkRequestBody) > 0 {
		if err := json.Unmarshal(sdkRequestBody, &bodySegment); err != nil {
			rac.t.Errorf("Error unmarshalling body, err: %+v", err)
		}
	}
	bodySegment = redactRequestBody(bodySegment)
	bodySegment = convertKeysToCamelCase(bodySegment, "").(map[string]any)
	omitEmptyValues(bodySegment)

	headers := make(map[string]string)
	for k, v := range sdkRequest.Header {
		headers[k] = strings.Join(v, ",")
	}
	// TODO(b/390425822): support headers validation.
	got := map[string]any{
		"method":       strings.ToLower(sdkRequest.Method),
		"url":          redactSDKURL(sdkRequest.URL.String()),
		"bodySegments": []map[string]any{bodySegment},
	}

	want := map[string]any{
		"method":       replayRequest.Method,
		"url":          redactReplayURL(replayRequest.URL),
		"bodySegments": replayRequest.BodySegments,
	}

	opts := cmp.Options{
		stringComparator,
	}

	if diff := cmp.Diff(got, want, opts); diff != "" {
		rac.t.Errorf("Requests had diffs (-got +want):\n%v", diff)
	}
}

// omitEmptyValues recursively traverses the given value and if it is a `map[string]any` or
// `[]any`, it omits the empty values.
func omitEmptyValues(v any) {
	if v == nil {
		return
	}
	switch m := v.(type) {
	case map[string]any:
		for k, v := range m {
			// If the value is empty, delete the key from the map.
			if reflect.ValueOf(v).IsZero() || v == "0001-01-01T00:00:00Z" {
				delete(m, k)
			} else {
				omitEmptyValues(v)
			}
		}
	case []any:
		for _, item := range m {
			omitEmptyValues(item)
		}
	case []map[string]any:
		for _, item := range m {
			omitEmptyValues(item)
		}
	}
}

func convertKeysToCamelCase(v any, parentKey string) any {
	if v == nil {
		return nil
	}
	switch m := v.(type) {
	case map[string]any:
		newMap := make(map[string]any)
		for key, value := range m {
			camelCaseKey := toCamelCase(key)
			if parentKey == "response" && key == "body_segments" {
				newMap[camelCaseKey] = value
			} else {
				newMap[camelCaseKey] = convertKeysToCamelCase(value, key)
			}
		}
		return newMap
	case []any:
		newSlice := make([]any, len(m))
		for i, item := range m {
			newSlice[i] = convertKeysToCamelCase(item, parentKey)
		}
		return newSlice
	default:
		return v
	}
}

// toCamelCase converts a string from snake case to camel case.
// Examples:
//
//	"foo" -> "foo"
//	"fooBar" -> "fooBar"
//	"foo_bar" -> "fooBar"
//	"foo_bar_baz" -> "fooBarBaz"
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) == 1 {
		// There is no underscore, so no need to modify the string.
		return s
	}
	// Skip the first word and convert the first letter of the remaining words to uppercase.
	for i, part := range parts[1:] {
		parts[i+1] = strings.ToUpper(part[:1]) + part[1:]
	}
	// Concat the parts back together to mak a camelCase string.
	return strings.Join(parts, "")
}

var stringComparator = cmp.Comparer(func(x, y string) bool {
	if timeStringComparator(x, y) || base64StringComparator(x, y) || floatStringComparator(x, y) {
		return true
	}
	return x == y
})

var floatComparator = cmp.Comparer(func(x, y float64) bool {
	return math.Abs(x-y) < 1e-6
})

var floatStringComparator = func(x, y string) bool {
	vx, err := strconv.ParseFloat(x, 64)
	if err != nil {
		return x == y
	}
	vy, err := strconv.ParseFloat(y, 64)
	if err != nil {
		return x == y
	}
	return math.Abs(vx-vy) < 1e-6
}

var timeStringComparator = func(x, y string) bool {
	xTime, err := time.Parse(time.RFC3339, x)
	if err != nil {
		return x == y
	}
	yTime, err := time.Parse(time.RFC3339, y)
	if err != nil {
		return x == y
	}
	return xTime.Truncate(time.Microsecond).Equal(yTime.Truncate(time.Microsecond))
}

var base64StringComparator = func(x, y string) bool {
	stdBase64Handler := func(s string) ([]byte, error) {
		b, err := base64.URLEncoding.DecodeString(s)
		if err != nil {
			b, err = base64.StdEncoding.DecodeString(s)
			if err != nil {
				return nil, fmt.Errorf("invalid base64 string %s", s)
			}
		}
		return b, nil
	}

	xb, err := stdBase64Handler(x)
	if err != nil {
		return x == y
	}
	yb, err := stdBase64Handler(y)
	if err != nil {
		return x == y
	}
	return bytes.Equal(xb, yb)
}
