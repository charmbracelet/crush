package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"unicode"
)

var (
	ErrRejected             = errors.New("memory observation rejected by policy")
	secretAssignmentPattern = regexp.MustCompile(`(?i)(api[_ -]?key|access[_ -]?token|refresh[_ -]?token|secret|password|authorization)\s*[:=]\s*["']?[A-Za-z0-9_./+=-]{12,}`)
	bearerPattern           = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9_./+=-]{12,}`)
	reactionPattern         = regexp.MustCompile(`(?i)^(thanks|thank you|great|good|nice|cool|awesome|perfect|ok|okay|lol|haha|this sucks|bad|wrong)[.! ]*$`)
)

func NormalizeObservation(in Observation, project Project, autoApprove float64) (Observation, error) {
	in.Name = cleanLine(in.Name, 80)
	in.Description = cleanLine(in.Description, 180)
	in.Content = strings.TrimSpace(in.Content)
	in.SourceKind = cleanLine(in.SourceKind, 40)

	if !validKind(in.Kind) || !validScope(in.Scope) || in.Name == "" || in.Description == "" || in.Content == "" {
		return Observation{}, ErrRejected
	}
	if in.Kind == KindUser {
		in.Scope = ScopeGlobal
	}
	if in.Kind == KindProject {
		in.Scope = ScopeProject
	}
	if in.Scope == ScopeProject {
		if project.ID == "" {
			return Observation{}, ErrRejected
		}
		in.ProjectID = project.ID
	} else {
		in.ProjectID = ""
	}
	if in.Derivable || containsSecret(in.Name+"\n"+in.Description+"\n"+in.Content) || reactionPattern.MatchString(strings.TrimSpace(in.Content)) {
		return Observation{}, ErrRejected
	}
	if len(in.Content) > 4000 {
		in.Content = strings.TrimSpace(in.Content[:4000])
	}
	if in.Confidence < 0 {
		in.Confidence = 0
	}
	if in.Confidence > 1 {
		in.Confidence = 1
	}
	if in.Status == "" {
		switch {
		case in.Explicit || in.Pinned || in.Confidence >= autoApprove:
			in.Status = StatusActive
		case in.Confidence >= 0.60:
			in.Status = StatusPending
		default:
			return Observation{}, ErrRejected
		}
	}
	if in.Status != StatusActive && in.Status != StatusPending {
		return Observation{}, ErrRejected
	}
	return in, nil
}

func Fingerprint(in Observation) string {
	key := strings.Join([]string{
		string(in.Scope),
		strings.ToLower(in.ProjectID),
		string(in.Kind),
		normalizeText(in.Description),
	}, "\x00")
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func containsSecret(text string) bool {
	return secretAssignmentPattern.MatchString(text) || bearerPattern.MatchString(text)
}

func validScope(scope Scope) bool {
	return scope == ScopeGlobal || scope == ScopeProject
}

func validKind(kind Kind) bool {
	switch kind {
	case KindUser, KindFeedback, KindProject, KindReference:
		return true
	default:
		return false
	}
}

func cleanLine(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > limit {
		value = strings.TrimSpace(value[:limit])
	}
	return value
}

func normalizeText(value string) string {
	var b strings.Builder
	space := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			space = false
			continue
		}
		if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}
