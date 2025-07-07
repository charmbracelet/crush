// Package sql3util implements SQLite utilities.
package sql3util

// ValidPageSize returns true if s is a valid page size.
//
// https://sqlite.org/fileformat.html#pages
func ValidPageSize(s int) bool {
	return s&(s-1) == 0 && 512 <= s && s <= 65536
}
