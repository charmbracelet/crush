package plugins

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveRefDefaultsToTheHeadOfTheRepo(t *testing.T) {
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, nil)

	commit, err := f.client().ResolveRef(context.Background(), mustSource(t, testRepo), "")
	require.NoError(t, err)
	require.Equal(t, commitOne, commit.SHA)
	require.Equal(t, []string{"/repos/example/crush-plugins/commits/HEAD"}, f.seen)
}

func TestResolveRefAcceptsBranchesWithSlashes(t *testing.T) {
	f := newFakeGitHub(t)
	f.publish(testRepo, "release/2", commitV2, nil)

	commit, err := f.client().ResolveRef(context.Background(), mustSource(t, testRepo), "release/2")
	require.NoError(t, err)
	require.Equal(t, commitV2, commit.SHA)
	require.Equal(t, "/repos/example/crush-plugins/commits/release/2", f.seen[0])
}

func TestGitHubSendsTheHeadersGitHubWants(t *testing.T) {
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, nil)

	gh := f.client()
	gh.Token = "secret-token"
	_, err := gh.ResolveRef(context.Background(), mustSource(t, testRepo), "")
	require.NoError(t, err)
	require.Equal(t, "Bearer secret-token", f.auth)
	require.Equal(t, "application/vnd.github+json", f.accept)
	require.Regexp(t, `^crush/`, f.userAgent)
}

func TestGitHubReadsTheTokenFromTheEnvironment(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	t.Setenv("GH_TOKEN", "")
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, nil)

	_, err := f.client().ResolveRef(context.Background(), mustSource(t, testRepo), "")
	require.NoError(t, err)
	require.Equal(t, "Bearer env-token", f.auth)
}

func TestGitHubSendsNoTokenWhenNoneIsAvailable(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
	f := newFakeGitHub(t)
	f.publish(testRepo, DefaultRef, commitOne, nil)

	_, err := f.client().ResolveRef(context.Background(), mustSource(t, testRepo), "")
	require.NoError(t, err)
	require.Empty(t, f.auth)
}

func TestGitHubErrorsAreActionable(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{"missing", http.StatusNotFound, "not found. If it is private, export GITHUB_TOKEN"},
		{"bad token", http.StatusUnauthorized, "rejected the credentials"},
		{"forbidden", http.StatusForbidden, "refused the request (403)"},
		{"server", http.StatusInternalServerError, "returned 500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeGitHub(t)
			f.status["/repos/example/crush-plugins/commits/HEAD"] = tc.status
			_, err := f.client().ResolveRef(context.Background(), mustSource(t, testRepo), "")
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestResolveRefRejectsTraversalWithoutARequest(t *testing.T) {
	f := newFakeGitHub(t)
	_, err := f.client().ResolveRef(context.Background(), mustSource(t, testRepo), "../../private")
	require.ErrorContains(t, err, "invalid ref")
	_, err = f.client().Tarball(context.Background(), mustSource(t, testRepo), "..%2f")
	require.ErrorContains(t, err, "invalid commit")
	require.Empty(t, f.seen, "a rejected ref must never reach the network")
}

func TestTarballReportsAMissingCommit(t *testing.T) {
	f := newFakeGitHub(t)
	_, err := f.client().Tarball(context.Background(), mustSource(t, testRepo), commitOne)
	require.ErrorContains(t, err, "not found. If it is private")
}

func TestTarballRejectsAnInvalidRepoName(t *testing.T) {
	f := newFakeGitHub(t)
	_, err := f.client().Tarball(context.Background(), Source{Author: "..", Repo: "x"}, commitOne)
	require.ErrorContains(t, err, "invalid repository")
	require.Empty(t, f.seen)
}
