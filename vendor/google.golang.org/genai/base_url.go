// Copyright 2025 Google LLC
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

var defaultBaseGeminiURL string = ""
var defaultBaseVertexURL string = ""

// BaseURLParameters are parameters for setting the base URLs for the Gemini API and Vertex AI API.
type BaseURLParameters struct {
	GeminiURL string
	VertexURL string
}

// SetDefaultBaseURLs overrides the base URLs for the Gemini API and Vertex AI API.
//
// [HTTPOptions.BaseURL] takes precedence over URLs set here.
//
// Note: This function should be called before initializing the SDK. If the
// base URLs are set after initializing the SDK, the base URLs will not be
// updated.
func SetDefaultBaseURLs(baseURLParams BaseURLParameters) {
	defaultBaseGeminiURL = baseURLParams.GeminiURL
	defaultBaseVertexURL = baseURLParams.VertexURL
}

// getDefaultBaseURLs returns the default base URLs for the Gemini API and Vertex AI API.
func getDefaultBaseURLs() *BaseURLParameters {
	return &BaseURLParameters{
		GeminiURL: defaultBaseGeminiURL,
		VertexURL: defaultBaseVertexURL,
	}
}

// getBaseURL returns the default base URL based on the following priority:
//
//  1. Base URLs set via HTTPOptions.
//  2. Base URLs set via the latest call to SetDefaultBaseURLs.
//  3. Base URLs set via environment variables.
func getBaseURL(backend Backend, httpOptions *HTTPOptions, envVars map[string]string) string {
	if httpOptions != nil && httpOptions.BaseURL != "" {
		return httpOptions.BaseURL
	}
	baseURLs := getDefaultBaseURLs()
	if backend == BackendVertexAI {
		if baseURLs.VertexURL != "" {
			return baseURLs.VertexURL
		} else if v := envVars["GOOGLE_VERTEX_BASE_URL"]; v != "" {
			return v
		}
	} else {
		if baseURLs.GeminiURL != "" {
			return baseURLs.GeminiURL
		} else if v := envVars["GOOGLE_GEMINI_BASE_URL"]; v != "" {
			return v
		}
	}

	return ""
}
