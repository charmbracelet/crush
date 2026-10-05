package plugins

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

// fakeGitHub stands in for api.github.com. A repository maps a ref to a commit
// and a commit to the files at its root, so a test simulates an upstream push
// by publishing a new commit under the same ref.
type fakeGitHub struct {
	t          *testing.T
	server     *httptest.Server
	byRef      map[string]string
	byCommit   map[string]map[string]string
	status     map[string]int
	seen       []string
	auth       string
	userAgent  string
	accept     string
	rawArchive []byte
	archiveTop string
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{
		t:        t,
		byRef:    map[string]string{},
		byCommit: map[string]map[string]string{},
		status:   map[string]int{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) client() GitHub {
	return GitHub{BaseURL: f.server.URL, Client: f.server.Client()}
}

// publish registers that ref resolves to sha, whose archive holds files.
func (f *fakeGitHub) publish(source, ref, sha string, files map[string]string) {
	f.byRef[source+"@"+ref] = sha
	f.byCommit[sha] = files
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	f.seen = append(f.seen, r.URL.Path)
	f.auth = r.Header.Get("Authorization")
	f.userAgent = r.Header.Get("User-Agent")
	f.accept = r.Header.Get("Accept")

	if code, ok := f.status[r.URL.Path]; ok {
		w.WriteHeader(code)
		fmt.Fprint(w, `{"message":"forced failure"}`)
		return
	}

	parts := strings.SplitN(strings.Trim(r.URL.Path, "/"), "/", 4)
	if len(parts) != 4 || parts[0] != "repos" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// A ref may itself contain slashes, so it is the remainder of the path.
	kindRef := strings.SplitN(parts[3], "/", 2)
	if len(kindRef) != 2 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	source, kind, ref := parts[1]+"/"+parts[2], kindRef[0], kindRef[1]

	switch kind {
	case "commits":
		sha, ok := f.byRef[source+"@"+ref]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"sha":      sha,
			"html_url": "https://github.com/" + source + "/commit/" + sha,
		})
	case "tarball":
		files, ok := f.byCommit[ref]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-gzip")
		if f.rawArchive != nil {
			_, _ = w.Write(f.rawArchive)
			return
		}
		top := f.archiveTop
		if top == "" {
			top = "repo-" + ref
		}
		_, _ = w.Write(tarGz(f.t, top, files))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// tarGz builds a repository archive: every entry lives under one top-level
// directory, exactly as GitHub's does.
func tarGz(t *testing.T, top string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		content := []byte(files[name])
		if err := tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     top + "/" + name,
			Size:     int64(len(content)),
			Mode:     0o644,
		}); err != nil {
			t.Fatalf("failed to write archive header: %v", err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("failed to write archive body: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close gzip: %v", err)
	}
	return buf.Bytes()
}
