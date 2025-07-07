// Package fastwalk provides a faster version of [filepath.WalkDir] for file
// system scanning tools.
package fastwalk

/*
 * This code borrows heavily from golang.org/x/tools/internal/fastwalk
 * and as such the Go license can be found in the go.LICENSE file and
 * is reproduced below:
 *
 * Copyright (c) 2009 The Go Authors. All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are
 * met:
 *
 *    * Redistributions of source code must retain the above copyright
 * notice, this list of conditions and the following disclaimer.
 *    * Redistributions in binary form must reproduce the above
 * copyright notice, this list of conditions and the following disclaimer
 * in the documentation and/or other materials provided with the
 * distribution.
 *    * Neither the name of Google Inc. nor the names of its
 * contributors may be used to endorse or promote products derived from
 * this software without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
 * "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
 * LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
 * A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
 * OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
 * SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
 * LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
 * DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
 * THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
 * OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ErrTraverseLink is used as a return value from WalkDirFuncs to indicate that
// the symlink named in the call may be traversed. This error is ignored if
// the Follow [Config] option is true.
var ErrTraverseLink = errors.New("fastwalk: traverse symlink, assuming target is a directory")

// ErrSkipFiles is a used as a return value from WalkFuncs to indicate that the
// callback should not be called for any other files in the current directory.
// Child directories will still be traversed.
var ErrSkipFiles = errors.New("fastwalk: skip remaining files in directory")

// SkipDir is used as a return value from WalkDirFuncs to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
var SkipDir = fs.SkipDir

// TODO(charlie): Look into implementing the fs.SkipAll behavior of
// filepath.Walk and filepath.WalkDir. This may not be possible without taking
// a performance hit.

// DefaultNumWorkers returns the default number of worker goroutines to use in
// [Walk] and is the value of [runtime.GOMAXPROCS](-1) clamped to a range
// of 4 to 32 except on Darwin where it is either 4 (8 cores or less), 6
// (10 cores or less), or 10 (more than 10 cores). This is because Walk / IO
// performance on Darwin degrades with more concurrency.
//
// The optimal number for your workload may be lower or higher. The results
// of BenchmarkFastWalkNumWorkers benchmark may be informative.
func DefaultNumWorkers() int {
	numCPU := runtime.GOMAXPROCS(-1)
	if numCPU < 4 {
		return 4
	}
	// Darwin IO performance on APFS can slow with increased parallelism.
	// Depending on CPU, stat(2) performance is best around 4-10 workers
	// and file IO is best around 4 workers. More workers only benefit CPU
	// intensive tasks.
	//
	// NB(Charlie): As of macOS 15, the parallel performance of readdir_r(3)
	// and stat(2) calls has improved and is now generally the number of
	// performance cores (on ARM Macs).
	//
	// TODO: Consider using the value of sysctl("hw.perflevel0.physicalcpu").
	// TODO: Find someone with a Mac Studio to test higher core counts.
	if runtime.GOOS == "darwin" {
		switch {
		case numCPU <= 8:
			return 4
		case numCPU <= 10:
			return 6
		default: // numCPU > 10
			return 10
		}
	}
	if numCPU > 32 {
		return 32
	}
	return numCPU
}

// DefaultToSlash returns true if this is a Go program compiled for Windows
// running in an environment ([MSYS/MSYS2] or [Git for Windows]) that uses
// forward slashes as the path separator instead of the native backslash.
//
// On non-Windows OSes this is a no-op and always returns false.
//
// To detect if we're running in [MSYS/MSYS2] we check if the "MSYSTEM"
// environment variable exists.
//
// DefaultToSlash does not detect if this is a Windows executable running in [WSL].
// Instead, users should (ideally) use programs compiled for Linux in WSL.
//
// See: [github.com/junegunn/fzf/issues/3859]
//
// NOTE: The reason that we do not check if we're running in WSL is that the
// test was inconsistent since it depended on the working directory (it seems
// that "/proc" cannot be accessed when programs are ran from a mounted Windows
// directory) and what environment variables are shared between WSL and Win32
// (this requires explicit [configuration]).
//
// [MSYS/MSYS2]: https://www.msys2.org/
// [WSL]: https://learn.microsoft.com/en-us/windows/wsl/about
// [Git for Windows]: https://gitforwindows.org/
// [github.com/junegunn/fzf/issues/3859]: https://github.com/junegunn/fzf/issues/3859
// [configuration]: https://devblogs.microsoft.com/commandline/share-environment-vars-between-wsl-and-windows/
func DefaultToSlash() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	// Previously this function attempted to determine if this is a Windows exe
	// running in WSL. The check was:
	//
	// * File /proc/sys/fs/binfmt_misc/WSLInterop exist
	// * Env var "WSL_DISTRO_NAME" exits
	// * /proc/version contains "Microsoft" or "microsoft"
	//
	// Below are my notes explaining why that check was flaky:
	//
	// NOTE: This appears to fail when ran from WSL when the current working
	// directory is a Windows directory that is mounted ("/mnt/c/...") since
	// "/proc" is not accessible. It works if ran from a directory that is not
	// mounted. Additionally, the "WSL_DISTRO_NAME" environment variable is not
	// set when ran from WSL.
	//
	// I'm not sure what causes this, but it would be great to find a solution.
	// My guess is that when ran from a Windows directory it uses the native
	// Windows path syscalls (for example os.Getwd reports the canonical Windows
	// path when a Go exe is ran from a mounted directory in WSL, but reports the
	// WSL path when ran from outside a mounted Windows directory).
	//
	// That said, the real solution here is to use programs compiled for Linux
	// when running in WSL.
	_, ok := os.LookupEnv("MSYSTEM")
	return ok
}

// SortMode determines the order that a directory's entries are visited by
// [Walk]. Sorting applies only at the directory level and since we process
// directories in parallel the order in which all files are visited is still
// non-deterministic.
//
// Sorting is mostly useful for programs that print the output of Walk since
// it makes it slightly more ordered compared to the default directory order.
// Sorting may also help some programs that wish to change the order in which
// a directory is processed by either processing all files first or enqueuing
// all directories before processing files.
//
// All lexical sorting is case-sensitive.
//
// The overhead of sorting is minimal compared to the syscalls needed to
// walk directories. The impact on performance due to changing the order
// in which directory entries are processed will be dependent on the workload
// and the structure of the file tree being visited (it might also have no
// impact).
type SortMode uint32

const (
	// Perform no sorting. Files will be visited in directory order.
	// This is the default.
	SortNone SortMode = iota

	// Directory entries are sorted by name before being visited.
	SortLexical

	// Sort the directory entries so that regular files and non-directories
	// (e.g. symbolic links) are visited before directories. Within each
	// group (regular files, other files, directories) the entries are sorted
	// by name.
	//
	// This is likely the mode that programs that print the output of Walk
	// want to use. Since by processing all files before enqueuing
	// sub-directories the output is slightly more grouped.
	//
	// Example order:
	//   - file: "a.txt"
	//   - file: "b.txt"
	//   - link: "a.link"
	//   - link: "b.link"
	//   - dir:  "d1/"
	//   - dir:  "d2/"
	//
	SortFilesFirst

	// Sort the directory entries so that directories are visited first, then
	// regular files are visited, and finally everything else is visited
	// (e.g. symbolic links). Within each group (directories, regular files,
	// other files) the entries are sorted by name.
	//
	// This mode is might be useful at preventing other walk goroutines from
	// stalling due to lack of work since it immediately enqueues all of a
	// directory's sub-directories for processing. The impact on performance
	// will be dependent on the workload and the structure of the file tree
	// being visited - it might also have no (or even a negative) impact on
	// performance so testing/benchmarking is recommend.
	//
	// An example workload that might cause this is: processing one directory
	// takes a long time, that directory has sub-directories we want to walk,
	// while processing that directory all other Walk goroutines have finished
	// processing their directories, those goroutines are now stalled waiting
	// for more work (waiting on the one running goroutine to enqueue its
	// sub-directories for processing).
	//
	// This might also be beneficial if processing files is expensive.
	//
	// Example order:
	//   - dir:  "d1/"
	//   - dir:  "d2/"
	//   - file: "a.txt"
	//   - file: "b.txt"
	//   - link: "a.link"
	//   - link: "b.link"
	//
	SortDirsFirst
)

var sortModeStrs = [...]string{
	SortNone:       "None",
	SortLexical:    "Lexical",
	SortDirsFirst:  "DirsFirst",
	SortFilesFirst: "FilesFirst",
}

func (s SortMode) String() string {
	if 0 <= int(s) && int(s) < len(sortModeStrs) {
		return sortModeStrs[s]
	}
	return "SortMode(" + itoa(uint64(s)) + ")"
}

// DefaultConfig is the default [Config] used when none is supplied.
var DefaultConfig = Config{
	Follow:     false,
	ToSlash:    DefaultToSlash(),
	NumWorkers: DefaultNumWorkers(),
	Sort:       SortNone,
	MaxDepth:   0,
}

// A Config controls the behavior of [Walk].
type Config struct {
	// TODO: do we want to pass a sentinel error to WalkFunc if
	// a symlink loop is detected?

	// Follow symbolic links ignoring directories that would lead
	// to infinite loops; that is, entering a previously visited
	// directory that is an ancestor of the last file encountered.
	//
	// The sentinel error ErrTraverseLink is ignored when Follow
	// is true (this to prevent users from defeating the loop
	// detection logic), but SkipDir and ErrSkipFiles are still
	// respected.
	Follow bool

	// Join all paths using a forward slash "/" instead of the system
	// default (the root path will be converted with filepath.ToSlash).
	// This option exists for users on Windows Subsystem for Linux (WSL)
	// that are running a Windows executable (like FZF) in WSL and need
	// forward slashes for compatibility (since binary was compiled for
	// Windows the path separator will be "\" which can cause issues in
	// in a Unix shell).
	//
	// This option has no effect when the OS path separator is a
	// forward slash "/".
	//
	// See FZF issue: https://github.com/junegunn/fzf/issues/3859
	ToSlash bool

	// Sort a directory's entries by SortMode before visiting them.
	// The order that files are visited is deterministic only at the directory
	// level, but not generally deterministic because we process directories
	// in parallel. The performance impact of sorting entries is generally
	// negligible compared to the syscalls required to read directories.
	//
	// This option mostly exists for programs that print the output of Walk
	// (like FZF) since it provides some order and thus makes the output much
	// nicer compared to the default directory order, which is basically random.
	Sort SortMode

	// Number of parallel workers to use. If NumWorkers if ≤ 0 then
	// DefaultNumWorkers is used.
	NumWorkers int

	// MaxDepth limits the depth of directory traversal to MaxDepth levels
	// beyond the root directory being walked. By default, there is no limit
	// on the search depth and a value of zero or less disables this feature.
	MaxDepth int
}

// Copy returns a copy of c. If c is nil an empty [Config] is returned.
func (c *Config) Copy() *Config {
	dupe := new(Config)
	if c != nil {
		*dupe = *c
	}
	return dupe
}

// A DirEntry extends the [fs.DirEntry] interface to add a Stat() method
// that returns the result of calling [os.Stat] on the underlying file.
// The results of Info() and Stat() are cached.
//
// The [fs.DirEntry] argument passed to the [fs.WalkDirFunc] by [Walk] is
// always a DirEntry.
type DirEntry interface {
	fs.DirEntry

	// Stat returns the fs.FileInfo for the file or subdirectory described
	// by the entry. The returned FileInfo may be from the time of the
	// original directory read or from the time of the call to os.Stat.
	// If the entry denotes a symbolic link, Stat reports the information
	// about the target itself, not the link.
	Stat() (fs.FileInfo, error)

	// Depth returns the depth at which this entry was generated relative to the
	// root being walked.
	Depth() int
}

// Walk is a faster implementation of [filepath.WalkDir] that walks the file
// tree rooted at root in parallel, calling walkFn for each file or directory
// in the tree, including root.
//
// All errors that arise visiting files and directories are filtered by walkFn
// see the [fs.WalkDirFunc] documentation for details.
// The [IgnorePermissionErrors] adapter is provided to handle to common case of
// ignoring [fs.ErrPermission] errors.
//
// By default files are walked in directory order, which makes the output
// non-deterministic. The Sort [Config] option can be used to control the order
// in which directory entries are visited, but since we walk the file tree in
// parallel the output is still non-deterministic (it's just slightly more
// sorted).
//
// When a symbolic link is encountered, by default Walk will not follow it
// unless walkFn returns [ErrTraverseLink] or the Follow [Config] setting is
// true. See below for a more detailed explanation.
//
// Walk calls walkFn with paths that use the separator character appropriate
// for the operating system unless the ToSlash [Config] setting is true which
// will cause all paths to be joined with a forward slash.
//
// If walkFn returns the [SkipDir] sentinel error, the directory is skipped.
// If walkFn returns the [ErrSkipFiles] sentinel error, the callback will not
// be called for any other files in the current directory. Unlike,
// [filepath.Walk] and [filepath.WalkDir] the [fs.SkipAll] sentinel error is
// not respected.
//
// Unlike [filepath.WalkDir]:
//
//   - Multiple goroutines stat the filesystem concurrently. The provided
//     walkFn must be safe for concurrent use.
//
//   - The order that directories are visited is non-deterministic.
//
//   - File stat calls must be done by the user and should be done via
//     the [DirEntry] argument to walkFn. The [DirEntry] caches the result
//     of both Info() and Stat(). The Stat() method is a fastwalk specific
//     extension and can be called by casting the [fs.DirEntry] to a
//     [fastwalk.DirEntry] or via the [StatDirEntry] helper. The [fs.DirEntry]
//     argument to walkFn will always be convertible to a [fastwalk.DirEntry].
//
//   - The [fs.DirEntry] argument is always a [fastwalk.DirEntry], which has
//     a Stat() method that returns the result of calling [os.Stat] on the
//     file. The result of Stat() and Info() are cached. The [StatDirEntry]
//     helper can be used to call Stat() on the returned [fastwalk.DirEntry].
//
//   - Additionally, the [fs.DirEntry] argument (which has type [fastwalk.DirEntry]),
//     has a Depth() method that returns the depth at which the entry was generated
//     relative to the root being walked. The [DirEntryDepth] helper function
//     can be used to call Depth() on the [fs.DirEntry] argument.
//
//   - Walk can follow symlinks in two ways: the fist, and simplest, is to
//     set Follow [Config] option to true - this will cause Walk to follow
//     symlinks and detect/ignore any symlink loops; the second, is for walkFn
//     to return the sentinel [ErrTraverseLink] error.
//     When using [ErrTraverseLink] to follow symlinks it is walkFn's
//     responsibility to prevent Walk from going into symlink cycles.
//     By default Walk does not follow symbolic links.
//
//   - When walking a directory, walkFn will be called for each non-directory
//     entry and directories will be enqueued and visited at a later time or
//     by another goroutine.
//
//   - The [fs.SkipAll] sentinel error is not respected and not ignored. If the
//     WalkDirFunc returns SkipAll then Walk will exit with the error SkipAll.
func Walk(conf *Config, root string, walkFn fs.WalkDirFunc) error {
	fi, err := os.Stat(root)
	if err != nil {
		return err
	}
	if conf == nil {
		dupe := DefaultConfig
		conf = &dupe
	}
	if conf.ToSlash {
		root = filepath.ToSlash(root)
	}

	// Make sure to wait for all workers to finish, otherwise
	// walkFn could still be called after returning. This Wait call
	// runs after close(e.donec) below.
	var wg sync.WaitGroup
	defer wg.Wait()

	numWorkers := conf.NumWorkers
	if numWorkers <= 0 {
		numWorkers = DefaultNumWorkers()
	}

	w := &walker{
		fn: walkFn,
		// TODO: Increase the size of enqueuec so that we don't stall
		// while processing a directory. Increasing the size of workc
		// doesn't help as much (needs more testing).
		enqueuec: make(chan walkItem, numWorkers), // buffered for performance
		workc:    make(chan walkItem, numWorkers), // buffered for performance
		donec:    make(chan struct{}),

		// buffered for correctness & not leaking goroutines:
		resc: make(chan error, numWorkers),

		// TODO: we should just pass the Config
		maxDepth: conf.MaxDepth,
		follow:   conf.Follow,
		toSlash:  conf.ToSlash,
		sortMode: conf.Sort,
	}
	if w.follow {
		w.ignoredDirs = append(w.ignoredDirs, fi)
	}

	defer close(w.donec)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go w.doWork(&wg)
	}

	root = cleanRootPath(root)
	// NOTE: in BenchmarkFastWalk the size of todo averages around
	// 170 and can be in the ~250 range at max.
	todo := []walkItem{{dir: root, info: fileInfoToDirEntry(filepath.Dir(root), fi)}}
	out := 0
	for {
		workc := w.workc
		var workItem walkItem
		if len(todo) == 0 {
			workc = nil
		} else {
			workItem = todo[len(todo)-1]
		}
		select {
		case workc <- workItem:
			todo = todo[:len(todo)-1]
			out++
		case it := <-w.enqueuec:
			// TODO: consider appending to todo directly and using a
			// mutext this might help with contention around select
			todo = append(todo, it)
		case err := <-w.resc:
			out--
			if err != nil {
				return err
			}
			if out == 0 && len(todo) == 0 {
				// It's safe to quit here, as long as the buffered
				// enqueue channel isn't also readable, which might
				// happen if the worker sends both another unit of
				// work and its result before the other select was
				// scheduled and both w.resc and w.enqueuec were
				// readable.
				select {
				case it := <-w.enqueuec:
					todo = append(todo, it)
				default:
					return nil
				}
			}
		}
	}
}

// doWork reads directories as instructed (via workc) and runs the
// user's callback function.
func (w *walker) doWork(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-w.donec:
			return
		case it := <-w.workc:
			select {
			case <-w.donec:
				return
			case w.resc <- w.walk(it.dir, it.info, !it.callbackDone):
			}
		}
	}
}

type walker struct {
	fn fs.WalkDirFunc

	donec    chan struct{} // closed on fastWalk's return
	workc    chan walkItem // to workers
	enqueuec chan walkItem // from workers
	resc     chan error    // from workers

	ignoredDirs []fs.FileInfo
	maxDepth    int
	follow      bool
	toSlash     bool
	sortMode    SortMode
}

type walkItem struct {
	dir          string
	info         DirEntry
	callbackDone bool // callback already called; don't do it again
}

func (w *walker) enqueue(it walkItem) {
	select {
	case w.enqueuec <- it:
	case <-w.donec:
	}
}

func (w *walker) shouldSkipDir(fi fs.FileInfo) bool {
	for _, ignored := range w.ignoredDirs {
		if os.SameFile(ignored, fi) {
			return true
		}
	}
	return false
}

func (w *walker) shouldTraverse(path string, de DirEntry) bool {
	ts, err := de.Stat()
	if err != nil {
		return false
	}
	if !ts.IsDir() {
		return false
	}
	if w.shouldSkipDir(ts) {
		return false
	}
	for {
		parent := filepath.Dir(path)
		if parent == path {
			return true
		}
		parentInfo, err := os.Stat(parent)
		if err != nil {
			return false
		}
		if os.SameFile(ts, parentInfo) {
			return false
		}
		path = parent
	}
}

func (w *walker) joinPaths(dir, base string) string {
	// Handle the case where the root path argument to Walk is "/"
	// without this the returned path is prefixed with "//".
	if os.PathSeparator == '/' {
		if len(dir) != 0 && dir[len(dir)-1] == '/' {
			return dir + base
		}
		return dir + "/" + base
	}
	if len(dir) != 0 && os.IsPathSeparator(dir[len(dir)-1]) {
		return dir + base
	}
	if w.toSlash {
		return dir + "/" + base
	}
	return dir + string(os.PathSeparator) + base
}

func (w *walker) onDirEnt(dirName, baseName string, de DirEntry) error {
	joined := w.joinPaths(dirName, baseName)
	typ := de.Type()
	if typ == os.ModeDir {
		w.enqueue(walkItem{dir: joined, info: de})
		return nil
	}

	err := w.fn(joined, de, nil)
	if typ == os.ModeSymlink {
		if err == ErrTraverseLink {
			if !w.follow {
				// Set callbackDone so we don't call it twice for both the
				// symlink-as-symlink and the symlink-as-directory later:
				w.enqueue(walkItem{dir: joined, info: de, callbackDone: true})
				return nil
			}
			err = nil // Ignore ErrTraverseLink when Follow is true.
		}
		if err == filepath.SkipDir {
			// Permit SkipDir on symlinks too.
			return nil
		}
		if err == nil && w.follow && w.shouldTraverse(joined, de) {
			// Traverse symlink
			w.enqueue(walkItem{dir: joined, info: de, callbackDone: true})
		}
	}
	return err
}

func (w *walker) walk(root string, info DirEntry, runUserCallback bool) error {
	if runUserCallback {
		err := w.fn(root, info, nil)
		if err == filepath.SkipDir {
			return nil
		}
		if err != nil {
			return err
		}
	}

	depth := info.Depth()
	if w.maxDepth > 0 && depth >= w.maxDepth {
		return nil
	}
	err := w.readDir(root, depth+1)
	if err != nil {
		// Second call, to report ReadDir error.
		return w.fn(root, info, err)
	}
	return nil
}

// cleanRootPath returns the root path trimmed of extraneous trailing slashes.
// This is a no-op on Windows.
func cleanRootPath(root string) string {
	if runtime.GOOS == "windows" || len(filepath.VolumeName(root)) != 0 {
		// Windows paths or any path with a volume name (which AFAIK should
		// only be Windows) are a bit too complicated to clean.
		return root
	}
	if len(filepath.VolumeName(root)) != 0 {
		return root
	}
	for i := len(root) - 1; i >= 0; i-- {
		if !os.IsPathSeparator(root[i]) {
			return root[:i+1]
		}
	}
	if root != "" {
		return root[0:1] // root is all path separators ("//")
	}
	return root
}

// Avoid the dependency on strconv since it pulls in a large number of other
// dependencies which bloats the size of this package.
func itoa(val uint64) string {
	buf := make([]byte, 20)
	i := len(buf) - 1
	for val >= 10 {
		buf[i] = byte(val%10 + '0')
		i--
		val /= 10
	}
	buf[i] = byte(val + '0')
	return string(buf[i:])
}
