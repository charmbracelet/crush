//go:build !solaris && !illumos

package shell

import (
	"mvdan.cc/sh/moreinterp/coreutils"
)

// coreUtilsExecHandler installs mvdan.cc/sh's Go coreutils. It is only wired
// in when useGoCoreUtils is set. On illumos and solaris, coreutils_exec_stub.go
// supplies a nil handler because the coreutils package does not build there.
var coreUtilsExecHandler execMiddleware = coreutils.ExecHandler
