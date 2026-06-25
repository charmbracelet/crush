package dialog

import (
	"regexp"
)

// awsSSOURLRe matches URLs commonly printed by `aws sso login` and
// related AWS SSO commands. It captures https:// URLs that contain
// typical SSO hostnames.
var awsSSOURLRe = regexp.MustCompile(`https://[^\s]+`)

// ExtractAWSSSOURL extracts the first HTTPS URL from command output.
// AWS SSO commands print a verification URL to stdout or stderr that
// the user must visit to complete authentication. Returns empty string
// if no URL is found.
func ExtractAWSSSOURL(output string) string {
	match := awsSSOURLRe.FindString(output)
	return match
}
