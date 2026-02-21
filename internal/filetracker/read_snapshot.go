package filetracker

import (
	"fmt"
	"os"
)

type Snapshot struct {
	SizeBytes    int64
	ModTimeNanos int64
}

func SnapshotFromPath(path string) (Snapshot, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Snapshot{}, err
	}
	if info.IsDir() {
		return Snapshot{}, fmt.Errorf("path is a directory: %s", path)
	}
	return Snapshot{
		SizeBytes:    info.Size(),
		ModTimeNanos: info.ModTime().UnixNano(),
	}, nil
}

func ReadFileWithSnapshot(path string) ([]byte, Snapshot, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, Snapshot{}, err
	}
	snapshot, err := SnapshotFromPath(path)
	if err != nil {
		return nil, Snapshot{}, err
	}
	return content, snapshot, nil
}
