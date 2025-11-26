package stringext

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func Capitalize(text string) string {
	return cases.Title(language.English, cases.Compact).String(text)
}

func ContainsAny(str string, args ...string) bool {
	for _, arg := range args {
		if strings.Contains(str, arg) {
			return true
		}
	}
	return false
}
