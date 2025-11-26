package claude

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// GetChallenge generates a PKCE verifier and its corresponding challenge.
func GetChallenge() (verifier string, challenge string, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	verifier = encodeBase64(bytes)
	hash := sha256.Sum256([]byte(verifier))
	challenge = encodeBase64(hash[:])
	return verifier, challenge, nil
}

func encodeBase64(input []byte) (encoded string) {
	encoded = base64.StdEncoding.EncodeToString(input)
	encoded = strings.ReplaceAll(encoded, "=", "")
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	return encoded
}
