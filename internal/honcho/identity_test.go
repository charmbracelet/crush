package honcho

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "already normal", in: "crush", want: "crush"},
		{name: "uppercase", in: "CRUSH", want: "crush"},
		{name: "mixed case", in: "CrUsH", want: "crush"},
		{name: "spaces", in: "hello world", want: "hello-world"},
		{name: "runs of spaces collapse", in: "a     b", want: "a-b"},
		{name: "surrounding whitespace", in: "   padded   ", want: "padded"},
		{name: "slashes", in: "a/b/c", want: "a-b-c"},
		{name: "windows path", in: `C:\Users\Kieran`, want: "c-users-kieran"},
		{name: "leading and trailing junk", in: "!!!abc???", want: "abc"},
		{name: "only symbols", in: "!@#$%^&*()", want: "default"},
		{name: "only whitespace", in: "   ", want: "default"},
		{name: "empty", in: "", want: "default"},
		{name: "unicode is replaced", in: "café", want: "caf"},
		{name: "unicode inside", in: "naïve-user", want: "na-ve-user"},
		{name: "cjk collapses", in: "世界", want: "default"},
		{name: "emoji between words", in: "a🎉b", want: "a-b"},
		{name: "dots and colons fold to hyphens", in: "a_b-c.d:e", want: "a_b-c-d-e"},
		{name: "separators around junk", in: "a_ _b", want: "a_-_b"},
		{name: "digits", in: "ws-2024", want: "ws-2024"},
		{name: "consecutive hyphens collapse", in: "a--b", want: "a-b"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeID(tc.in)
			require.Equal(t, tc.want, got)
			require.NotEmpty(t, got)
		})
	}
}

func TestNormalizeIDLength(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{name: "long run of letters", in: strings.Repeat("a", 500)},
		{name: "long path", in: strings.Repeat("/some/deep/dir", 40)},
		{name: "hyphen lands on the cut", in: strings.Repeat("a", maxIDLen-1) + "  tail"},
		{name: "all junk past the cut", in: strings.Repeat("a", maxIDLen) + strings.Repeat("!", 50)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeID(tc.in)
			require.LessOrEqual(t, len(got), maxIDLen)
			require.NotEmpty(t, got)
			require.False(t, strings.HasSuffix(got, "-"), "trailing hyphen in %q", got)
			require.False(t, strings.HasPrefix(got, "-"), "leading hyphen in %q", got)
		})
	}

	// The hyphen-on-the-cut case trims back to plain letters.
	require.Equal(t,
		strings.Repeat("a", maxIDLen-1),
		NormalizeID(strings.Repeat("a", maxIDLen-1)+"  tail"))
}

func TestNormalizeIDIsDeterministic(t *testing.T) {
	t.Parallel()

	in := "Some Mixed/Input: 42"
	first := NormalizeID(in)
	for range 5 {
		require.Equal(t, first, NormalizeID(in))
	}
	// Normalizing an already-normalized value is a no-op.
	require.Equal(t, first, NormalizeID(first))
}

func TestDeriveIdentityPeerCollision(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		cfg       Config
		wantUser  string
		wantAgent string
	}{
		{
			name:      "distinct peers are left alone",
			cfg:       Config{PeerName: "kieran", AgentPeer: "crush"},
			wantUser:  "kieran",
			wantAgent: "crush",
		},
		{
			name:      "identical peers are broken apart",
			cfg:       Config{PeerName: "crush", AgentPeer: "crush"},
			wantUser:  "crush-user",
			wantAgent: "crush",
		},
		{
			name:      "collision after normalization",
			cfg:       Config{PeerName: "CRUSH", AgentPeer: "crush"},
			wantUser:  "crush-user",
			wantAgent: "crush",
		},
		{
			name:      "collision on a custom name",
			cfg:       Config{PeerName: "robot", AgentPeer: "robot"},
			wantUser:  "robot-user",
			wantAgent: "robot",
		},
		{
			name:      "empty peers fall back to defaults",
			cfg:       Config{},
			wantUser:  DefaultPeerName,
			wantAgent: DefaultAgentPeer,
		},
		{
			name:      "unnormalizable peers fall back to defaults",
			cfg:       Config{PeerName: "!!!", AgentPeer: "???"},
			wantUser:  DefaultPeerName,
			wantAgent: DefaultAgentPeer,
		},
		{
			name:      "agent named like the default user",
			cfg:       Config{PeerName: "user", AgentPeer: "user"},
			wantUser:  "user-user",
			wantAgent: "user",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			id := DeriveIdentity(tc.cfg, t.TempDir(), "")
			require.Equal(t, tc.wantUser, id.UserPeer)
			require.Equal(t, tc.wantAgent, id.AgentPeer)
			require.NotEqual(t, id.UserPeer, id.AgentPeer,
				"user and agent peer must never be the same identity")
		})
	}
}

func TestDeriveIdentityWorkspace(t *testing.T) {
	t.Parallel()

	require.Equal(t, "my-ws",
		DeriveIdentity(Config{Workspace: "My WS"}, t.TempDir(), "").Workspace)
	require.Equal(t, "default",
		DeriveIdentity(Config{Workspace: ""}, t.TempDir(), "").Workspace)
}

// fakeRepo creates a directory containing a .git directory and returns
// the repository root.
func fakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))
	return root
}

// subdir creates and returns a directory beneath parent.
func subdir(t *testing.T, parent string, parts ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{parent}, parts...)...)
	require.NoError(t, os.MkdirAll(p, 0o755))
	return p
}

// writeHEAD writes a .git/HEAD file inside root's git directory.
func writeHEAD(t *testing.T, gitDir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(content), 0o644))
}

func keyFor(strategy SessionStrategy, workDir, sessionID string) string {
	return DeriveIdentity(Config{SessionStrategy: strategy}, workDir, sessionID).SessionKey
}

func TestSessionStrategyPerDirectory(t *testing.T) {
	t.Parallel()

	repo := fakeRepo(t)
	a := subdir(t, repo, "pkg", "a")
	b := subdir(t, repo, "pkg", "b")

	keyA := keyFor(StrategyPerDirectory, a, "")
	keyB := keyFor(StrategyPerDirectory, b, "")

	require.NotEqual(t, keyA, keyB, "different directories must not share a session")
	require.Equal(t, keyA, keyFor(StrategyPerDirectory, a, ""), "derivation must be deterministic")
	require.Equal(t, keyA, keyFor(StrategyPerDirectory, a, "some-crush-session"),
		"per-directory must ignore the Crush session ID")
	require.Contains(t, keyA, "per-directory")
	require.NotEmpty(t, keyB)
}

func TestSessionStrategyPerDirectoryOutsideRepo(t *testing.T) {
	t.Parallel()

	a := t.TempDir()
	b := t.TempDir()

	require.NotEqual(t, keyFor(StrategyPerDirectory, a, ""), keyFor(StrategyPerDirectory, b, ""))
	require.Equal(t, keyFor(StrategyPerDirectory, a, ""), keyFor(StrategyPerDirectory, a, ""))
}

func TestSessionStrategyPerDirectoryEmptyWorkDir(t *testing.T) {
	t.Parallel()

	key := keyFor(StrategyPerDirectory, "", "")
	require.NotEmpty(t, key)
	require.Contains(t, key, "default")
}

func TestSessionStrategyPerRepo(t *testing.T) {
	t.Parallel()

	repo := fakeRepo(t)
	a := subdir(t, repo, "pkg", "a")
	b := subdir(t, repo, "cmd", "b")

	keyA := keyFor(StrategyPerRepo, a, "")
	keyB := keyFor(StrategyPerRepo, b, "")

	require.Equal(t, keyA, keyB, "subdirectories of one repo must share a session")
	require.Equal(t, keyA, keyFor(StrategyPerRepo, repo, ""))
	require.Contains(t, keyA, "per-repo")

	// A different repository still gets its own session.
	other := fakeRepo(t)
	require.NotEqual(t, keyA, keyFor(StrategyPerRepo, other, ""))
}

func TestSessionStrategyPerRepoOutsideRepo(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// With no repository the strategy falls back to the directory.
	require.Equal(t,
		NormalizeID("per-repo:"+dirScope(dir)+":"+DefaultAgentPeer),
		keyFor(StrategyPerRepo, dir, ""))
}

func TestSessionStrategyGlobal(t *testing.T) {
	t.Parallel()

	repo := fakeRepo(t)
	a := subdir(t, repo, "a")
	b := t.TempDir()

	key := keyFor(StrategyGlobal, a, "")
	require.Equal(t, key, keyFor(StrategyGlobal, b, ""))
	require.Equal(t, key, keyFor(StrategyGlobal, "", "sess-1"))
	require.Equal(t, key, keyFor(StrategyGlobal, b, "sess-2"))
	require.Equal(t, "global-global-crush", key)
}

func TestSessionStrategyPerSession(t *testing.T) {
	t.Parallel()

	dir := fakeRepo(t)

	key1 := keyFor(StrategyPerSession, dir, "11111111-1111-1111-1111-111111111111")
	key2 := keyFor(StrategyPerSession, dir, "22222222-2222-2222-2222-222222222222")

	require.NotEqual(t, key1, key2, "different Crush sessions must not share a Honcho session")
	require.Equal(t, key1, keyFor(StrategyPerSession, dir, "11111111-1111-1111-1111-111111111111"))
	require.Contains(t, key1, "11111111-1111-1111-1111-111111111111")

	// Without a session ID the directory is the honest fallback.
	require.Equal(t,
		NormalizeID("per-session:"+dirScope(dir)+":"+DefaultAgentPeer),
		keyFor(StrategyPerSession, dir, ""))
}

func TestSessionStrategyGitBranch(t *testing.T) {
	t.Parallel()

	repo := fakeRepo(t)
	gitDir := filepath.Join(repo, ".git")
	writeHEAD(t, gitDir, "ref: refs/heads/feature-x\n")

	sub := subdir(t, repo, "internal", "thing")
	key := keyFor(StrategyGitBranch, repo, "")

	require.Contains(t, key, "feature-x")
	require.Contains(t, key, "git-branch")
	require.Equal(t, key, keyFor(StrategyGitBranch, sub, ""),
		"the branch scopes the whole repo, not a subdirectory")

	// Switching branches switches sessions.
	writeHEAD(t, gitDir, "ref: refs/heads/main\n")
	other := keyFor(StrategyGitBranch, repo, "")
	require.Contains(t, other, "main")
	require.NotEqual(t, key, other)
}

func TestSessionStrategyGitBranchNested(t *testing.T) {
	t.Parallel()

	repo := fakeRepo(t)
	writeHEAD(t, filepath.Join(repo, ".git"), "ref: refs/heads/release/2.0\n")

	key := keyFor(StrategyGitBranch, repo, "")
	// The slash in the ref and the dot in the version both normalize
	// to hyphens, since Honcho ids allow neither.
	require.Contains(t, key, "release-2-0")
}

func TestSessionStrategyGitBranchDetachedHEAD(t *testing.T) {
	t.Parallel()

	repo := fakeRepo(t)
	const sha = "3f2a1b9c8d7e6f504132231445566778899aabbc"
	require.Len(t, sha, 40)
	writeHEAD(t, filepath.Join(repo, ".git"), sha+"\n")

	key := keyFor(StrategyGitBranch, repo, "")
	require.Contains(t, key, sha[:8])
	require.NotContains(t, key, sha[:9],
		"only the short SHA should enter the key")
}

func TestSessionStrategyGitBranchWorktree(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		absolute bool
	}{
		{name: "absolute gitdir", absolute: true},
		{name: "relative gitdir", absolute: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			realGit := subdir(t, root, "real-git-dir")
			writeHEAD(t, realGit, "ref: refs/heads/worktree-branch\n")

			worktree := subdir(t, root, "wt")
			target := realGit
			if !tc.absolute {
				target = filepath.Join("..", "real-git-dir")
			}
			require.NoError(t, os.WriteFile(
				filepath.Join(worktree, ".git"),
				[]byte("gitdir: "+target+"\n"),
				0o644,
			))

			key := keyFor(StrategyGitBranch, worktree, "")
			require.Contains(t, key, "worktree-branch")
		})
	}
}

func TestSessionStrategyGitBranchFallsBackWithoutHEAD(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		// setup prepares a directory and returns the work dir.
		setup func(t *testing.T) string
	}{
		{
			name:  "no repository",
			setup: func(t *testing.T) string { return t.TempDir() },
		},
		{
			name:  "repository with no HEAD",
			setup: func(t *testing.T) string { return fakeRepo(t) },
		},
		{
			name: "dot-git file without a gitdir line",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, ".git"), []byte("nonsense\n"), 0o644))
				return dir
			},
		},
		{
			name: "HEAD too short to be a SHA",
			setup: func(t *testing.T) string {
				repo := fakeRepo(t)
				writeHEAD(t, filepath.Join(repo, ".git"), "abc\n")
				return repo
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := tc.setup(t)
			require.Equal(t,
				NormalizeID("git-branch:"+dirScope(dir)+":"+DefaultAgentPeer),
				keyFor(StrategyGitBranch, dir, ""))
		})
	}
}

func TestSessionStrategyUnknownFallsBackToDirectory(t *testing.T) {
	t.Parallel()

	dir := fakeRepo(t)
	require.Equal(t,
		NormalizeID("wat:"+dirScope(dir)+":"+DefaultAgentPeer),
		keyFor(SessionStrategy("wat"), dir, ""))
}

func TestSessionKeyIncludesAgentPeer(t *testing.T) {
	t.Parallel()

	dir := fakeRepo(t)
	one := DeriveIdentity(Config{SessionStrategy: StrategyGlobal, AgentPeer: "crush"}, dir, "")
	two := DeriveIdentity(Config{SessionStrategy: StrategyGlobal, AgentPeer: "other"}, dir, "")
	require.NotEqual(t, one.SessionKey, two.SessionKey,
		"different agents must not write into one session")
}

func TestIdentityObserverPeer(t *testing.T) {
	t.Parallel()

	id := Identity{UserPeer: "kieran", AgentPeer: "crush"}

	cases := []struct {
		mode ObservationMode
		want string
	}{
		{mode: ObserveUnified, want: "kieran"},
		{mode: ObserveDirectional, want: "crush"},
		{mode: ObservationMode(""), want: "kieran"},
		{mode: ObservationMode("nonsense"), want: "kieran"},
	}

	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, id.ObserverPeer(tc.mode))
		})
	}
}

func TestIdentityChatTarget(t *testing.T) {
	t.Parallel()

	id := Identity{UserPeer: "kieran", AgentPeer: "crush"}

	cases := []struct {
		mode ObservationMode
		want string
	}{
		{mode: ObserveUnified, want: ""},
		{mode: ObserveDirectional, want: "kieran"},
		{mode: ObservationMode(""), want: ""},
		{mode: ObservationMode("nonsense"), want: ""},
	}

	for _, tc := range cases {
		t.Run(string(tc.mode), func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, id.ChatTarget(tc.mode))
		})
	}
}

func TestIdentitySessionPeers(t *testing.T) {
	t.Parallel()

	id := Identity{UserPeer: "kieran", AgentPeer: "crush"}

	for _, agentObserveMe := range []bool{false, true} {
		t.Run(map[bool]string{false: "agent unobserved", true: "agent observed"}[agentObserveMe], func(t *testing.T) {
			t.Parallel()

			peers := id.SessionPeers(agentObserveMe)
			require.Len(t, peers, 2)

			user, ok := peers["kieran"]
			require.True(t, ok)
			require.NotNil(t, user.ObserveMe)
			require.True(t, *user.ObserveMe, "the user is always observed")
			require.NotNil(t, user.ObserveOthers)
			require.False(t, *user.ObserveOthers)

			agent, ok := peers["crush"]
			require.True(t, ok)
			require.NotNil(t, agent.ObserveMe)
			require.Equal(t, agentObserveMe, *agent.ObserveMe)
			require.NotNil(t, agent.ObserveOthers)
			require.True(t, *agent.ObserveOthers, "the agent always observes the user")
		})
	}
}

func TestIdentitySessionPeersPointersAreIndependent(t *testing.T) {
	t.Parallel()

	id := Identity{UserPeer: "u", AgentPeer: "a"}
	peers := id.SessionPeers(true)

	// Every true-valued flag must be its own variable, or clearing
	// one would silently clear the others.
	*peers["u"].ObserveMe = false
	require.True(t, *peers["a"].ObserveMe)
	require.True(t, *peers["a"].ObserveOthers)

	*peers["a"].ObserveMe = false
	require.True(t, *peers["a"].ObserveOthers)

	fresh := id.SessionPeers(true)
	require.True(t, *fresh["u"].ObserveMe, "a later call must not see earlier mutations")
	require.True(t, *fresh["a"].ObserveMe)
	require.True(t, *fresh["a"].ObserveOthers)
}

// honchoIDPattern is the server-side constraint on every id Crush
// sends. Honcho rejects anything else with a 422, which fails session
// creation and then silently drops every write that follows.
var honchoIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func TestDerivedIDsAreAcceptedByHoncho(t *testing.T) {
	t.Parallel()

	// Inputs chosen for the characters that look harmless in an id
	// and are not: dots in versions and hostnames, colons in Windows
	// paths and in the separators a session key is assembled from.
	dirs := []string{
		t.TempDir(),
		`C:\Users\Kieran\code`,
		"/home/user/my.project",
		"/tmp/release-2.0",
		"",
	}
	cfgs := []Config{
		{Workspace: "crush", PeerName: "user", AgentPeer: "crush"},
		{Workspace: "my.project", PeerName: "a.b:c", AgentPeer: "crush.dev"},
		{Workspace: "", PeerName: "", AgentPeer: ""},
	}

	for _, strategy := range []SessionStrategy{
		StrategyPerDirectory, StrategyPerRepo, StrategyGitBranch,
		StrategyPerSession, StrategyGlobal,
	} {
		for _, cfg := range cfgs {
			for _, dir := range dirs {
				cfg.SessionStrategy = strategy
				id := DeriveIdentity(cfg, dir, "sess:1.2")
				for name, value := range map[string]string{
					"workspace": id.Workspace,
					"user peer": id.UserPeer,
					"agent":     id.AgentPeer,
					"session":   id.SessionKey,
				} {
					require.Regexp(t, honchoIDPattern, value,
						"%s id must satisfy Honcho's id pattern (strategy %q, dir %q)", name, strategy, dir)
				}
			}
		}
	}
}
