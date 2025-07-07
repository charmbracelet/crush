package driver

import "database/sql/driver"

func namedValues(args []driver.Value) []driver.NamedValue {
	named := make([]driver.NamedValue, len(args))
	for i, v := range args {
		named[i] = driver.NamedValue{
			Ordinal: i + 1,
			Value:   v,
		}
	}
	return named
}

func notWhitespace(sql string) bool {
	const (
		code = iota
		slash
		minus
		ccomment
		sqlcomment
		endcomment
	)

	state := code
	for _, b := range ([]byte)(sql) {
		if b == 0 {
			break
		}

		switch state {
		case code:
			switch b {
			case '/':
				state = slash
			case '-':
				state = minus
			case ' ', ';', '\t', '\n', '\v', '\f', '\r':
				continue
			default:
				return true
			}
		case slash:
			if b != '*' {
				return true
			}
			state = ccomment
		case minus:
			if b != '-' {
				return true
			}
			state = sqlcomment
		case ccomment:
			if b == '*' {
				state = endcomment
			}
		case sqlcomment:
			if b == '\n' {
				state = code
			}
		case endcomment:
			switch b {
			case '/':
				state = code
			case '*':
				state = endcomment
			default:
				state = ccomment
			}
		}
	}
	return state == slash || state == minus
}
