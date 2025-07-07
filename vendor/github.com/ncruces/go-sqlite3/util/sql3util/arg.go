package sql3util

import "strings"

// NamedArg splits an named arg into a key and value,
// around an equals sign.
// Spaces are trimmed around both key and value.
func NamedArg(arg string) (key, val string) {
	key, val, _ = strings.Cut(arg, "=")
	key = strings.TrimSpace(key)
	val = strings.TrimSpace(val)
	return
}

// Unquote unquotes a string.
//
// https://sqlite.org/lang_keywords.html
func Unquote(val string) string {
	if len(val) < 2 {
		return val
	}
	fst := val[0]
	lst := val[len(val)-1]
	rst := val[1 : len(val)-1]
	if fst == '[' && lst == ']' {
		return rst
	}
	if fst != lst {
		return val
	}
	var old, new string
	switch fst {
	default:
		return val
	case '`':
		old, new = "``", "`"
	case '"':
		old, new = `""`, `"`
	case '\'':
		old, new = `''`, `'`
	}
	return strings.ReplaceAll(rst, old, new)
}

// ParseBool parses a boolean.
//
// https://sqlite.org/pragma.html#syntax
func ParseBool(s string) (b, ok bool) {
	if len(s) == 0 {
		return false, false
	}
	if s[0] == '0' {
		return false, true
	}
	if '1' <= s[0] && s[0] <= '9' {
		return true, true
	}
	switch strings.ToLower(s) {
	case "true", "yes", "on":
		return true, true
	case "false", "no", "off":
		return false, true
	}
	return false, false
}
