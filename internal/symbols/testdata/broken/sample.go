package broken

func ValidFunc() string {
	return "ok"
}

func Broken( {
	// missing parameter and closing paren — triggers parse error
	return ""
}
