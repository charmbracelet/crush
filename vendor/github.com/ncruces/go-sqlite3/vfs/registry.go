package vfs

import "sync"

var (
	// +checklocks:vfsRegistryMtx
	vfsRegistry    map[string]VFS
	vfsRegistryMtx sync.RWMutex
)

// Find returns a VFS given its name.
// If there is no match, nil is returned.
// If name is empty, the default VFS is returned.
//
// https://sqlite.org/c3ref/vfs_find.html
func Find(name string) VFS {
	if name == "" || name == "os" {
		return vfsOS{}
	}
	vfsRegistryMtx.RLock()
	defer vfsRegistryMtx.RUnlock()
	return vfsRegistry[name]
}

// Register registers a VFS.
// Empty and "os" are reserved names.
//
// https://sqlite.org/c3ref/vfs_find.html
func Register(name string, vfs VFS) {
	if name == "" || name == "os" {
		return
	}
	vfsRegistryMtx.Lock()
	defer vfsRegistryMtx.Unlock()
	if vfsRegistry == nil {
		vfsRegistry = map[string]VFS{}
	}
	vfsRegistry[name] = vfs
}

// Unregister unregisters a VFS.
//
// https://sqlite.org/c3ref/vfs_find.html
func Unregister(name string) {
	vfsRegistryMtx.Lock()
	defer vfsRegistryMtx.Unlock()
	delete(vfsRegistry, name)
}
