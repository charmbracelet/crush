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
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/httptransport"
)

// Client is the GenAI client. It provides access to the various GenAI services.
type Client struct {
	clientConfig ClientConfig
	// Models provides access to the Models service.
	Models *Models
	// Live provides access to the Live service.
	Live *Live
	// Caches provides access to the Caches service.
	Caches *Caches
	// Chats provides util functions for creating a new chat session.
	Chats *Chats
	// Files provides access to the Files service.
	Files *Files
	// Operations provides access to long-running operations.
	Operations *Operations
}

// Backend is the GenAI backend to use for the client.
type Backend int

const (
	// BackendUnspecified causes the backend determined automatically. If the
	// GOOGLE_GENAI_USE_VERTEXAI environment variable is set to "1" or "true", then
	// the backend is BackendVertexAI. Otherwise, if GOOGLE_GENAI_USE_VERTEXAI
	// is unset or set to any other value, then BackendGeminiAPI is used.  Explicitly
	// setting the backend in ClientConfig overrides the environment variable.
	BackendUnspecified Backend = iota
	// BackendGeminiAPI is the Gemini API backend.
	BackendGeminiAPI
	// BackendVertexAI is the Vertex AI backend.
	BackendVertexAI
)

// The Stringer interface for Backend.
func (t Backend) String() string {
	switch t {
	case BackendGeminiAPI:
		return "BackendGeminiAPI"
	case BackendVertexAI:
		return "BackendVertexAI"
	default:
		return "BackendUnspecified"
	}
}

// ClientConfig is the configuration for the GenAI client.
type ClientConfig struct {
	// Optional. API Key for GenAI. Required for BackendGeminiAPI.
	// Can also be set via the GOOGLE_API_KEY environment variable.
	// Get a Gemini API key: https://ai.google.dev/gemini-api/docs/api-key
	APIKey string

	// Optional. Backend for GenAI. See Backend constants. Defaults to BackendGeminiAPI unless explicitly set to BackendVertexAI,
	// or the environment variable GOOGLE_GENAI_USE_VERTEXAI is set to "1" or "true".
	Backend Backend

	// Optional. GCP Project ID for Vertex AI. Required for BackendVertexAI.
	// Can also be set via the GOOGLE_CLOUD_PROJECT environment variable.
	// Find your Project ID: https://cloud.google.com/resource-manager/docs/creating-managing-projects#identifying_projects
	Project string

	// Optional. GCP Location/Region for Vertex AI. Required for BackendVertexAI.
	// Can also be set via the GOOGLE_CLOUD_LOCATION or GOOGLE_CLOUD_REGION environment variable.
	// Generative AI locations: https://cloud.google.com/vertex-ai/generative-ai/docs/learn/locations.
	Location string

	// Optional. Google credentials.  If not specified, [Application Default Credentials] will be used.
	//
	// [Application Default Credentials]: https://developers.google.com/accounts/docs/application-default-credentials
	Credentials *auth.Credentials

	// Optional HTTP client to use. If nil, a default client will be created.
	// For Vertex AI, this client must handle authentication appropriately.
	HTTPClient *http.Client

	// Optional HTTP options to override.
	HTTPOptions HTTPOptions

	envVarProvider func() map[string]string
}

func defaultEnvVarProvider() map[string]string {
	vars := make(map[string]string)
	if v, ok := os.LookupEnv("GOOGLE_GENAI_USE_VERTEXAI"); ok {
		vars["GOOGLE_GENAI_USE_VERTEXAI"] = v
	}
	if v, ok := os.LookupEnv("GOOGLE_API_KEY"); ok {
		vars["GOOGLE_API_KEY"] = v
	}
	if v, ok := os.LookupEnv("GOOGLE_CLOUD_PROJECT"); ok {
		vars["GOOGLE_CLOUD_PROJECT"] = v
	}
	if v, ok := os.LookupEnv("GOOGLE_CLOUD_LOCATION"); ok {
		vars["GOOGLE_CLOUD_LOCATION"] = v
	}
	if v, ok := os.LookupEnv("GOOGLE_CLOUD_REGION"); ok {
		vars["GOOGLE_CLOUD_REGION"] = v
	}
	if v, ok := os.LookupEnv("GOOGLE_GEMINI_BASE_URL"); ok {
		vars["GOOGLE_GEMINI_BASE_URL"] = v
	}
	if v, ok := os.LookupEnv("GOOGLE_VERTEX_BASE_URL"); ok {
		vars["GOOGLE_VERTEX_BASE_URL"] = v
	}
	return vars
}

// NewClient creates a new GenAI client.
//
// You can configure the client by passing in a ClientConfig struct.
//
// If a nil ClientConfig is provided, the client will be configured using
// default settings and environment variables:
//
//   - Environment Variables for BackendGeminiAPI:
//
//   - GOOGLE_API_KEY: Required. Specifies the API key for the Gemini API.
//
//   - Environment Variables for BackendVertexAI:
//
//   - GOOGLE_GENAI_USE_VERTEXAI: Must be set to "1" or "true" to use the Vertex AI
//     backend.
//
//   - GOOGLE_CLOUD_PROJECT: Required. Specifies the GCP project ID.
//
//   - GOOGLE_CLOUD_LOCATION or GOOGLE_CLOUD_REGION: Required. Specifies the GCP
//     location/region.
//
// If using the Vertex AI backend and no credentials are provided in the
// ClientConfig, the client will attempt to use application default credentials.
func NewClient(ctx context.Context, cc *ClientConfig) (*Client, error) {
	if cc == nil {
		cc = &ClientConfig{}
	}

	if cc.envVarProvider == nil {
		cc.envVarProvider = defaultEnvVarProvider
	}
	envVars := cc.envVarProvider()

	if cc.Project != "" && cc.APIKey != "" {
		return nil, fmt.Errorf("project and API key are mutually exclusive in the client initializer. ClientConfig: %#v", cc)
	}
	if cc.Location != "" && cc.APIKey != "" {
		return nil, fmt.Errorf("location and API key are mutually exclusive in the client initializer. ClientConfig: %#v", cc)
	}

	if cc.Backend == BackendUnspecified {
		if v, ok := envVars["GOOGLE_GENAI_USE_VERTEXAI"]; ok {
			v = strings.ToLower(v)
			if v == "1" || v == "true" {
				cc.Backend = BackendVertexAI
			} else {
				cc.Backend = BackendGeminiAPI
			}
		} else {
			cc.Backend = BackendGeminiAPI
		}
	}

	// Only set the API key for MLDev API.
	if cc.APIKey == "" && cc.Backend == BackendGeminiAPI {
		cc.APIKey = envVars["GOOGLE_API_KEY"]
	}
	if cc.Project == "" {
		cc.Project = envVars["GOOGLE_CLOUD_PROJECT"]
	}
	if cc.Location == "" {
		if location, ok := envVars["GOOGLE_CLOUD_LOCATION"]; ok {
			cc.Location = location
		} else if location, ok := envVars["GOOGLE_CLOUD_REGION"]; ok {
			cc.Location = location
		}
	}

	if cc.Backend == BackendVertexAI {
		if cc.Project == "" {
			return nil, fmt.Errorf("project is required for Vertex AI backend. ClientConfig: %#v", cc)
		}
		if cc.Location == "" {
			return nil, fmt.Errorf("location is required for Vertex AI backend. ClientConfig: %#v", cc)
		}
	} else {
		if cc.APIKey == "" {
			return nil, fmt.Errorf("api key is required for Google AI backend. ClientConfig: %#v.\nYou can get the API key from https://ai.google.dev/gemini-api/docs/api-key", cc)
		}
	}

	if cc.Backend == BackendVertexAI && cc.Credentials == nil {
		cred, err := credentials.DetectDefault(&credentials.DetectOptions{
			Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to find default credentials: %w", err)
		}
		cc.Credentials = cred
	}

	baseURL := getBaseURL(cc.Backend, &cc.HTTPOptions, envVars)
	if baseURL != "" {
		cc.HTTPOptions.BaseURL = baseURL
	}
	if cc.HTTPOptions.BaseURL == "" && cc.Backend == BackendVertexAI {
		if cc.Location == "global" {
			cc.HTTPOptions.BaseURL = "https://aiplatform.googleapis.com/"
		} else {
			cc.HTTPOptions.BaseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com/", cc.Location)
		}
	} else if cc.HTTPOptions.BaseURL == "" {
		cc.HTTPOptions.BaseURL = "https://generativelanguage.googleapis.com/"
	}

	if cc.HTTPOptions.APIVersion == "" && cc.Backend == BackendVertexAI {
		cc.HTTPOptions.APIVersion = "v1beta1"
	} else if cc.HTTPOptions.APIVersion == "" {
		cc.HTTPOptions.APIVersion = "v1beta"
	}

	if cc.HTTPClient == nil {
		if cc.Backend == BackendVertexAI {
			quotaProjectID, err := cc.Credentials.QuotaProjectID(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get quota project ID: %w", err)
			}
			client, err := httptransport.NewClient(&httptransport.Options{
				Credentials: cc.Credentials,
				Headers: http.Header{
					"X-Goog-User-Project": []string{quotaProjectID},
				},
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create HTTP client: %w", err)
			}
			cc.HTTPClient = client
		} else {
			cc.HTTPClient = &http.Client{}
		}
	}

	ac := &apiClient{clientConfig: cc}
	c := &Client{
		clientConfig: *cc,
		Models:       &Models{apiClient: ac},
		Live:         &Live{apiClient: ac},
		Caches:       &Caches{apiClient: ac},
		Chats:        &Chats{apiClient: ac},
		Operations:   &Operations{apiClient: ac},
		Files:        &Files{apiClient: ac},
	}
	return c, nil
}

// ClientConfig returns the ClientConfig for the client.
//
// The returned ClientConfig is a copy of the ClientConfig used to create the client.
func (c Client) ClientConfig() ClientConfig {
	return c.clientConfig
}
