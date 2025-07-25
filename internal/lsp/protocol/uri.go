// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol

// This file declares URI, DocumentUri, and its methods.
//
// For the LSP definition of these types, see
// https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#uri

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
)

// A DocumentURI is the URI of a client editor document.
//
// According to the LSP specification:
//
//	Care should be taken to handle encoding in URIs. For
//	example, some clients (such as VS Code) may encode colons
//	in drive letters while others do not. The URIs below are
//	both valid, but clients and servers should be consistent
//	with the form they use themselves to ensure the other party
//	doesn’t interpret them as distinct URIs. Clients and
//	servers should not assume that each other are encoding the
//	same way (for example a client encoding colons in drive
//	letters cannot assume server responses will have encoded
//	colons). The same applies to casing of drive letters - one
//	party should not assume the other party will return paths
//	with drive letters cased the same as it.
//
//	file:///c:/project/readme.md
//	file:///C%3A/project/readme.md
//
// This is done during JSON unmarshalling;
// see [DocumentURI.UnmarshalText] for details.
type DocumentURI string

// A URI is an arbitrary URL (e.g. https), not necessarily a file.
type URI = string

// UnmarshalText implements decoding of DocumentUri values.
//
// In particular, it implements a systematic correction of various odd
// features of the definition of DocumentUri in the LSP spec that
// appear to be workarounds for bugs in VS Code. For example, it may
// URI-encode the URI itself, so that colon becomes %3A, and it may
// send file://foo.go URIs that have two slashes (not three) and no
// hostname.
//
// We use UnmarshalText, not UnmarshalJSON, because it is called even
// for non-addressable values such as keys and values of map[K]V,
// where there is no pointer of type *K or *V on which to call
// UnmarshalJSON. (See Go issue #28189 for more detail.)
//
// Non-empty DocumentUris are valid "file"-scheme URIs.
// The empty DocumentUri is valid.
func (uri *DocumentURI) UnmarshalText(data []byte) (err error) {
	*uri, err = ParseDocumentURI(string(data))
	return
}

// Path returns the file path for the given URI.
//
// DocumentUri("").Path() returns the empty string.
//
// Path panics if called on a URI that is not a valid filename.
func (uri DocumentURI) Path() (string, error) {
	filename, err := filename(uri)
	if err != nil {
		// e.g. ParseRequestURI failed.
		//
		// This can only affect DocumentUris created by
		// direct string manipulation; all DocumentUris
		// received from the client pass through
		// ParseRequestURI, which ensures validity.
		return "", fmt.Errorf("invalid URI %q: %w", uri, err)
	}
	return filepath.FromSlash(filename), nil
}

// Dir returns the URI for the directory containing the receiver.
func (uri DocumentURI) Dir() (DocumentURI, error) {
	// XXX: Legacy comment:
	// This function could be more efficiently implemented by avoiding any call
	// to Path(), but at least consolidates URI manipulation.

	path, err := uri.DirPath()
	if err != nil {
		return "", fmt.Errorf("invalid URI %q: %w", uri, err)
	}

	return URIFromPath(path), nil
}

// DirPath returns the file path to the directory containing this URI, which
// must be a file URI.
func (uri DocumentURI) DirPath() (string, error) {
	path, err := uri.Path()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}

func filename(uri DocumentURI) (string, error) {
	if uri == "" {
		return "", nil
	}

	// This conservative check for the common case
	// of a simple non-empty absolute POSIX filename
	// avoids the allocation of a net.URL.
	if strings.HasPrefix(string(uri), "file:///") {
		rest := string(uri)[len("file://"):] // leave one slash
		for i := range len(rest) {
			b := rest[i]
			// Reject these cases:
			if b < ' ' || b == 0x7f || // control character
				b == '%' || b == '+' || // URI escape
				b == ':' || // Windows drive letter
				b == '@' || b == '&' || b == '?' { // authority or query
				goto slow
			}
		}
		return rest, nil
	}
slow:

	u, err := url.ParseRequestURI(string(uri))
	if err != nil {
		return "", err
	}
	if u.Scheme != fileScheme {
		return "", fmt.Errorf("only file URIs are supported, got %q from %q", u.Scheme, uri)
	}
	// If the URI is a Windows URI, we trim the leading "/" and uppercase
	// the drive letter, which will never be case sensitive.
	if isWindowsDriveURIPath(u.Path) {
		u.Path = strings.ToUpper(string(u.Path[1])) + u.Path[2:]
	}

	return u.Path, nil
}

// ParseDocumentURI interprets a string as a DocumentUri, applying VS
// Code workarounds; see [DocumentURI.UnmarshalText] for details.
func ParseDocumentURI(s string) (DocumentURI, error) {
	if s == "" {
		return "", nil
	}

	if !strings.HasPrefix(s, "file://") {
		return "", fmt.Errorf("DocumentUri scheme is not 'file': %s", s)
	}

	// VS Code sends URLs with only two slashes,
	// which are invalid. golang/go#39789.
	if !strings.HasPrefix(s, "file:///") {
		s = "file:///" + s[len("file://"):]
	}

	// Even though the input is a URI, it may not be in canonical form. VS Code
	// in particular over-escapes :, @, etc. Unescape and re-encode to canonicalize.
	path, err := url.PathUnescape(s[len("file://"):])
	if err != nil {
		return "", err
	}

	// File URIs from Windows may have lowercase drive letters.
	// Since drive letters are guaranteed to be case insensitive,
	// we change them to uppercase to remain consistent.
	// For example, file:///c:/x/y/z becomes file:///C:/x/y/z.
	if isWindowsDriveURIPath(path) {
		path = path[:1] + strings.ToUpper(string(path[1])) + path[2:]
	}
	u := url.URL{Scheme: fileScheme, Path: path}
	return DocumentURI(u.String()), nil
}

// URIFromPath returns DocumentUri for the supplied file path.
// Given "", it returns "".
func URIFromPath(path string) DocumentURI {
	if path == "" {
		return ""
	}
	if !isWindowsDrivePath(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	// Check the file path again, in case it became absolute.
	if isWindowsDrivePath(path) {
		path = "/" + strings.ToUpper(string(path[0])) + path[1:]
	}
	path = filepath.ToSlash(path)
	u := url.URL{
		Scheme: fileScheme,
		Path:   path,
	}
	return DocumentURI(u.String())
}

const fileScheme = "file"

// isWindowsDrivePath returns true if the file path is of the form used by
// Windows. We check if the path begins with a drive letter, followed by a ":".
// For example: C:/x/y/z.
func isWindowsDrivePath(path string) bool {
	if len(path) < 3 {
		return false
	}
	return unicode.IsLetter(rune(path[0])) && path[1] == ':'
}

// isWindowsDriveURIPath returns true if the file URI is of the format used by
// Windows URIs. The url.Parse package does not specially handle Windows paths
// (see golang/go#6027), so we check if the URI path has a drive prefix (e.g. "/C:").
func isWindowsDriveURIPath(uri string) bool {
	if len(uri) < 4 {
		return false
	}
	return uri[0] == '/' && unicode.IsLetter(rune(uri[1])) && uri[2] == ':'
}
