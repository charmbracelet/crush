// Package binary provisions the pinned official filesnap executable.
package binary

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/charmbracelet/crush/internal/lock"
)

const version = "0.5.0"
const maxArchive = 32 << 20
const maxBinary = 64 << 20

//go:embed platforms.json LICENSE NOTICE
var metadata embed.FS

type artifact struct {
	URL       string `json:"url"`
	Integrity string `json:"integrity"`
	Member    string `json:"member"`
	SHA256    string `json:"sha256"`
	Archive   string `json:"archive"`
}

func artifacts() (map[string]artifact, error) {
	data, err := metadata.ReadFile("platforms.json")
	if err != nil {
		return nil, err
	}
	var result map[string]artifact
	err = json.Unmarshal(data, &result)
	return result, err
}

// Supported reports whether an official executable exists for this platform.
func Supported() bool {
	entries, err := artifacts()
	if err != nil {
		return false
	}
	_, ok := entries[runtime.GOOS+"/"+runtime.GOARCH]
	return ok
}

func checkArchive(data []byte, a artifact) error {
	sum := sha512.Sum512(data)
	if "sha512-"+base64.StdEncoding.EncodeToString(sum[:]) != a.Integrity {
		return fmt.Errorf("filesnap archive integrity mismatch")
	}
	return nil
}

func download(ctx context.Context, client *http.Client, a artifact) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download filesnap %s: %w", version, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download filesnap: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxArchive+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxArchive {
		return nil, fmt.Errorf("filesnap archive exceeds size limit")
	}
	if err := checkArchive(data, a); err != nil {
		return nil, err
	}
	return data, nil
}

func extract(data []byte, a artifact) ([]byte, error) {
	if err := checkArchive(data, a); err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	reader := tar.NewReader(io.LimitReader(gz, maxBinary+1))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("filesnap executable missing from archive")
		}
		if err != nil {
			return nil, err
		}
		if header.Name != a.Member {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > maxBinary {
			return nil, fmt.Errorf("invalid filesnap executable archive entry")
		}
		content, err := io.ReadAll(io.LimitReader(reader, maxBinary+1))
		if err != nil {
			return nil, err
		}
		if !matches(content, a.SHA256) {
			return nil, fmt.Errorf("filesnap executable checksum mismatch")
		}
		return content, nil
	}
}

func matches(data []byte, expected string) bool {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) == expected
}

// Resolve returns a verified managed executable, without searching PATH.
func Resolve(ctx context.Context, dataDir string) (string, error) {
	entries, err := artifacts()
	if err != nil {
		return "", err
	}
	a, ok := entries[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("file history is unavailable on %s/%s: no official filesnap %s binary", runtime.GOOS, runtime.GOARCH, version)
	}
	return provision(ctx, dataDir, a, func(ctx context.Context) ([]byte, error) {
		data, err := bundledArchive(a.Archive)
		if err != nil {
			return nil, err
		}
		if data != nil {
			return data, nil
		}
		return download(ctx, &http.Client{Timeout: 2 * time.Minute}, a)
	})
}

func provision(ctx context.Context, dataDir string, a artifact, source func(context.Context) ([]byte, error)) (string, error) {
	directory := filepath.Join(dataDir, "runtime", "filesnap", version+"-"+a.SHA256)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	release, err := lock.File(ctx, filepath.Join(directory, "install.lock"))
	if err != nil {
		return "", err
	}
	defer release()
	name := "filesnap"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	destination := filepath.Join(directory, name)
	existing, err := os.ReadFile(destination)
	if err == nil {
		if !matches(existing, a.SHA256) {
			return "", fmt.Errorf("filesnap cache checksum mismatch; remove %s and retry", directory)
		}
		return destination, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	data, err := source(ctx)
	if err != nil {
		return "", err
	}
	content, err := extract(data, a)
	if err != nil {
		return "", err
	}
	for _, name := range []string{"LICENSE", "NOTICE"} {
		notice, err := metadata.ReadFile(name)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(directory, name), notice, 0o600); err != nil {
			return "", err
		}
	}
	temp, err := os.CreateTemp(directory, ".install-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Chmod(0o700); err != nil {
		temp.Close()
		return "", err
	}
	if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temp.Name(), destination); err != nil {
		return "", err
	}
	return destination, nil
}

// PrepareBundle downloads checked archives before a release or local build.
// An empty platform selects all supported targets; otherwise use GOOS/GOARCH.
func PrepareBundle(ctx context.Context, directory, platform string) error {
	entries, err := artifacts()
	if err != nil {
		return err
	}
	if platform != "" {
		selected, ok := entries[platform]
		if !ok {
			return fmt.Errorf("no official filesnap binary for %s", platform)
		}
		entries = map[string]artifact{platform: selected}
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	for _, a := range entries {
		destination := filepath.Join(directory, a.Archive)
		data, err := os.ReadFile(destination)
		if err == nil && checkArchive(data, a) == nil {
			continue
		}
		data, err = download(ctx, &http.Client{Timeout: 2 * time.Minute}, a)
		if err != nil {
			return err
		}
		if _, err = extract(data, a); err != nil {
			return err
		}
		if err = os.WriteFile(destination, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}
