package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Store handles reading and writing raw config JSON data to persistent
// storage.
type Store interface {
	Read() ([]byte, error)
	Write(data []byte) error
}

// FileStore is a Store backed by a file on disk.
type FileStore struct {
	path string
}

// NewFileStore creates a new FileStore at the given path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Read returns the raw bytes from the backing file. If the file does not
// exist an empty JSON object is returned.
func (s *FileStore) Read() ([]byte, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte("{}"), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	return data, nil
}

// Write persists raw bytes to the backing file, creating parent directories
// as needed. The JSON is pretty-printed with two-space indentation before
// writing.
func (s *FileStore) Write(data []byte) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return fmt.Errorf("failed to format config JSON: %w", err)
	}
	buf.WriteByte('\n')
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %q: %w", s.path, err)
	}
	if err := os.WriteFile(s.path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}

// HasField returns true if the given dotted key path exists in the store.
func HasField(s Store, key string) bool {
	data, err := s.Read()
	if err != nil {
		return false
	}
	return gjson.Get(string(data), key).Exists()
}

// SetField sets a value at the given dotted key path and persists it.
func SetField(s Store, key string, value any) error {
	data, err := s.Read()
	if err != nil {
		return err
	}

	updated, err := sjson.Set(string(data), key, value)
	if err != nil {
		return fmt.Errorf("failed to set config field %s: %w", key, err)
	}
	return s.Write([]byte(updated))
}

// RemoveField deletes a value at the given dotted key path and persists it.
func RemoveField(s Store, key string) error {
	data, err := s.Read()
	if err != nil {
		return err
	}

	updated, err := sjson.Delete(string(data), key)
	if err != nil {
		return fmt.Errorf("failed to delete config field %s: %w", key, err)
	}
	return s.Write([]byte(updated))
}
