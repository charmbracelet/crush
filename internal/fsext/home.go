package fsext

import (
	"cmp"
	"os"
	"os/user"
	"sync"
)

var HomeDir = sync.OnceValue(func() string {
	u, err := user.Current()
	if err == nil {
		return u.HomeDir
	}
	return cmp.Or(
		os.Getenv("HOME"),
		os.Getenv("USERPROFILE"),
		os.Getenv("HOMEPATH"),
	)
})
