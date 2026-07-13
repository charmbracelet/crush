//go:build !((darwin && (amd64 || arm64)) || (freebsd && (amd64 || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (windows && (386 || amd64 || arm64)))

package memory

import (
	"database/sql"
	"fmt"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
)

func openDatabase(path string) (*sql.DB, error) {
	return driver.Open(fmt.Sprintf("file:%s?_txlock=immediate", path), func(conn *sqlite3.Conn) error {
		for name, value := range memoryPragmas {
			if err := conn.Exec(fmt.Sprintf("PRAGMA %s = %s;", name, value)); err != nil {
				return err
			}
		}
		return nil
	})
}
