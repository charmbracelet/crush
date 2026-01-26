package anthropic

import (
	"testing"
)

func TestGenerateCodeVerifier(t *testing.T) {
	verifier, err := generateCodeVerifier()
	if err != nil {
		t.Fatalf("generateCodeVerifier failed: %v", err)
	}

	if len(verifier) == 0 {
		t.Error("code verifier should not be empty")
	}

	// Verify it's URL-safe base64
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("code verifier length should be between 43-128 characters, got %d", len(verifier))
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test_verifier_1234567890"
	challenge := generateCodeChallenge(verifier)

	if len(challenge) == 0 {
		t.Error("code challenge should not be empty")
	}

	// SHA256 hash encoded in base64 should be 43 characters
	if len(challenge) != 43 {
		t.Errorf("code challenge should be 43 characters, got %d", len(challenge))
	}
}

func TestGenerateState(t *testing.T) {
	state, err := generateState()
	if err != nil {
		t.Fatalf("generateState failed: %v", err)
	}

	if len(state) == 0 {
		t.Error("state should not be empty")
	}

	// Verify uniqueness by generating multiple states
	state2, err := generateState()
	if err != nil {
		t.Fatalf("generateState failed: %v", err)
	}

	if state == state2 {
		t.Error("generated states should be unique")
	}
}

func TestInitiateAuth(t *testing.T) {
	params, err := InitiateAuth()
	if err != nil {
		t.Fatalf("InitiateAuth failed: %v", err)
	}

	if params == nil {
		t.Fatal("params should not be nil")
	}

	if params.AuthURL == "" {
		t.Error("AuthURL should not be empty")
	}

	if params.CodeVerifier == "" {
		t.Error("CodeVerifier should not be empty")
	}

	if params.State == "" {
		t.Error("State should not be empty")
	}

	if params.CodeChallenge == "" {
		t.Error("CodeChallenge should not be empty")
	}

	// Verify the auth URL contains required parameters
	if !contains(params.AuthURL, "code_challenge=") {
		t.Error("AuthURL should contain code_challenge parameter")
	}

	if !contains(params.AuthURL, "state=") {
		t.Error("AuthURL should contain state parameter")
	}

	if !contains(params.AuthURL, "client_id=") {
		t.Error("AuthURL should contain client_id parameter")
	}

	if !contains(params.AuthURL, "scope=") {
		t.Error("AuthURL should contain scope parameter")
	}
}

func TestBuildAuthURL(t *testing.T) {
	challenge := "test_challenge"
	state := "test_state"

	url := buildAuthURL(challenge, state)

	// Check for key parameters (values may be URL-encoded)
	expectedKeys := []string{
		"code_challenge=",
		"state=",
		"client_id=",
		"response_type=",
		"redirect_uri=",
		"scope=",
		"code_challenge_method=",
	}

	for _, key := range expectedKeys {
		if !contains(url, key) {
			t.Errorf("AuthURL should contain parameter key: %s", key)
		}
	}

	// Verify it starts with the authorize URL
	if !contains(url, authorizeURL) {
		t.Errorf("AuthURL should start with %s", authorizeURL)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
