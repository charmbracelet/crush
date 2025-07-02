package vfs

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

type vfsOS struct{}

func (vfsOS) FullPathname(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return path, testSymlinks(filepath.Dir(path))
}

func testSymlinks(path string) error {
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if p != path {
		return _OK_SYMLINK
	}
	return nil
}

func (vfsOS) Delete(path string, syncDir bool) error {
	err := os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return _IOERR_DELETE_NOENT
	}
	if err != nil {
		return err
	}
	if isUnix && syncDir {
		f, err := os.Open(filepath.Dir(path))
		if err != nil {
			return _OK
		}
		defer f.Close()
		err = osSync(f, false, false)
		if err != nil {
			return _IOERR_DIR_FSYNC
		}
	}
	return nil
}

func (vfsOS) Access(name string, flags AccessFlag) (bool, error) {
	err := osAccess(name, flags)
	if flags == ACCESS_EXISTS {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
	} else {
		if errors.Is(err, fs.ErrPermission) {
			return false, nil
		}
	}
	return err == nil, err
}

func (vfsOS) Open(name string, flags OpenFlag) (File, OpenFlag, error) {
	// notest // OpenFilename is called instead
	return nil, 0, _CANTOPEN
}

func (vfsOS) OpenFilename(name *Filename, flags OpenFlag) (File, OpenFlag, error) {
	oflags := _O_NOFOLLOW
	if flags&OPEN_EXCLUSIVE != 0 {
		oflags |= os.O_EXCL
	}
	if flags&OPEN_CREATE != 0 {
		oflags |= os.O_CREATE
	}
	if flags&OPEN_READONLY != 0 {
		oflags |= os.O_RDONLY
	}
	if flags&OPEN_READWRITE != 0 {
		oflags |= os.O_RDWR
	}

	isCreate := flags&(OPEN_CREATE) != 0
	isJournl := flags&(OPEN_MAIN_JOURNAL|OPEN_SUPER_JOURNAL|OPEN_WAL) != 0

	var err error
	var f *os.File
	if name == nil {
		f, err = os.CreateTemp(os.Getenv("SQLITE_TMPDIR"), "*.db")
	} else {
		f, err = os.OpenFile(name.String(), oflags, 0666)
	}
	if err != nil {
		if name == nil {
			return nil, flags, _IOERR_GETTEMPPATH
		}
		if errors.Is(err, syscall.EISDIR) {
			return nil, flags, _CANTOPEN_ISDIR
		}
		if isCreate && isJournl && errors.Is(err, fs.ErrPermission) &&
			osAccess(name.String(), ACCESS_EXISTS) != nil {
			return nil, flags, _READONLY_DIRECTORY
		}
		return nil, flags, err
	}

	if modeof := name.URIParameter("modeof"); modeof != "" {
		if err = osSetMode(f, modeof); err != nil {
			f.Close()
			return nil, flags, _IOERR_FSTAT
		}
	}
	if isUnix && flags&OPEN_DELETEONCLOSE != 0 {
		os.Remove(f.Name())
	}

	file := vfsFile{
		File:     f,
		psow:     true,
		atomic:   osBatchAtomic(f),
		readOnly: flags&OPEN_READONLY != 0,
		syncDir:  isUnix && isCreate && isJournl,
		delete:   !isUnix && flags&OPEN_DELETEONCLOSE != 0,
		shm:      NewSharedMemory(name.String()+"-shm", flags),
	}
	return &file, flags, nil
}

type vfsFile struct {
	*os.File
	shm      SharedMemory
	lock     LockLevel
	readOnly bool
	keepWAL  bool
	syncDir  bool
	atomic   bool
	delete   bool
	psow     bool
}

var (
	// Ensure these interfaces are implemented:
	_ FileLockState          = &vfsFile{}
	_ FileHasMoved           = &vfsFile{}
	_ FileSizeHint           = &vfsFile{}
	_ FilePersistWAL         = &vfsFile{}
	_ FilePowersafeOverwrite = &vfsFile{}
)

func (f *vfsFile) Close() error {
	if f.delete {
		defer os.Remove(f.Name())
	}
	if f.shm != nil {
		f.shm.Close()
	}
	f.Unlock(LOCK_NONE)
	return f.File.Close()
}

func (f *vfsFile) ReadAt(p []byte, off int64) (n int, err error) {
	return osReadAt(f.File, p, off)
}

func (f *vfsFile) WriteAt(p []byte, off int64) (n int, err error) {
	return osWriteAt(f.File, p, off)
}

func (f *vfsFile) Sync(flags SyncFlag) error {
	dataonly := (flags & SYNC_DATAONLY) != 0
	fullsync := (flags & 0x0f) == SYNC_FULL

	err := osSync(f.File, fullsync, dataonly)
	if err != nil {
		return err
	}
	if isUnix && f.syncDir {
		f.syncDir = false
		d, err := os.Open(filepath.Dir(f.File.Name()))
		if err != nil {
			return nil
		}
		defer d.Close()
		err = osSync(d, false, false)
		if err != nil {
			return _IOERR_DIR_FSYNC
		}
	}
	return nil
}

func (f *vfsFile) Size() (int64, error) {
	return f.Seek(0, io.SeekEnd)
}

func (f *vfsFile) SectorSize() int {
	return _DEFAULT_SECTOR_SIZE
}

func (f *vfsFile) DeviceCharacteristics() DeviceCharacteristic {
	ret := IOCAP_SUBPAGE_READ
	if f.atomic {
		ret |= IOCAP_BATCH_ATOMIC
	}
	if f.psow {
		ret |= IOCAP_POWERSAFE_OVERWRITE
	}
	if runtime.GOOS == "windows" {
		ret |= IOCAP_UNDELETABLE_WHEN_OPEN
	}
	return ret
}

func (f *vfsFile) SizeHint(size int64) error {
	return osAllocate(f.File, size)
}

func (f *vfsFile) HasMoved() (bool, error) {
	if runtime.GOOS == "windows" {
		return false, nil
	}
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	pi, err := os.Stat(f.Name())
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !os.SameFile(fi, pi), nil
}

func (f *vfsFile) LockState() LockLevel            { return f.lock }
func (f *vfsFile) PowersafeOverwrite() bool        { return f.psow }
func (f *vfsFile) PersistWAL() bool                { return f.keepWAL }
func (f *vfsFile) SetPowersafeOverwrite(psow bool) { f.psow = psow }
func (f *vfsFile) SetPersistWAL(keepWAL bool)      { f.keepWAL = keepWAL }
