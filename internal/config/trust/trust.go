// Package trust provides config file trust verification. It hashes
// project-level config files and persists user trust decisions (both
// "yes" and "no") so that unchanged configs are handled silently on
// subsequent runs: trusted configs load without a prompt, rejected
// configs are excluded without a prompt, and new or modified configs
// trigger a trust prompt.
package trust

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/charmbracelet/crush/internal/home"
)

// trustFileName is the name of the JSON file that stores trusted
// config hashes alongside the global data config.
const trustFileName = "trusted_configs.json"

// rejectedPrefix marks a stored decision as a rejection. Entries are
// stored as "no:<hash>" so an unchanged rejected config is not
// re-prompted, while a modified one falls back to unknown and prompts
// again.
const rejectedPrefix = "no:"

// TrustStore manages trusted config file hashes. It is safe for
// concurrent use.
type TrustStore struct {
	mu   sync.RWMutex
	path string
	data map[string]string // absolute path -> sha256 hex hash, prefixed with "no:" for rejections
}

// New creates a TrustStore backed by a JSON file in the user's Crush
// data directory.
func New() *TrustStore {
	dataDir := filepath.Dir(globalDataPath())
	return &TrustStore{
		path: filepath.Join(dataDir, trustFileName),
		data: make(map[string]string),
	}
}

// Load reads the persisted trust store from disk. Missing or corrupt
// files are treated as an empty store rather than an error.
func (s *TrustStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = make(map[string]string)
			return nil
		}
		return fmt.Errorf("reading trust store: %w", err)
	}

	var data map[string]string
	if err := json.Unmarshal(raw, &data); err != nil {
		s.data = make(map[string]string)
		return nil // corrupt file; start fresh
	}
	s.data = data
	return nil
}

// Save persists the current trust store to disk.
func (s *TrustStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating trust store directory: %w", err)
	}

	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling trust store: %w", err)
	}

	if err := os.WriteFile(s.path, raw, 0o644); err != nil {
		return fmt.Errorf("writing trust store: %w", err)
	}
	return nil
}

// IsTrusted reports whether the file at the given path has been
// previously trusted and its content hash matches the stored value.
func (s *TrustStore) IsTrusted(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	hash, err := hashFile(abs)
	if err != nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data[abs] == hash
}

// Trust records the current content hash of the file at the given
// path as trusted.
func (s *TrustStore) Trust(path string) error {
	return s.record(path, false)
}

// Reject records the current content hash of the file at the given
// path as rejected. Unchanged rejected configs are excluded without
// prompting; once the content changes the decision no longer matches
// and the config is treated as unknown again.
func (s *TrustStore) Reject(path string) error {
	return s.record(path, true)
}

// record stores the current content hash of the file at the given
// path, optionally marked as a rejection.
func (s *TrustStore) record(path string, rejected bool) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	hash, err := hashFile(abs)
	if err != nil {
		return fmt.Errorf("hashing file: %w", err)
	}

	if rejected {
		hash = rejectedPrefix + hash
	}

	s.mu.Lock()
	s.data[abs] = hash
	s.mu.Unlock()

	return nil
}

// IsRejected reports whether the file at the given path has been
// explicitly rejected and its content is unchanged since.
func (s *TrustStore) IsRejected(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	hash, err := hashFile(abs)
	if err != nil {
		return false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.data[abs] == rejectedPrefix+hash
}

// Untrust removes the trust entry for the given path.
func (s *TrustStore) Untrust(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}

	s.mu.Lock()
	delete(s.data, abs)
	s.mu.Unlock()
}

// HasEntries reports whether the trust store contains any trusted
// entries. Used to distinguish a first run (empty store) from a
// subsequent run where the user has previously made trust decisions.
func (s *TrustStore) HasEntries() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data) > 0
}

// UntrustedPaths returns the subset of paths whose content has no
// matching trust decision: never seen before, or changed since the
// last trust or rejection. These are the configs that should trigger
// a trust prompt. Unchanged rejected configs are excluded because
// the user already answered "no" for that content.
func (s *TrustStore) UntrustedPaths(paths []string) []string {
	var untrusted []string
	for _, p := range paths {
		if s.IsTrusted(p) || s.IsRejected(p) {
			continue
		}
		untrusted = append(untrusted, p)
	}
	return untrusted
}

// TrustedPaths returns the subset of paths that are currently marked
// as trusted with a matching content hash.
func (s *TrustStore) TrustedPaths(paths []string) []string {
	var trusted []string
	for _, p := range paths {
		if s.IsTrusted(p) {
			trusted = append(trusted, p)
		}
	}
	return trusted
}

// RejectedPaths returns the subset of paths that are currently marked
// as rejected with a matching content hash.
func (s *TrustStore) RejectedPaths(paths []string) []string {
	var rejected []string
	for _, p := range paths {
		if s.IsRejected(p) {
			rejected = append(rejected, p)
		}
	}
	return rejected
}

// hashFile computes the SHA-256 hash of a file's contents and returns
// it as a hex-encoded string.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}

// globalDataPath returns the path to the global data config file.
// Duplicated here to avoid an import cycle with the config package.
func globalDataPath() string {
	if d := os.Getenv("CRUSH_GLOBAL_DATA"); d != "" {
		return filepath.Join(d, "crush.json")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "crush", "crush.json")
	}
	return filepath.Join(home.Dir(), ".local", "share", "crush", "crush.json")
}
