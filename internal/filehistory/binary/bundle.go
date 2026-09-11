//go:build filesnap_bundle

package binary

import "embed"

//go:embed assets/*.tgz
var archives embed.FS

func bundledArchive(name string) ([]byte, error) {
	return archives.ReadFile("assets/" + name)
}
