package honcho

import (
	"os"
	"path/filepath"
	"strings"
)

// maxIDLen bounds a derived identifier. Honcho accepts long IDs, but
// session keys are built from filesystem paths, which have no useful
// upper bound and would otherwise produce unwieldy identifiers.
const maxIDLen = 128

// NormalizeID converts arbitrary text into a stable Honcho
// identifier: lowercase, with every character outside the set Honcho
// accepts collapsed to single hyphens.
//
// Honcho validates ids against ^[a-zA-Z0-9_-]+$, so only letters,
// digits, underscore, and hyphen may survive. Dots and colons look
// like harmless structure in a session key but are rejected by the
// API, which fails the request that creates the session and every
// write that follows.
//
// Derivation must be deterministic across runs and machines, because
// the identifier is how Crush finds the memory it wrote last time. An
// input that normalizes to nothing yields "default" rather than an
// empty ID the API would reject.
func NormalizeID(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "default"
	}

	var b strings.Builder
	b.Grow(len(s))
	lastHyphen := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastHyphen = false
		case r == '_':
			b.WriteRune(r)
			lastHyphen = false
		default:
			// Everything else, including the separators a session
			// key is assembled from, becomes a hyphen.
			if !lastHyphen {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "default"
	}
	if len(out) > maxIDLen {
		out = strings.Trim(out[:maxIDLen], "-")
	}
	return out
}

// Identity is the resolved set of Honcho identifiers for one Crush
// session: which workspace to write to, which peers are involved, and
// which Honcho session the turns belong to.
type Identity struct {
	// Workspace isolates this application's data.
	Workspace string
	// UserPeer is the human Honcho builds a representation of.
	UserPeer string
	// AgentPeer is Crush itself.
	AgentPeer string
	// SessionKey is the Honcho session turns are written to.
	SessionKey string
}

// ObserverPeer returns the peer whose collection conclusions are
// written to and read from.
//
// In unified mode the user observes themselves, so several agents
// sharing a workspace contribute to and read from one collection. In
// directional mode the agent holds its own private view of the user.
func (i Identity) ObserverPeer(mode ObservationMode) string {
	if mode == ObserveDirectional {
		return i.AgentPeer
	}
	return i.UserPeer
}

// ChatTarget returns the target peer for a dialectic query, or the
// empty string when the peer should query its own self-collection.
//
// Honcho treats an absent target as "ask about yourself", which is
// exactly unified mode; directional mode must name the user
// explicitly.
func (i Identity) ChatTarget(mode ObservationMode) string {
	if mode == ObserveDirectional {
		return i.UserPeer
	}
	return ""
}

// SessionPeers describes how both peers participate in the session.
//
// The user is observed so Honcho builds a representation of them. The
// agent observes others so it can reason about the user, but is only
// observed itself when explicitly asked: the point of the integration
// is to remember the human, and modelling the assistant doubles
// reasoning cost for little gain.
func (i Identity) SessionPeers(agentObserveMe bool) map[string]PeerConfig {
	yes, no := true, false
	userObserveMe := yes
	agentObserved := no
	if agentObserveMe {
		agentObserved = yes
	}
	return map[string]PeerConfig{
		i.UserPeer: {
			ObserveMe:     &userObserveMe,
			ObserveOthers: &no,
		},
		i.AgentPeer: {
			ObserveMe:     &agentObserved,
			ObserveOthers: &yes,
		},
	}
}

// DeriveIdentity resolves the identifiers for a Crush session.
//
// workDir is the directory Crush was started in and crushSessionID is
// the current session's UUID; both feed the session strategy. Callers
// that have no session ID yet may pass an empty string, which only
// matters for the per-session strategy.
func DeriveIdentity(cfg Config, workDir, crushSessionID string) Identity {
	agent := NormalizeID(cfg.AgentPeer)
	if agent == "default" {
		agent = DefaultAgentPeer
	}
	user := NormalizeID(cfg.PeerName)
	if user == "default" {
		user = DefaultPeerName
	}
	// A peer cannot hold a representation of itself and act as the
	// observer of the other side, so break a collision rather than
	// silently merging the two identities.
	if user == agent {
		user = NormalizeID(user + "-user")
	}

	scope := deriveScope(cfg.SessionStrategy, workDir, crushSessionID)
	key := NormalizeID(string(cfg.SessionStrategy) + ":" + scope + ":" + agent)

	return Identity{
		Workspace:  NormalizeID(cfg.Workspace),
		UserPeer:   user,
		AgentPeer:  agent,
		SessionKey: key,
	}
}

// deriveScope computes the strategy-specific portion of a session key.
func deriveScope(strategy SessionStrategy, workDir, crushSessionID string) string {
	switch strategy {
	case StrategyGlobal:
		return "global"

	case StrategyPerSession:
		if crushSessionID != "" {
			return crushSessionID
		}
		// Without a session ID the only honest fallback is the
		// directory; a constant would silently merge unrelated work.
		return dirScope(workDir)

	case StrategyPerRepo:
		if root := repoRoot(workDir); root != "" {
			return filepath.Base(root)
		}
		return dirScope(workDir)

	case StrategyGitBranch:
		if branch := currentBranch(workDir); branch != "" {
			return filepath.Base(repoRootOr(workDir)) + ":" + branch
		}
		return dirScope(workDir)

	case StrategyPerDirectory:
		return dirScope(workDir)

	default:
		return dirScope(workDir)
	}
}

// dirScope renders a working directory as a scope. Paths inside a
// repository are made relative to its root so the same project scopes
// identically regardless of where it was cloned.
func dirScope(workDir string) string {
	if workDir == "" {
		return "default"
	}
	abs, err := filepath.Abs(workDir)
	if err != nil {
		abs = workDir
	}
	root := repoRoot(abs)
	if root == "" {
		return abs
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == "." {
		return filepath.Base(root)
	}
	return filepath.Base(root) + ":" + filepath.ToSlash(rel)
}

// repoRoot walks up from dir looking for a .git entry and returns the
// containing directory, or the empty string when there is none.
func repoRoot(dir string) string {
	if dir == "" {
		return ""
	}
	cur, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(cur, ".git")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		cur = parent
	}
}

// repoRootOr returns the repository root, falling back to dir itself.
func repoRootOr(dir string) string {
	if root := repoRoot(dir); root != "" {
		return root
	}
	return dir
}

// currentBranch reads the checked-out branch by parsing .git/HEAD
// directly. Shelling out to git would be slower and would fail in the
// sandboxed environments Crush sometimes runs in.
func currentBranch(dir string) string {
	root := repoRoot(dir)
	if root == "" {
		return ""
	}

	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	// In a worktree or submodule, .git is a file pointing at the real
	// git directory.
	if !info.IsDir() {
		data, err := os.ReadFile(gitPath)
		if err != nil {
			return ""
		}
		line := strings.TrimSpace(string(data))
		target, ok := strings.CutPrefix(line, "gitdir:")
		if !ok {
			return ""
		}
		gitPath = strings.TrimSpace(target)
		if !filepath.IsAbs(gitPath) {
			gitPath = filepath.Join(root, gitPath)
		}
	}

	head, err := os.ReadFile(filepath.Join(gitPath, "HEAD"))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(head))
	if ref, ok := strings.CutPrefix(line, "ref:"); ok {
		ref = strings.TrimSpace(ref)
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	// Detached HEAD: the commit itself is the best available scope.
	if len(line) >= 8 {
		return line[:8]
	}
	return ""
}
