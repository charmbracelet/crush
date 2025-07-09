package vfs

import (
	"context"
	"crypto/rand"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/ncruces/go-sqlite3/internal/util"
	"github.com/ncruces/go-sqlite3/util/sql3util"
	"github.com/ncruces/julianday"
)

// ExportHostFunctions is an internal API users need not call directly.
//
// ExportHostFunctions registers the required VFS host functions
// with the provided env module.
func ExportHostFunctions(env wazero.HostModuleBuilder) wazero.HostModuleBuilder {
	util.ExportFuncII(env, "go_vfs_find", vfsFind)
	util.ExportFuncIIJ(env, "go_localtime", vfsLocaltime)
	util.ExportFuncIIII(env, "go_randomness", vfsRandomness)
	util.ExportFuncIII(env, "go_sleep", vfsSleep)
	util.ExportFuncIII(env, "go_current_time_64", vfsCurrentTime64)
	util.ExportFuncIIIII(env, "go_full_pathname", vfsFullPathname)
	util.ExportFuncIIII(env, "go_delete", vfsDelete)
	util.ExportFuncIIIII(env, "go_access", vfsAccess)
	util.ExportFuncIIIIIII(env, "go_open", vfsOpen)
	util.ExportFuncII(env, "go_close", vfsClose)
	util.ExportFuncIIIIJ(env, "go_read", vfsRead)
	util.ExportFuncIIIIJ(env, "go_write", vfsWrite)
	util.ExportFuncIIJ(env, "go_truncate", vfsTruncate)
	util.ExportFuncIII(env, "go_sync", vfsSync)
	util.ExportFuncIII(env, "go_file_size", vfsFileSize)
	util.ExportFuncIIII(env, "go_file_control", vfsFileControl)
	util.ExportFuncII(env, "go_sector_size", vfsSectorSize)
	util.ExportFuncII(env, "go_device_characteristics", vfsDeviceCharacteristics)
	util.ExportFuncIII(env, "go_lock", vfsLock)
	util.ExportFuncIII(env, "go_unlock", vfsUnlock)
	util.ExportFuncIII(env, "go_check_reserved_lock", vfsCheckReservedLock)
	util.ExportFuncIIIIII(env, "go_shm_map", vfsShmMap)
	util.ExportFuncIIIII(env, "go_shm_lock", vfsShmLock)
	util.ExportFuncIII(env, "go_shm_unmap", vfsShmUnmap)
	util.ExportFuncVI(env, "go_shm_barrier", vfsShmBarrier)
	return env
}

func vfsFind(ctx context.Context, mod api.Module, zVfsName ptr_t) uint32 {
	name := util.ReadString(mod, zVfsName, _MAX_NAME)
	if vfs := Find(name); vfs != nil && vfs != (vfsOS{}) {
		return 1
	}
	return 0
}

func vfsLocaltime(ctx context.Context, mod api.Module, pTm ptr_t, t int64) _ErrorCode {
	const size = 32 / 8
	tm := time.Unix(t, 0)
	// https://pubs.opengroup.org/onlinepubs/7908799/xsh/time.h.html
	util.Write32(mod, pTm+0*size, int32(tm.Second()))
	util.Write32(mod, pTm+1*size, int32(tm.Minute()))
	util.Write32(mod, pTm+2*size, int32(tm.Hour()))
	util.Write32(mod, pTm+3*size, int32(tm.Day()))
	util.Write32(mod, pTm+4*size, int32(tm.Month()-time.January))
	util.Write32(mod, pTm+5*size, int32(tm.Year()-1900))
	util.Write32(mod, pTm+6*size, int32(tm.Weekday()-time.Sunday))
	util.Write32(mod, pTm+7*size, int32(tm.YearDay()-1))
	util.WriteBool(mod, pTm+8*size, tm.IsDST())
	return _OK
}

func vfsRandomness(ctx context.Context, mod api.Module, pVfs ptr_t, nByte int32, zByte ptr_t) uint32 {
	mem := util.View(mod, zByte, int64(nByte))
	n, _ := rand.Reader.Read(mem)
	return uint32(n)
}

func vfsSleep(ctx context.Context, mod api.Module, pVfs ptr_t, nMicro int32) _ErrorCode {
	time.Sleep(time.Duration(nMicro) * time.Microsecond)
	return _OK
}

func vfsCurrentTime64(ctx context.Context, mod api.Module, pVfs, piNow ptr_t) _ErrorCode {
	day, nsec := julianday.Date(time.Now())
	msec := day*86_400_000 + nsec/1_000_000
	util.Write64(mod, piNow, msec)
	return _OK
}

func vfsFullPathname(ctx context.Context, mod api.Module, pVfs, zRelative ptr_t, nFull int32, zFull ptr_t) _ErrorCode {
	vfs := vfsGet(mod, pVfs)
	path := util.ReadString(mod, zRelative, _MAX_PATHNAME)

	path, err := vfs.FullPathname(path)

	if len(path) >= int(nFull) {
		return _CANTOPEN_FULLPATH
	}
	util.WriteString(mod, zFull, path)

	return vfsErrorCode(err, _CANTOPEN_FULLPATH)
}

func vfsDelete(ctx context.Context, mod api.Module, pVfs, zPath ptr_t, syncDir int32) _ErrorCode {
	vfs := vfsGet(mod, pVfs)
	path := util.ReadString(mod, zPath, _MAX_PATHNAME)

	err := vfs.Delete(path, syncDir != 0)
	return vfsErrorCode(err, _IOERR_DELETE)
}

func vfsAccess(ctx context.Context, mod api.Module, pVfs, zPath ptr_t, flags AccessFlag, pResOut ptr_t) _ErrorCode {
	vfs := vfsGet(mod, pVfs)
	path := util.ReadString(mod, zPath, _MAX_PATHNAME)

	ok, err := vfs.Access(path, flags)
	util.WriteBool(mod, pResOut, ok)
	return vfsErrorCode(err, _IOERR_ACCESS)
}

func vfsOpen(ctx context.Context, mod api.Module, pVfs, zPath, pFile ptr_t, flags OpenFlag, pOutFlags, pOutVFS ptr_t) _ErrorCode {
	vfs := vfsGet(mod, pVfs)
	name := GetFilename(ctx, mod, zPath, flags)

	var file File
	var err error
	if ffs, ok := vfs.(VFSFilename); ok {
		file, flags, err = ffs.OpenFilename(name, flags)
	} else {
		file, flags, err = vfs.Open(name.String(), flags)
	}
	if err != nil {
		return vfsErrorCode(err, _CANTOPEN)
	}

	if file, ok := file.(FilePowersafeOverwrite); ok {
		if b, ok := sql3util.ParseBool(name.URIParameter("psow")); ok {
			file.SetPowersafeOverwrite(b)
		}
	}
	if file, ok := file.(FileSharedMemory); ok && pOutVFS != 0 {
		util.WriteBool(mod, pOutVFS, file.SharedMemory() != nil)
	}
	if pOutFlags != 0 {
		util.Write32(mod, pOutFlags, flags)
	}
	file = cksmWrapFile(name, flags, file)
	vfsFileRegister(ctx, mod, pFile, file)
	return _OK
}

func vfsClose(ctx context.Context, mod api.Module, pFile ptr_t) _ErrorCode {
	err := vfsFileClose(ctx, mod, pFile)
	return vfsErrorCode(err, _IOERR_CLOSE)
}

func vfsRead(ctx context.Context, mod api.Module, pFile, zBuf ptr_t, iAmt int32, iOfst int64) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	buf := util.View(mod, zBuf, int64(iAmt))

	n, err := file.ReadAt(buf, iOfst)
	if n == int(iAmt) {
		return _OK
	}
	if err != io.EOF {
		return vfsErrorCode(err, _IOERR_READ)
	}
	clear(buf[n:])
	return _IOERR_SHORT_READ
}

func vfsWrite(ctx context.Context, mod api.Module, pFile, zBuf ptr_t, iAmt int32, iOfst int64) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	buf := util.View(mod, zBuf, int64(iAmt))

	_, err := file.WriteAt(buf, iOfst)
	return vfsErrorCode(err, _IOERR_WRITE)
}

func vfsTruncate(ctx context.Context, mod api.Module, pFile ptr_t, nByte int64) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	err := file.Truncate(nByte)
	return vfsErrorCode(err, _IOERR_TRUNCATE)
}

func vfsSync(ctx context.Context, mod api.Module, pFile ptr_t, flags SyncFlag) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	err := file.Sync(flags)
	return vfsErrorCode(err, _IOERR_FSYNC)
}

func vfsFileSize(ctx context.Context, mod api.Module, pFile, pSize ptr_t) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	size, err := file.Size()
	util.Write64(mod, pSize, size)
	return vfsErrorCode(err, _IOERR_SEEK)
}

func vfsLock(ctx context.Context, mod api.Module, pFile ptr_t, eLock LockLevel) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	err := file.Lock(eLock)
	return vfsErrorCode(err, _IOERR_LOCK)
}

func vfsUnlock(ctx context.Context, mod api.Module, pFile ptr_t, eLock LockLevel) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	err := file.Unlock(eLock)
	return vfsErrorCode(err, _IOERR_UNLOCK)
}

func vfsCheckReservedLock(ctx context.Context, mod api.Module, pFile, pResOut ptr_t) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	locked, err := file.CheckReservedLock()
	util.WriteBool(mod, pResOut, locked)
	return vfsErrorCode(err, _IOERR_CHECKRESERVEDLOCK)
}

func vfsFileControl(ctx context.Context, mod api.Module, pFile ptr_t, op _FcntlOpcode, pArg ptr_t) _ErrorCode {
	file := vfsFileGet(ctx, mod, pFile).(File)
	if file, ok := file.(fileControl); ok {
		return file.fileControl(ctx, mod, op, pArg)
	}
	return vfsFileControlImpl(ctx, mod, file, op, pArg)
}

func vfsFileControlImpl(ctx context.Context, mod api.Module, file File, op _FcntlOpcode, pArg ptr_t) _ErrorCode {
	switch op {
	case _FCNTL_LOCKSTATE:
		if file, ok := file.(FileLockState); ok {
			if lk := file.LockState(); lk <= LOCK_EXCLUSIVE {
				util.Write32(mod, pArg, lk)
				return _OK
			}
		}

	case _FCNTL_PERSIST_WAL:
		if file, ok := file.(FilePersistWAL); ok {
			if i := util.Read32[int32](mod, pArg); i < 0 {
				util.WriteBool(mod, pArg, file.PersistWAL())
			} else {
				file.SetPersistWAL(i != 0)
			}
			return _OK
		}

	case _FCNTL_POWERSAFE_OVERWRITE:
		if file, ok := file.(FilePowersafeOverwrite); ok {
			if i := util.Read32[int32](mod, pArg); i < 0 {
				util.WriteBool(mod, pArg, file.PowersafeOverwrite())
			} else {
				file.SetPowersafeOverwrite(i != 0)
			}
			return _OK
		}

	case _FCNTL_CHUNK_SIZE:
		if file, ok := file.(FileChunkSize); ok {
			size := util.Read32[int32](mod, pArg)
			file.ChunkSize(int(size))
			return _OK
		}

	case _FCNTL_SIZE_HINT:
		if file, ok := file.(FileSizeHint); ok {
			size := util.Read64[int64](mod, pArg)
			err := file.SizeHint(size)
			return vfsErrorCode(err, _IOERR_TRUNCATE)
		}

	case _FCNTL_HAS_MOVED:
		if file, ok := file.(FileHasMoved); ok {
			moved, err := file.HasMoved()
			util.WriteBool(mod, pArg, moved)
			return vfsErrorCode(err, _IOERR_FSTAT)
		}

	case _FCNTL_OVERWRITE:
		if file, ok := file.(FileOverwrite); ok {
			err := file.Overwrite()
			return vfsErrorCode(err, _IOERR)
		}

	case _FCNTL_SYNC:
		if file, ok := file.(FileSync); ok {
			var name string
			if pArg != 0 {
				name = util.ReadString(mod, pArg, _MAX_PATHNAME)
			}
			err := file.SyncSuper(name)
			return vfsErrorCode(err, _IOERR)
		}

	case _FCNTL_COMMIT_PHASETWO:
		if file, ok := file.(FileCommitPhaseTwo); ok {
			err := file.CommitPhaseTwo()
			return vfsErrorCode(err, _IOERR)
		}

	case _FCNTL_BEGIN_ATOMIC_WRITE:
		if file, ok := file.(FileBatchAtomicWrite); ok {
			err := file.BeginAtomicWrite()
			return vfsErrorCode(err, _IOERR_BEGIN_ATOMIC)
		}
	case _FCNTL_COMMIT_ATOMIC_WRITE:
		if file, ok := file.(FileBatchAtomicWrite); ok {
			err := file.CommitAtomicWrite()
			return vfsErrorCode(err, _IOERR_COMMIT_ATOMIC)
		}
	case _FCNTL_ROLLBACK_ATOMIC_WRITE:
		if file, ok := file.(FileBatchAtomicWrite); ok {
			err := file.RollbackAtomicWrite()
			return vfsErrorCode(err, _IOERR_ROLLBACK_ATOMIC)
		}

	case _FCNTL_CKPT_START:
		if file, ok := file.(FileCheckpoint); ok {
			file.CheckpointStart()
			return _OK
		}
	case _FCNTL_CKPT_DONE:
		if file, ok := file.(FileCheckpoint); ok {
			file.CheckpointDone()
			return _OK
		}

	case _FCNTL_PRAGMA:
		if file, ok := file.(FilePragma); ok {
			ptr := util.Read32[ptr_t](mod, pArg+1*ptrlen)
			name := util.ReadString(mod, ptr, _MAX_SQL_LENGTH)
			var value string
			if ptr := util.Read32[ptr_t](mod, pArg+2*ptrlen); ptr != 0 {
				value = util.ReadString(mod, ptr, _MAX_SQL_LENGTH)
			}

			out, err := file.Pragma(strings.ToLower(name), value)

			ret := vfsErrorCode(err, _ERROR)
			if ret == _ERROR {
				out = err.Error()
			}
			if out != "" {
				fn := mod.ExportedFunction("sqlite3_malloc64")
				stack := [...]stk_t{stk_t(len(out) + 1)}
				if err := fn.CallWithStack(ctx, stack[:]); err != nil {
					panic(err)
				}
				util.Write32(mod, pArg, ptr_t(stack[0]))
				util.WriteString(mod, ptr_t(stack[0]), out)
			}
			return ret
		}

	case _FCNTL_BUSYHANDLER:
		if file, ok := file.(FileBusyHandler); ok {
			arg := util.Read64[stk_t](mod, pArg)
			fn := mod.ExportedFunction("sqlite3_invoke_busy_handler_go")
			file.BusyHandler(func() bool {
				stack := [...]stk_t{arg}
				if err := fn.CallWithStack(ctx, stack[:]); err != nil {
					panic(err)
				}
				return uint32(stack[0]) != 0
			})
			return _OK
		}

	case _FCNTL_LOCK_TIMEOUT:
		if file, ok := file.(FileSharedMemory); ok {
			if shm, ok := file.SharedMemory().(blockingSharedMemory); ok {
				shm.shmEnableBlocking(util.ReadBool(mod, pArg))
				return _OK
			}
		}

	case _FCNTL_PDB:
		if file, ok := file.(filePDB); ok {
			file.SetDB(ctx.Value(util.ConnKey{}))
			return _OK
		}
	}

	return _NOTFOUND
}

func vfsSectorSize(ctx context.Context, mod api.Module, pFile ptr_t) uint32 {
	file := vfsFileGet(ctx, mod, pFile).(File)
	return uint32(file.SectorSize())
}

func vfsDeviceCharacteristics(ctx context.Context, mod api.Module, pFile ptr_t) DeviceCharacteristic {
	file := vfsFileGet(ctx, mod, pFile).(File)
	return file.DeviceCharacteristics()
}

func vfsShmBarrier(ctx context.Context, mod api.Module, pFile ptr_t) {
	shm := vfsFileGet(ctx, mod, pFile).(FileSharedMemory).SharedMemory()
	shm.shmBarrier()
}

func vfsShmMap(ctx context.Context, mod api.Module, pFile ptr_t, iRegion, szRegion, bExtend int32, pp ptr_t) _ErrorCode {
	shm := vfsFileGet(ctx, mod, pFile).(FileSharedMemory).SharedMemory()
	p, rc := shm.shmMap(ctx, mod, iRegion, szRegion, bExtend != 0)
	util.Write32(mod, pp, p)
	return rc
}

func vfsShmLock(ctx context.Context, mod api.Module, pFile ptr_t, offset, n int32, flags _ShmFlag) _ErrorCode {
	shm := vfsFileGet(ctx, mod, pFile).(FileSharedMemory).SharedMemory()
	return shm.shmLock(offset, n, flags)
}

func vfsShmUnmap(ctx context.Context, mod api.Module, pFile ptr_t, bDelete int32) _ErrorCode {
	shm := vfsFileGet(ctx, mod, pFile).(FileSharedMemory).SharedMemory()
	shm.shmUnmap(bDelete != 0)
	return _OK
}

func vfsGet(mod api.Module, pVfs ptr_t) VFS {
	var name string
	if pVfs != 0 {
		const zNameOffset = 16
		ptr := util.Read32[ptr_t](mod, pVfs+zNameOffset)
		name = util.ReadString(mod, ptr, _MAX_NAME)
	}
	if vfs := Find(name); vfs != nil {
		return vfs
	}
	panic(util.NoVFSErr + util.ErrorString(name))
}

func vfsFileRegister(ctx context.Context, mod api.Module, pFile ptr_t, file File) {
	const fileHandleOffset = 4
	id := util.AddHandle(ctx, file)
	util.Write32(mod, pFile+fileHandleOffset, id)
}

func vfsFileGet(ctx context.Context, mod api.Module, pFile ptr_t) any {
	const fileHandleOffset = 4
	id := util.Read32[ptr_t](mod, pFile+fileHandleOffset)
	return util.GetHandle(ctx, id)
}

func vfsFileClose(ctx context.Context, mod api.Module, pFile ptr_t) error {
	const fileHandleOffset = 4
	id := util.Read32[ptr_t](mod, pFile+fileHandleOffset)
	return util.DelHandle(ctx, id)
}

func vfsErrorCode(err error, def _ErrorCode) _ErrorCode {
	if err == nil {
		return _OK
	}
	switch v := reflect.ValueOf(err); v.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return _ErrorCode(v.Uint())
	}
	return def
}
