package sqlite3

// Backup is an handle to an ongoing online backup operation.
//
// https://sqlite.org/c3ref/backup.html
type Backup struct {
	c      *Conn
	handle ptr_t
	otherc ptr_t
}

// Backup backs up srcDB on the src connection to the "main" database in dstURI.
//
// Backup opens the SQLite database file dstURI,
// and blocks until the entire backup is complete.
// Use [Conn.BackupInit] for incremental backup.
//
// https://sqlite.org/backup.html
func (src *Conn) Backup(srcDB, dstURI string) error {
	b, err := src.BackupInit(srcDB, dstURI)
	if err != nil {
		return err
	}
	defer b.Close()
	_, err = b.Step(-1)
	return err
}

// Restore restores dstDB on the dst connection from the "main" database in srcURI.
//
// Restore opens the SQLite database file srcURI,
// and blocks until the entire restore is complete.
//
// https://sqlite.org/backup.html
func (dst *Conn) Restore(dstDB, srcURI string) error {
	src, err := dst.openDB(srcURI, OPEN_READONLY|OPEN_URI)
	if err != nil {
		return err
	}
	b, err := dst.backupInit(dst.handle, dstDB, src, "main")
	if err != nil {
		return err
	}
	defer b.Close()
	_, err = b.Step(-1)
	return err
}

// BackupInit initializes a backup operation to copy the content of one database into another.
//
// BackupInit opens the SQLite database file dstURI,
// then initializes a backup that copies the contents of srcDB on the src connection
// to the "main" database in dstURI.
//
// https://sqlite.org/c3ref/backup_finish.html#sqlite3backupinit
func (src *Conn) BackupInit(srcDB, dstURI string) (*Backup, error) {
	dst, err := src.openDB(dstURI, OPEN_READWRITE|OPEN_CREATE|OPEN_URI)
	if err != nil {
		return nil, err
	}
	return src.backupInit(dst, "main", src.handle, srcDB)
}

func (c *Conn) backupInit(dst ptr_t, dstName string, src ptr_t, srcName string) (*Backup, error) {
	defer c.arena.mark()()
	dstPtr := c.arena.string(dstName)
	srcPtr := c.arena.string(srcName)

	other := dst
	if c.handle == dst {
		other = src
	}

	ptr := ptr_t(c.call("sqlite3_backup_init",
		stk_t(dst), stk_t(dstPtr),
		stk_t(src), stk_t(srcPtr)))
	if ptr == 0 {
		defer c.closeDB(other)
		rc := res_t(c.call("sqlite3_errcode", stk_t(dst)))
		return nil, c.sqlite.error(rc, dst)
	}

	return &Backup{
		c:      c,
		otherc: other,
		handle: ptr,
	}, nil
}

// Close finishes a backup operation.
//
// It is safe to close a nil, zero or closed Backup.
//
// https://sqlite.org/c3ref/backup_finish.html#sqlite3backupfinish
func (b *Backup) Close() error {
	if b == nil || b.handle == 0 {
		return nil
	}

	rc := res_t(b.c.call("sqlite3_backup_finish", stk_t(b.handle)))
	b.c.closeDB(b.otherc)
	b.handle = 0
	return b.c.error(rc)
}

// Step copies up to nPage pages between the source and destination databases.
// If nPage is negative, all remaining source pages are copied.
//
// https://sqlite.org/c3ref/backup_finish.html#sqlite3backupstep
func (b *Backup) Step(nPage int) (done bool, err error) {
	rc := res_t(b.c.call("sqlite3_backup_step", stk_t(b.handle), stk_t(nPage)))
	if rc == _DONE {
		return true, nil
	}
	return false, b.c.error(rc)
}

// Remaining returns the number of pages still to be backed up
// at the conclusion of the most recent [Backup.Step].
//
// https://sqlite.org/c3ref/backup_finish.html#sqlite3backupremaining
func (b *Backup) Remaining() int {
	n := int32(b.c.call("sqlite3_backup_remaining", stk_t(b.handle)))
	return int(n)
}

// PageCount returns the total number of pages in the source database
// at the conclusion of the most recent [Backup.Step].
//
// https://sqlite.org/c3ref/backup_finish.html#sqlite3backuppagecount
func (b *Backup) PageCount() int {
	n := int32(b.c.call("sqlite3_backup_pagecount", stk_t(b.handle)))
	return int(n)
}
