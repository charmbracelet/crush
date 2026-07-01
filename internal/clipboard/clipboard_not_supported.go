//go:build crush_omit_native || !((darwin || linux || windows || freebsd || openbsd || netbsd) && !ios && !android)

package clipboard

func initClipboard() error {
	return ErrUnsupported
}

func writeText(string) {}

func read(Format) ([]byte, error) {
	return nil, ErrUnsupported
}
