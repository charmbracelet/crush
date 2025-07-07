package jsons

import (
	"path/filepath"
	"strings"
)

func must(errs ...error) {
	for _, err := range errs {
		if err != nil {
			panic(err)
		}
	}
}

func getExtension(filename string) string {
	ext := filepath.Ext(filename)
	return strings.ToLower(ext)
}
