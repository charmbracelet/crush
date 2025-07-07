// Package autolink provides a function to detect and format GitHub links into
// a more readable manner.
package autolink

import (
	"fmt"
	"regexp"
)

type pattern struct {
	pattern *regexp.Regexp
	yield   func(m []string) string
}

var patterns = []pattern{
	{
		regexp.MustCompile(`^https?://github\.com/([A-z0-9_-]+)/([A-z0-9_-]+)/(issues?|pulls?|discussions?)/([0-9]+)$`),
		func(m []string) string { return fmt.Sprintf("%s/%s#%s", m[1], m[2], m[4]) },
	},
	{
		regexp.MustCompile(`^https?://github\.com/([A-z0-9_-]+)/([A-z0-9_-]+)/(issues?|pulls?|discussions?)/([0-9]+)#issuecomment-[0-9]+$`),
		func(m []string) string { return fmt.Sprintf("%s/%s#%s (comment)", m[1], m[2], m[4]) },
	},
	{
		regexp.MustCompile(`^https?://github\.com/([A-z0-9_-]+)/([A-z0-9_-]+)/pulls?/([0-9]+)#discussion_r[0-9]+$`),
		func(m []string) string { return fmt.Sprintf("%s/%s#%s (comment)", m[1], m[2], m[3]) },
	},
	{
		regexp.MustCompile(`^https?://github\.com/([A-z0-9_-]+)/([A-z0-9_-]+)/pulls?/([0-9]+)#pullrequestreview-[0-9]+$`),
		func(m []string) string { return fmt.Sprintf("%s/%s#%s (review)", m[1], m[2], m[3]) },
	},
	{
		regexp.MustCompile(`^https?://github\.com/([A-z0-9_-]+)/([A-z0-9_-]+)/discussions/([0-9]+)#discussioncomment-[0-9]+$`),
		func(m []string) string { return fmt.Sprintf("%s/%s#%s (comment)", m[1], m[2], m[3]) },
	},
	{
		regexp.MustCompile(`^https?://github\.com/([A-z0-9_-]+)/([A-z0-9_-]+)/commit/([A-z0-9]{7,})(#.*)?$`),
		func(m []string) string { return fmt.Sprintf("%s/%s@%s", m[1], m[2], m[3][:7]) },
	},
	{
		regexp.MustCompile(`^https?://github\.com/([A-z0-9_-]+)/([A-z0-9_-]+)/pulls?/[0-9]+/commits/([A-z0-9]{7,})(#.*)?$`),
		func(m []string) string { return fmt.Sprintf("%s/%s@%s", m[1], m[2], m[3][:7]) },
	},
}

// Detect checks if the given URL matches any of the known patterns and
// returns a human-readable formatted string if a match is found.
func Detect(u string) (string, bool) {
	for _, p := range patterns {
		if m := p.pattern.FindStringSubmatch(u); len(m) > 0 {
			return p.yield(m), true
		}
	}
	return "", false
}
