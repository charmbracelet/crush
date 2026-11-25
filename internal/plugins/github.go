package plugins

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/crush/internal/version"
)

const (
	// DefaultBaseURL is the GitHub REST API.
	DefaultBaseURL = "https://api.github.com"

	// resolveTimeout bounds a ref lookup, which is a small JSON reply.
	resolveTimeout = 30 * time.Second
	// downloadTimeout bounds a tarball, which can be a repository's history
	// worth of bytes on a slow link.
	downloadTimeout = 2 * time.Minute
)

// Commit is one resolved commit of a repository.
type Commit struct {
	SHA     string
	HTMLURL string
}

// GitHub fetches plugin repositories. BaseURL and Client exist so a test can
// point the client at a local server instead of api.github.com.
type GitHub struct {
	BaseURL string
	Client  *http.Client
	// Token authorizes requests for private repositories and raises the rate
	// limit. It is read from GITHUB_TOKEN or GH_TOKEN when empty.
	Token string
}

// DefaultGitHub builds a client for the command line.
func DefaultGitHub() GitHub {
	return GitHub{
		BaseURL: DefaultBaseURL,
		Client:  &http.Client{Timeout: downloadTimeout},
		Token:   cmp.Or(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN")),
	}
}

func (g GitHub) base() string {
	return cmp.Or(g.BaseURL, DefaultBaseURL)
}

func (g GitHub) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return &http.Client{Timeout: downloadTimeout}
}

// ResolveRef turns a branch, tag, or commit into the exact commit it points at.
// An empty ref resolves the repository's default branch.
func (g GitHub) ResolveRef(ctx context.Context, src Source, ref string) (Commit, error) {
	if !src.Valid() {
		return Commit{}, fmt.Errorf("invalid repository %q", src)
	}
	ref = cmp.Or(ref, DefaultRef)
	if !ValidRef(ref) {
		return Commit{}, fmt.Errorf("invalid ref %q", ref)
	}

	ctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	var reply struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
	}
	url := fmt.Sprintf("%s/repos/%s/%s/commits/%s", g.base(), src.Author, src.Repo, ref)
	if err := g.getJSON(ctx, src, url, &reply); err != nil {
		return Commit{}, err
	}
	if reply.SHA == "" {
		return Commit{}, fmt.Errorf("%s@%s resolved to no commit", src, ref)
	}
	return Commit{SHA: reply.SHA, HTMLURL: reply.HTMLURL}, nil
}

// Tarball streams the repository archive for one commit. The caller closes it.
func (g GitHub) Tarball(ctx context.Context, src Source, sha string) (io.ReadCloser, error) {
	if !src.Valid() {
		return nil, fmt.Errorf("invalid repository %q", src)
	}
	if !ValidRef(sha) {
		return nil, fmt.Errorf("invalid commit %q", sha)
	}
	url := fmt.Sprintf("%s/repos/%s/%s/tarball/%s", g.base(), src.Author, src.Repo, sha)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	g.decorate(req, "application/vnd.github+json")

	resp, err := g.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, apiError(src, resp)
	}
	return resp.Body, nil
}

func (g GitHub) getJSON(ctx context.Context, src Source, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	g.decorate(req, "application/vnd.github+json")

	resp, err := g.client().Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return apiError(src, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to parse the reply from %s: %w", url, err)
	}
	return nil
}

// decorate sets the headers GitHub requires, including the token that makes a
// private repository readable.
func (g GitHub) decorate(req *http.Request, accept string) {
	req.Header.Set("User-Agent", "crush/"+version.Version)
	req.Header.Set("Accept", accept)
	if token := cmp.Or(g.Token, cmp.Or(os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN"))); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// apiError explains a non-200 reply in terms a person can act on. GitHub
// answers 404 for a repository that exists but is not readable with the
// credentials offered, so the message has to name both possibilities.
func apiError(src Source, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	target := src.String()
	if target == "/" {
		target = "GitHub"
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%s not found. If it is private, export GITHUB_TOKEN with access to it", target)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s rejected the credentials in GITHUB_TOKEN or GH_TOKEN (401)", target)
	case http.StatusForbidden, http.StatusTooManyRequests:
		if strings.Contains(strings.ToLower(string(body)), "rate limit") {
			return fmt.Errorf("%s rate limit reached. Export GITHUB_TOKEN to raise it: %s", target, snippet(body))
		}
		return fmt.Errorf("%s refused the request (403): %s", target, snippet(body))
	default:
		return fmt.Errorf("%s returned %s: %s", target, resp.Status, snippet(body))
	}
}

func snippet(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if len(text) > 200 {
		text = text[:200] + "…"
	}
	return text
}
