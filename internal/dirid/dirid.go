// Package dirid identifies directories independently of how their path is
// spelled, so that two spellings of one directory (another case on a
// case-insensitive filesystem, a symlinked parent, a Windows junction, subst
// drive or UNC path) can be recognized as the same directory.
package dirid

// ID identifies a directory. The zero value means the identity is unknown;
// it is never [ID.Same] as any ID, including itself.
type ID struct {
	id identity
	ok bool
}

// Of returns the identity of the directory at path, following symlinks and
// junctions. It reports false when the identity cannot be read or is not
// reliable enough to tell directories apart; callers must then treat the
// directory as distinct from every other one.
func Of(path string) (ID, bool) {
	id, ok := of(path)
	if !ok {
		return ID{}, false
	}
	return ID{id: id, ok: true}, true
}

// Same reports whether a and b are known to identify the same directory.
// It does no IO.
func (a ID) Same(b ID) bool {
	return a.ok && b.ok && same(a.id, b.id)
}

// validFileID reports whether a 128-bit Windows file ID can identify a
// file. All zeros and all ones are reserved values that do not.
func validFileID(id [16]byte) bool {
	zero, ones := true, true
	for _, b := range id {
		zero = zero && b == 0
		ones = ones && b == 0xff
	}
	return !zero && !ones
}

// validFileIndex reports whether a 64-bit Windows file index can identify a
// file. ReFS reports 0xFFFFFFFFFFFFFFFF (FILE_INVALID_FILE_ID) for every
// file whose 128-bit ID does not fit in 64 bits, and MS-FSCC says the value
// must then be ignored; 0 is likewise not a usable index.
func validFileIndex(idx uint64) bool {
	return idx != 0 && idx != ^uint64(0)
}
