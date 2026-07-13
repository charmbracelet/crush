//go:build (darwin && (amd64 || arm64)) || (freebsd && (amd64 || arm64)) || (linux && (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x)) || (windows && (386 || amd64 || arm64))

package memory

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

func openDatabase(path string) (*sql.DB, error) {
	params := url.Values{}
	for name, value := range memoryPragmas {
		params.Add("_pragma", fmt.Sprintf("%s(%s)", name, value))
	}
	params.Set("_txlock", "immediate")
	return sql.Open("sqlite", fmt.Sprintf("file:%s?%s", path, params.Encode()))
}
