//go:build !filesnap_bundle

package binary

func bundledArchive(_ string) ([]byte, error) { return nil, nil }
